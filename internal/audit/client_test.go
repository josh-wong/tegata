package audit_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/audit/rpc"
	"github.com/josh-wong/tegata/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
)

const bufSize = 1 << 20 // 1 MiB in-process buffer

// mockSigner records calls to Sign and returns a fixed signature.
type mockSigner struct {
	calls []signCall
	sig   []byte
	err   error
}

type signCall struct {
	contractID       string
	contractArgument string
	nonce            string
	entityID         string
	keyVersion       uint32
}

func (m *mockSigner) Sign(contractID, contractArgument, nonce, entityID string, keyVersion uint32) ([]byte, error) {
	m.calls = append(m.calls, signCall{
		contractID:       contractID,
		contractArgument: contractArgument,
		nonce:            nonce,
		entityID:         entityID,
		keyVersion:       keyVersion,
	})
	return m.sig, m.err
}

// mockLedgerServer is a minimal in-process ScalarDL Ledger gRPC server for unit tests.
type mockLedgerServer struct {
	rpc.UnimplementedLedgerServer
	execResult string
	execErr    error
}

func (m *mockLedgerServer) ExecuteContract(_ context.Context, req *rpc.ContractExecutionRequest) (*rpc.ContractExecutionResponse, error) {
	if m.execErr != nil {
		return nil, m.execErr
	}
	return &rpc.ContractExecutionResponse{ContractResult: m.execResult}, nil
}

// mockPrivilegedServer is a minimal in-process LedgerPrivileged gRPC server.
type mockPrivilegedServer struct {
	rpc.UnimplementedLedgerPrivilegedServer
	receivedReq *rpc.CertificateRegistrationRequest
	regErr      error
}

func (m *mockPrivilegedServer) RegisterCert(_ context.Context, req *rpc.CertificateRegistrationRequest) (*emptypb.Empty, error) {
	m.receivedReq = req
	if m.regErr != nil {
		return nil, m.regErr
	}
	return &emptypb.Empty{}, nil
}

// newBufconnServer starts an in-process gRPC server using bufconn and registers
// both mock services. Returns a cleanup function and a client connection.
func newBufconnServer(t *testing.T, ledger *mockLedgerServer, privileged *mockPrivilegedServer) *grpc.ClientConn {
	t.Helper()
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	rpc.RegisterLedgerServer(srv, ledger)
	rpc.RegisterLedgerPrivilegedServer(srv, privileged)

	go func() {
		if err := srv.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			t.Logf("bufconn server error: %v", err)
		}
	}()
	t.Cleanup(func() { srv.Stop(); _ = lis.Close() })

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("bufconn dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// TestClient_RequiresTLS verifies that NewLedgerClient rejects a nil TLS config.
func TestClient_RequiresTLS(t *testing.T) {
	_, err := audit.NewLedgerClient("localhost:50051", "localhost:50052", nil, "test-entity", 1, &audit.NoOpSigner{})
	if err == nil {
		t.Fatal("expected error for nil TLS config, got nil")
	}
}

// TestClient_PutCallsSigner verifies that Put calls Sign exactly once with the
// correct fields (contractID="object.v1_0_0.Put", entityID, keyVersion).
func TestClient_PutCallsSigner(t *testing.T) {
	signer := &mockSigner{sig: []byte("fake-sig")}
	ledgerSrv := &mockLedgerServer{}
	privSrv := &mockPrivilegedServer{}
	conn := newBufconnServer(t, ledgerSrv, privSrv)

	client := audit.NewLedgerClientFromConn(conn, nil, signer, "test-entity", 1)
	defer func() { _ = client.Close() }()

	err := client.Put(context.Background(), "obj-123", "deadbeef")
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	if len(signer.calls) != 1 {
		t.Fatalf("expected 1 Sign call, got %d", len(signer.calls))
	}
	call := signer.calls[0]
	if call.contractID != "object.v1_0_0.Put" {
		t.Errorf("contractID = %q, want %q", call.contractID, "object.v1_0_0.Put")
	}
	if call.entityID != "test-entity" {
		t.Errorf("entityID = %q, want %q", call.entityID, "test-entity")
	}
	if call.keyVersion != 1 {
		t.Errorf("keyVersion = %d, want 1", call.keyVersion)
	}
}

// TestClient_RegisterCertUsesPrivilegedService verifies that RegisterCert routes
// to the LedgerPrivileged service (not the Ledger service) via the bufconn mock.
func TestClient_RegisterCertUsesPrivilegedService(t *testing.T) {
	signer := &audit.NoOpSigner{}
	ledgerSrv := &mockLedgerServer{}
	privSrv := &mockPrivilegedServer{}
	conn := newBufconnServer(t, ledgerSrv, privSrv)

	client := audit.NewLedgerClientFromConn(conn, nil, signer, "test-entity", 1)
	defer func() { _ = client.Close() }()

	err := client.RegisterCert(context.Background(), "reg-entity", 2, "-----BEGIN CERTIFICATE-----\nfake\n-----END CERTIFICATE-----")
	if err != nil {
		t.Fatalf("RegisterCert: %v", err)
	}

	// Verify the request reached the privileged server (not the ledger server).
	if privSrv.receivedReq == nil {
		t.Fatal("LedgerPrivileged.RegisterCert was not called")
	}
	if privSrv.receivedReq.EntityId != "reg-entity" {
		t.Errorf("EntityId = %q, want %q", privSrv.receivedReq.EntityId, "reg-entity")
	}
	if privSrv.receivedReq.KeyVersion != 2 {
		t.Errorf("KeyVersion = %d, want 2", privSrv.receivedReq.KeyVersion)
	}
}

// mockLedgerServerMulti is a mock that returns different results for sequential
// ExecuteContract calls, allowing tests to verify multi-step flows like
// Get-then-Validate.
type mockLedgerServerMulti struct {
	rpc.UnimplementedLedgerServer
	results []string // each call pops the first element
	errs    []error
	calls   []*rpc.ContractExecutionRequest
}

func (m *mockLedgerServerMulti) ExecuteContract(_ context.Context, req *rpc.ContractExecutionRequest) (*rpc.ContractExecutionResponse, error) {
	m.calls = append(m.calls, req)
	idx := len(m.calls) - 1
	var execErr error
	if idx < len(m.errs) {
		execErr = m.errs[idx]
	}
	if execErr != nil {
		return nil, execErr
	}
	var result string
	if idx < len(m.results) {
		result = m.results[idx]
	}
	return &rpc.ContractExecutionResponse{ContractResult: result}, nil
}

// newBufconnServerMulti is like newBufconnServer but uses mockLedgerServerMulti
// for multi-step flow tests.
func newBufconnServerMulti(t *testing.T, ledger *mockLedgerServerMulti, privileged *mockPrivilegedServer) *grpc.ClientConn {
	t.Helper()
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	rpc.RegisterLedgerServer(srv, ledger)
	rpc.RegisterLedgerPrivilegedServer(srv, privileged)

	go func() {
		if err := srv.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			t.Logf("bufconn server error: %v", err)
		}
	}()
	t.Cleanup(func() { srv.Stop(); _ = lis.Close() })

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("bufconn dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// TestClient_ValidateArgSchema verifies that Validate calls object.v1_0_0.Get first,
// then object.v1_0_0.Validate with a versions array containing version_id and
// hash_value pairs from the Get result.
func TestClient_ValidateArgSchema(t *testing.T) {
	signer := &mockSigner{sig: []byte("fake-sig")}

	// The Get response returns two records.
	getResult := `[{"object_id":"evt-001","hash_value":"aaa","age":100},{"object_id":"evt-002","hash_value":"bbb","age":200}]`
	// The Validate response returns correct status.
	validateResult := `{"status":"correct","details":"","faulty_versions":[]}`

	ledgerSrv := &mockLedgerServerMulti{
		results: []string{getResult, validateResult},
	}
	privSrv := &mockPrivilegedServer{}
	conn := newBufconnServerMulti(t, ledgerSrv, privSrv)

	client := audit.NewLedgerClientFromConn(conn, nil, signer, "test-entity", 1)
	defer func() { _ = client.Close() }()

	result, err := client.Validate(context.Background(), "tegata-")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	// Verify two ExecuteContract calls were made (Get + Validate).
	if len(ledgerSrv.calls) != 2 {
		t.Fatalf("expected 2 ExecuteContract calls, got %d", len(ledgerSrv.calls))
	}

	// First call should be object.v1_0_0.Get.
	if ledgerSrv.calls[0].ContractId != "object.v1_0_0.Get" {
		t.Errorf("first call contract ID = %q, want %q", ledgerSrv.calls[0].ContractId, "object.v1_0_0.Get")
	}

	// Second call should be object.v1_0_0.Validate with versions array.
	if ledgerSrv.calls[1].ContractId != "object.v1_0_0.Validate" {
		t.Errorf("second call contract ID = %q, want %q", ledgerSrv.calls[1].ContractId, "object.v1_0_0.Validate")
	}

	// The second call's argument must contain "versions" and "object_id" keys.
	arg := ledgerSrv.calls[1].ContractArgument
	if !strings.Contains(arg, `"versions"`) {
		t.Errorf("Validate argument missing 'versions' key: %s", arg)
	}
	if !strings.Contains(arg, `"object_id"`) {
		t.Errorf("Validate argument missing 'object_id' key: %s", arg)
	}

	// Result should be valid with 2 events.
	if !result.Valid {
		t.Error("expected Valid=true, got false")
	}
	if result.EventCount != 2 {
		t.Errorf("EventCount = %d, want 2", result.EventCount)
	}
}

// TestClient_ValidateEmptyRecords verifies that when Get returns no records,
// Validate returns Valid=true with EventCount=0 without making a second RPC call.
func TestClient_ValidateEmptyRecords(t *testing.T) {
	signer := &mockSigner{sig: []byte("fake-sig")}

	// The Get response returns an empty array.
	ledgerSrv := &mockLedgerServerMulti{
		results: []string{"[]"},
	}
	privSrv := &mockPrivilegedServer{}
	conn := newBufconnServerMulti(t, ledgerSrv, privSrv)

	client := audit.NewLedgerClientFromConn(conn, nil, signer, "test-entity", 1)
	defer func() { _ = client.Close() }()

	result, err := client.Validate(context.Background(), "tegata-")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	// Only one call should have been made (Get only, no Validate).
	if len(ledgerSrv.calls) != 1 {
		t.Fatalf("expected 1 ExecuteContract call (Get only), got %d", len(ledgerSrv.calls))
	}

	if !result.Valid {
		t.Error("expected Valid=true for empty records, got false")
	}
	if result.EventCount != 0 {
		t.Errorf("EventCount = %d, want 0", result.EventCount)
	}
}

// TestClient_TLSEnforced_WithValidConfig verifies that NewLedgerClient accepts a
// non-nil TLS config (actual dial failure expected since there is no server).
func TestClient_TLSEnforced_WithValidConfig(t *testing.T) {
	cfg := &tls.Config{InsecureSkipVerify: true} //nolint:gosec // test only
	_, err := audit.NewLedgerClient("localhost:59999", "localhost:59998", cfg, "entity", 1, &audit.NoOpSigner{})
	// grpc.NewClient is lazy — the error (if any) occurs on first RPC, not here.
	// The important thing is that no error is returned for a non-nil TLS config.
	if err != nil {
		t.Errorf("NewLedgerClient with valid TLS config returned error: %v", err)
	}
}

// generateSelfSignedCA creates a temporary self-signed CA certificate PEM file
// for testing TLS configuration. Returns the path to the PEM file.
func generateSelfSignedCA(t *testing.T) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating CA key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{Organization: []string{"Test CA"}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:         true,
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("creating CA certificate: %v", err)
	}

	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	dir := t.TempDir()
	path := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(path, pemBytes, 0600); err != nil {
		t.Fatalf("writing CA PEM: %v", err)
	}
	return path
}

// baseCfg returns a minimal AuditConfig for NewClientFromConfig tests.
func baseCfg() config.AuditConfig {
	return config.AuditConfig{
		Server:           "localhost:50051",
		PrivilegedServer: "localhost:50052",
		EntityID:         "test",
		KeyVersion:       1,
		SecretKey:        "test-secret",
	}
}

// TestNewClientFromConfig_TLS verifies that when Insecure=false and CACertPath
// is empty, NewClientFromConfig dials with TLS using the system CA pool and
// returns a non-nil client.
func TestNewClientFromConfig_TLS(t *testing.T) {
	cfg := baseCfg()
	cfg.Insecure = false
	cfg.CACertPath = ""

	client, err := audit.NewClientFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewClientFromConfig(TLS, system CA) returned error: %v", err)
	}
	if client == nil {
		t.Fatal("NewClientFromConfig(TLS, system CA) returned nil client")
	}
	_ = client.Close()
}

// TestNewClientFromConfig_CustomCA verifies that when CACertPath points to a
// valid PEM file, NewClientFromConfig returns a non-nil client without error.
func TestNewClientFromConfig_CustomCA(t *testing.T) {
	caPath := generateSelfSignedCA(t)

	cfg := baseCfg()
	cfg.Insecure = false
	cfg.CACertPath = caPath

	client, err := audit.NewClientFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewClientFromConfig(custom CA) returned error: %v", err)
	}
	if client == nil {
		t.Fatal("NewClientFromConfig(custom CA) returned nil client")
	}
	_ = client.Close()
}

// TestNewClientFromConfig_CustomCA_InvalidFile verifies that when CACertPath
// points to a nonexistent file, NewClientFromConfig returns an error containing
// "reading CA cert".
func TestNewClientFromConfig_CustomCA_InvalidFile(t *testing.T) {
	cfg := baseCfg()
	cfg.Insecure = false
	cfg.CACertPath = "/nonexistent/ca.pem"

	_, err := audit.NewClientFromConfig(cfg)
	if err == nil {
		t.Fatal("expected error for nonexistent CA file, got nil")
	}
	if !strings.Contains(err.Error(), "reading CA cert") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "reading CA cert")
	}
}

// TestNewClientFromConfig_CustomCA_InvalidPEM verifies that when CACertPath
// points to a file with non-PEM content, NewClientFromConfig returns an error
// containing "failed to parse CA certificate".
func TestNewClientFromConfig_CustomCA_InvalidPEM(t *testing.T) {
	dir := t.TempDir()
	badFile := filepath.Join(dir, "bad.pem")
	if err := os.WriteFile(badFile, []byte("not a certificate"), 0600); err != nil {
		t.Fatalf("writing bad PEM: %v", err)
	}

	cfg := baseCfg()
	cfg.Insecure = false
	cfg.CACertPath = badFile

	_, err := audit.NewClientFromConfig(cfg)
	if err == nil {
		t.Fatal("expected error for invalid PEM, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse CA certificate") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "failed to parse CA certificate")
	}
}

// TestNewClientFromConfig_Insecure verifies that when Insecure=true,
// NewClientFromConfig returns a non-nil client without error (preserving the
// insecure development path per D-04).
func TestNewClientFromConfig_Insecure(t *testing.T) {
	cfg := baseCfg()
	cfg.Insecure = true

	client, err := audit.NewClientFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewClientFromConfig(insecure) returned error: %v", err)
	}
	if client == nil {
		t.Fatal("NewClientFromConfig(insecure) returned nil client")
	}
	_ = client.Close()
}

// TestNewClientFromConfig_EmptySecretKey verifies that when SecretKey is empty,
// NewClientFromConfig returns an error containing "audit.secret_key is required".
func TestNewClientFromConfig_EmptySecretKey(t *testing.T) {
	cfg := baseCfg()
	cfg.SecretKey = ""

	_, err := audit.NewClientFromConfig(cfg)
	if err == nil {
		t.Fatal("expected error for empty secret key, got nil")
	}
	if !strings.Contains(err.Error(), "audit.secret_key is required") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "audit.secret_key is required")
	}
}
