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
type LedgerClient struct {
	conn       *grpc.ClientConn
	ledger     rpc.LedgerClient
	privileged rpc.LedgerPrivilegedClient
	signer     Signer
	entityID   string
	keyVersion uint32
}

// NewLedgerClient dials the ScalarDL Ledger at addr using TLS. tlsCfg must be
// non-nil — plaintext connections are refused (SECR-06). Both the Ledger and
// LedgerPrivileged services share the same underlying connection; the caller
// should call RegisterCert on port 50052 if using the default setup.
func NewLedgerClient(addr string, tlsCfg *tls.Config, entityID string, keyVersion uint32, signer Signer) (*LedgerClient, error) {
	if tlsCfg == nil {
		return nil, fmt.Errorf("TLS config is required for ScalarDL Ledger connection (SECR-06): plaintext connections are not permitted")
	}

	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)),
	)
	if err != nil {
		return nil, fmt.Errorf("dialing ScalarDL Ledger at %s: %w", addr, err)
	}

	return &LedgerClient{
		conn:       conn,
		ledger:     rpc.NewLedgerClient(conn),
		privileged: rpc.NewLedgerPrivilegedClient(conn),
		signer:     signer,
		entityID:   entityID,
		keyVersion: keyVersion,
	}, nil
}

// NewLedgerClientFromConn creates a LedgerClient from an existing grpc.ClientConn.
// This is used in tests to inject a bufconn-backed connection without TLS.
// Production code should use NewLedgerClient which enforces TLS.
func NewLedgerClientFromConn(conn *grpc.ClientConn, signer Signer, entityID string, keyVersion uint32) *LedgerClient {
	return &LedgerClient{
		conn:       conn,
		ledger:     rpc.NewLedgerClient(conn),
		privileged: rpc.NewLedgerPrivilegedClient(conn),
		signer:     signer,
		entityID:   entityID,
		keyVersion: keyVersion,
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
		return fmt.Errorf("%w: RegisterCert failed: %s", tegerrors.ErrNetworkFailed, err)
	}
	return nil
}

// Ping sends a sentinel contract execution with a 3-second timeout to verify
// connectivity. Returns ErrNetworkFailed if the connection is refused or times out.
// Note: The server will likely return an error for the unknown "object.Ping" contract,
// but a gRPC-level response (even an error) confirms the connection is alive.
func (c *LedgerClient) Ping(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	nonce := uuid.New().String()
	arg := `{"object_id":"__ping__"}`
	sig, err := c.signer.Sign("object.Ping", arg, nonce, c.entityID, c.keyVersion)
	if err != nil {
		return fmt.Errorf("signing Ping request: %w", err)
	}

	_, err = c.ledger.ExecuteContract(pingCtx, &rpc.ContractExecutionRequest{
		ContractId:       "object.Ping",
		ContractArgument: arg,
		EntityId:         c.entityID,
		KeyVersion:       c.keyVersion,
		Signature:        sig,
		Nonce:            nonce,
	})
	if err != nil {
		// Context deadline exceeded → network failure.
		if pingCtx.Err() != nil {
			return fmt.Errorf("%w: ping timed out after 3 seconds", tegerrors.ErrNetworkFailed)
		}
		// A gRPC application-level error (e.g. contract not found, Unimplemented)
		// means the server is reachable — treat as connectivity success.
		// Only Unavailable indicates a transport-level failure.
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.Unavailable {
			return fmt.Errorf("%w: ping failed: %s", tegerrors.ErrNetworkFailed, err)
		}
	}
	return nil
}

// Close releases the underlying gRPC connection.
func (c *LedgerClient) Close() error {
	return c.conn.Close()
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

