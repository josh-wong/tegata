package audit

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"crypto/sha256"

	"github.com/google/uuid"
	"github.com/josh-wong/tegata/internal/audit/rpc"
	tegerrors "github.com/josh-wong/tegata/internal/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

// Client is the minimal interface for interacting with a ScalarDL Ledger.
// LedgerClient implements this interface. Tests should use a mock Client.
type Client interface {
	// Put stores an event record (objectID → hashValue) on the ledger.
	Put(ctx context.Context, objectID, hashValue string) error
	// Get retrieves all event records for objectID from the ledger.
	Get(ctx context.Context, objectID string) ([]*EventRecord, error)
	// Validate verifies the integrity of all records for objectID on the ledger.
	Validate(ctx context.Context, objectID string) (*ValidationResult, error)
	// RegisterCert registers the client certificate with the LedgerPrivileged
	// service. Must be called once before any Put/Get/Validate calls.
	RegisterCert(ctx context.Context, entityID string, keyVersion uint32, certPEM string) error
	// Ping sends a sentinel request and verifies connectivity. Returns
	// ErrNetworkFailed if the server is unreachable or the call times out.
	Ping(ctx context.Context) error
	// Close releases the underlying gRPC connection.
	Close() error
	// Submit implements the Submitter interface from the offline queue package.
	// It calls Put with the entry's EventID as objectID and hex(SHA-256(entryJSON)) as hashValue.
	Submit(ctx context.Context, entry QueueEntry) error
}

// EventRecord is a single record returned by a Get call.
type EventRecord struct {
	ObjectID  string
	HashValue string
	Age       int64
}

// ValidationResult is returned by Validate.
type ValidationResult struct {
	Valid       bool
	EventCount  int
	ErrorDetail string
}

// LedgerClient is the production implementation of Client backed by a real
// ScalarDL Ledger gRPC connection. It requires a non-nil TLS config (SECR-06:
// plaintext connections to the audit ledger are not permitted).
// The Ledger service (ExecuteContract etc.) and LedgerPrivileged service
// (RegisterCert) listen on separate ports and require separate connections.
type LedgerClient struct {
	conn           *grpc.ClientConn
	privilegedConn *grpc.ClientConn
	ledger         rpc.LedgerClient
	privileged     rpc.LedgerPrivilegedClient
	signer         Signer
	entityID       string
	keyVersion     uint32
}

// NewLedgerClient dials the ScalarDL Ledger and LedgerPrivileged services using
// TLS. tlsCfg must be non-nil — plaintext connections are refused (SECR-06).
// addr is the Ledger service address (default port 50051); privilegedAddr is
// the LedgerPrivileged service address (default port 50052).
func NewLedgerClient(addr, privilegedAddr string, tlsCfg *tls.Config, entityID string, keyVersion uint32, signer Signer) (*LedgerClient, error) {
	if tlsCfg == nil {
		return nil, fmt.Errorf("TLS config is required for ScalarDL Ledger connection (SECR-06): plaintext connections are not permitted")
	}

	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)),
	)
	if err != nil {
		return nil, fmt.Errorf("dialing ScalarDL Ledger at %s: %w", addr, err)
	}

	privConn, err := grpc.NewClient(privilegedAddr,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)),
	)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("dialing ScalarDL LedgerPrivileged at %s: %w", privilegedAddr, err)
	}

	return &LedgerClient{
		conn:           conn,
		privilegedConn: privConn,
		ledger:         rpc.NewLedgerClient(conn),
		privileged:     rpc.NewLedgerPrivilegedClient(privConn),
		signer:         signer,
		entityID:       entityID,
		keyVersion:     keyVersion,
	}, nil
}

// NewLedgerClientInsecure dials the ScalarDL Ledger and LedgerPrivileged
// services without TLS. For local development only — set insecure = true in
// tegata.toml. Never use in production.
func NewLedgerClientInsecure(addr, privilegedAddr string, entityID string, keyVersion uint32, signer Signer) (*LedgerClient, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("dialing ScalarDL Ledger at %s (insecure): %w", addr, err)
	}

	privConn, err := grpc.NewClient(privilegedAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("dialing ScalarDL LedgerPrivileged at %s (insecure): %w", privilegedAddr, err)
	}

	return &LedgerClient{
		conn:           conn,
		privilegedConn: privConn,
		ledger:         rpc.NewLedgerClient(conn),
		privileged:     rpc.NewLedgerPrivilegedClient(privConn),
		signer:         signer,
		entityID:       entityID,
		keyVersion:     keyVersion,
	}, nil
}

// NewLedgerClientFromConn creates a LedgerClient from existing grpc.ClientConns.
// This is used in tests to inject bufconn-backed connections without TLS.
// Production code should use NewLedgerClient which enforces TLS.
// privConn may be nil, in which case the same conn is used for both services.
func NewLedgerClientFromConn(conn *grpc.ClientConn, privConn *grpc.ClientConn, signer Signer, entityID string, keyVersion uint32) *LedgerClient {
	if privConn == nil {
		privConn = conn
	}
	return &LedgerClient{
		conn:           conn,
		privilegedConn: privConn,
		ledger:         rpc.NewLedgerClient(conn),
		privileged:     rpc.NewLedgerPrivilegedClient(privConn),
		signer:         signer,
		entityID:       entityID,
		keyVersion:     keyVersion,
	}
}

// Put calls the object.Put contract on the ledger to store objectID → hashValue.
// A UUID v4 nonce is generated per request. The request is signed with the
// configured Signer before transmission.
func (c *LedgerClient) Put(ctx context.Context, objectID, hashValue string) error {
	contractID := "object.Put"
	arg, err := json.Marshal(map[string]string{
		"object_id":  objectID,
		"hash_value": hashValue,
	})
	if err != nil {
		return fmt.Errorf("marshalling Put argument: %w", err)
	}

	nonce := uuid.New().String()
	sig, err := c.signer.Sign(contractID, string(arg), nonce, c.entityID, c.keyVersion)
	if err != nil {
		return fmt.Errorf("signing Put request: %w", err)
	}

	_, err = c.ledger.ExecuteContract(ctx, &rpc.ContractExecutionRequest{
		ContractId:       contractID,
		ContractArgument: string(arg),
		EntityId:         c.entityID,
		KeyVersion:       c.keyVersion,
		Signature:        sig,
		Nonce:            nonce,
	})
	if err != nil {
		return fmt.Errorf("%w: Put contract failed: %s", tegerrors.ErrNetworkFailed, err)
	}
	return nil
}

// getResult is the JSON shape returned by the object.Get contract.
type getResult struct {
	ObjectID  string `json:"object_id"`
	HashValue string `json:"hash_value"`
	Age       int64  `json:"age"`
}

// Get calls the object.Get contract and returns all records for objectID.
func (c *LedgerClient) Get(ctx context.Context, objectID string) ([]*EventRecord, error) {
	contractID := "object.Get"
	arg, err := json.Marshal(map[string]string{"object_id": objectID})
	if err != nil {
		return nil, fmt.Errorf("marshalling Get argument: %w", err)
	}

	nonce := uuid.New().String()
	sig, err := c.signer.Sign(contractID, string(arg), nonce, c.entityID, c.keyVersion)
	if err != nil {
		return nil, fmt.Errorf("signing Get request: %w", err)
	}

	resp, err := c.ledger.ExecuteContract(ctx, &rpc.ContractExecutionRequest{
		ContractId:       contractID,
		ContractArgument: string(arg),
		EntityId:         c.entityID,
		KeyVersion:       c.keyVersion,
		Signature:        sig,
		Nonce:            nonce,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: Get contract failed: %s", tegerrors.ErrNetworkFailed, err)
	}

	// Parse the contract_result JSON.
	var results []getResult
	if resp.GetContractResult() != "" {
		if err := json.Unmarshal([]byte(resp.GetContractResult()), &results); err != nil {
			return nil, fmt.Errorf("parsing Get result: %w", err)
		}
	}

	records := make([]*EventRecord, len(results))
	for i, r := range results {
		records[i] = &EventRecord{
			ObjectID:  r.ObjectID,
			HashValue: r.HashValue,
			Age:       r.Age,
		}
	}
	return records, nil
}

// validateResult is the JSON shape returned by the object.Validate contract.
type validateResult struct {
	Valid       bool   `json:"valid"`
	EventCount  int    `json:"event_count"`
	ErrorDetail string `json:"error_detail"`
}

// Validate calls the object.Validate contract and checks ledger integrity for objectID.
func (c *LedgerClient) Validate(ctx context.Context, objectID string) (*ValidationResult, error) {
	contractID := "object.Validate"
	arg, err := json.Marshal(map[string]string{"object_id": objectID})
	if err != nil {
		return nil, fmt.Errorf("marshalling Validate argument: %w", err)
	}

	nonce := uuid.New().String()
	sig, err := c.signer.Sign(contractID, string(arg), nonce, c.entityID, c.keyVersion)
	if err != nil {
		return nil, fmt.Errorf("signing Validate request: %w", err)
	}

	resp, err := c.ledger.ExecuteContract(ctx, &rpc.ContractExecutionRequest{
		ContractId:       contractID,
		ContractArgument: string(arg),
		EntityId:         c.entityID,
		KeyVersion:       c.keyVersion,
		Signature:        sig,
		Nonce:            nonce,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: Validate contract failed: %s", tegerrors.ErrNetworkFailed, err)
	}

	var vr validateResult
	if resp.GetContractResult() != "" {
		if err := json.Unmarshal([]byte(resp.GetContractResult()), &vr); err != nil {
			return nil, fmt.Errorf("parsing Validate result: %w", err)
		}
	}

	return &ValidationResult{
		Valid:       vr.Valid,
		EventCount:  vr.EventCount,
		ErrorDetail: vr.ErrorDetail,
	}, nil
}

// RegisterCert registers the client certificate on the LedgerPrivileged service.
// This must be called once per entity/keyVersion pair before submitting contracts.
// Uses the LedgerPrivileged gRPC service (not the Ledger service).
func (c *LedgerClient) RegisterCert(ctx context.Context, entityID string, keyVersion uint32, certPEM string) error {
	_, err := c.privileged.RegisterCert(ctx, &rpc.CertificateRegistrationRequest{
		EntityId:   entityID,
		KeyVersion: keyVersion,
		CertPem:    certPEM,
	})
	if err != nil {
		// AlreadyExists means the certificate is already registered — idempotent success.
		if st, ok := status.FromError(err); ok && st.Code() == codes.AlreadyExists {
			return nil
		}
		return fmt.Errorf("%w: RegisterCert failed: %s", tegerrors.ErrNetworkFailed, err)
	}
	return nil
}

// Ping calls the standard gRPC health check service on the ledger connection
// with a 5-second timeout to verify connectivity. Returns ErrNetworkFailed if
// the server is unreachable or the call times out.
func (c *LedgerClient) Ping(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	hc := grpc_health_v1.NewHealthClient(c.conn)
	resp, err := hc.Check(pingCtx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		if pingCtx.Err() != nil {
			return fmt.Errorf("%w: ping timed out after 5 seconds", tegerrors.ErrNetworkFailed)
		}
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.Unavailable {
			return fmt.Errorf("%w: ping failed: %s", tegerrors.ErrNetworkFailed, err)
		}
		// Any other gRPC error (e.g. Unimplemented) still confirms the server
		// is reachable at the transport level.
		return nil
	}
	if resp.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
		return fmt.Errorf("%w: ledger health status: %s", tegerrors.ErrNetworkFailed, resp.GetStatus())
	}
	return nil
}

// Close releases the underlying gRPC connections.
func (c *LedgerClient) Close() error {
	err := c.conn.Close()
	if c.privilegedConn != nil {
		if err2 := c.privilegedConn.Close(); err == nil {
			err = err2
		}
	}
	return err
}

// Submit implements the Submitter interface. It calls Put with:
//   - objectID = entry.Event.EventID
//   - hashValue = hex(SHA-256(JSON(entry)))
//
// This allows the LedgerClient to be used directly as the Submitter for Queue.Flush.
func (c *LedgerClient) Submit(ctx context.Context, entry QueueEntry) error {
	entryJSON, err := json.Marshal(entry.Event)
	if err != nil {
		return fmt.Errorf("serialising queue entry for submission: %w", err)
	}
	sum := sha256.Sum256(entryJSON)
	hashValue := hex.EncodeToString(sum[:])

	return c.Put(ctx, entry.Event.EventID, hashValue)
}

