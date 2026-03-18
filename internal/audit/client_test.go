package audit_test

import (
	"context"
	"crypto/tls"
	"net"
	"testing"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/audit/rpc"
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
	t.Cleanup(func() { srv.Stop(); lis.Close() })

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("bufconn dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// TestClient_RequiresTLS verifies that NewLedgerClient rejects a nil TLS config.
func TestClient_RequiresTLS(t *testing.T) {
	_, err := audit.NewLedgerClient("localhost:50051", nil, "test-entity", 1, &audit.NoOpSigner{})
	if err == nil {
		t.Fatal("expected error for nil TLS config, got nil")
	}
}

// TestClient_PutCallsSigner verifies that Put calls Sign exactly once with the
// correct fields (contractID="object.Put", entityID, keyVersion).
func TestClient_PutCallsSigner(t *testing.T) {
	signer := &mockSigner{sig: []byte("fake-sig")}
	ledgerSrv := &mockLedgerServer{}
	privSrv := &mockPrivilegedServer{}
	conn := newBufconnServer(t, ledgerSrv, privSrv)

	client := audit.NewLedgerClientFromConn(conn, signer, "test-entity", 1)
	defer client.Close()

	err := client.Put(context.Background(), "obj-123", "deadbeef")
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	if len(signer.calls) != 1 {
		t.Fatalf("expected 1 Sign call, got %d", len(signer.calls))
	}
	call := signer.calls[0]
	if call.contractID != "object.Put" {
		t.Errorf("contractID = %q, want %q", call.contractID, "object.Put")
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

	client := audit.NewLedgerClientFromConn(conn, signer, "test-entity", 1)
	defer client.Close()

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

// TestClient_TLSEnforced_WithValidConfig verifies that NewLedgerClient accepts a
// non-nil TLS config (actual dial failure expected since there is no server).
func TestClient_TLSEnforced_WithValidConfig(t *testing.T) {
	cfg := &tls.Config{InsecureSkipVerify: true} //nolint:gosec // test only
	_, err := audit.NewLedgerClient("localhost:59999", cfg, "entity", 1, &audit.NoOpSigner{})
	// grpc.NewClient is lazy — the error (if any) occurs on first RPC, not here.
	// The important thing is that no error is returned for a non-nil TLS config.
	if err != nil {
		t.Errorf("NewLedgerClient with valid TLS config returned error: %v", err)
	}
}
