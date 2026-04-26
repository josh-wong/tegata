package audit

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/josh-wong/tegata/internal/audit/rpc"
	"github.com/josh-wong/tegata/internal/config"
	tegerrors "github.com/josh-wong/tegata/internal/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

// Sentinels for collection gRPC status codes, used in the Submit flow to
// distinguish "collection not found" from transient errors.
var (
	errCollectionNotFound = fmt.Errorf("collection does not exist")
	errCollectionExists   = fmt.Errorf("collection already exists")
)

// Client is the minimal interface for interacting with a ScalarDL Ledger.
// LedgerClient implements this interface. Tests should use a mock Client.
type Client interface {
	// Put stores an event record (objectID → hashValue) on the ledger.
	Put(ctx context.Context, objectID, hashValue string) error
	// PutWithMetadata stores an event with metadata (operation, label, timestamp).
	PutWithMetadata(ctx context.Context, objectID, hashValue string, metadata map[string]interface{}) error
	// Get retrieves all event records for objectID from the ledger.
	Get(ctx context.Context, objectID string) ([]*EventRecord, error)
	// Validate verifies the integrity of objectID against the caller-supplied
	// expectedHash from the vault. It fetches the stored record via Get and
	// performs three local checks: (1) stored hash_value == expectedHash, and
	// (2) SHA-256(stored content) == expectedHash when content is present,
	// and (3) individual metadata display fields (operation, label_hash) match
	// the authenticated content. Check (3) detects targeted tampering of
	// individual fields when hash_value and content are left intact.
	Validate(ctx context.Context, objectID, expectedHash string) (*ValidationResult, error)
	// CollectionCreate creates a new collection with the given object IDs.
	CollectionCreate(ctx context.Context, collectionID string, objectIDs []string) error
	// CollectionAdd adds object IDs to an existing collection.
	CollectionAdd(ctx context.Context, collectionID string, objectIDs []string) error
	// CollectionGet retrieves all object IDs in a collection.
	CollectionGet(ctx context.Context, collectionID string) ([]string, error)
	// RegisterCert registers the client certificate with the LedgerPrivileged
	// service. Must be called once before any Put/Get/Validate calls.
	RegisterCert(ctx context.Context, entityID string, keyVersion uint32, certPEM string) error
	// Ping sends a sentinel request and verifies connectivity. Returns
	// ErrNetworkFailed if the server is unreachable or the call times out.
	Ping(ctx context.Context) error
	// Close releases the underlying gRPC connection.
	Close() error
	// Submit stores each event as its own ScalarDL object with metadata and
	// adds it to a per-entity collection. Returns the computed hash value
	// (hex-encoded SHA-256 of the event JSON) for vault storage.
	Submit(ctx context.Context, entry QueueEntry) (string, error)
}

// EventRecord is a single record returned by a Get call.
type EventRecord struct {
	ObjectID  string
	HashValue string
	Version   int64 // ScalarDL version number (age)
	Metadata  map[string]interface{}
}

// ValidationResult is returned by Validate.
type ValidationResult struct {
	Valid       bool
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

// buildTLSConfig constructs a *tls.Config for server-only TLS. When
// caCertPath is non-empty, the CA certificate is loaded from disk and used
// as the root CA pool (for self-signed or private CA certificates). When
// caCertPath is empty, the system CA pool is used.
func buildTLSConfig(caCertPath string) (*tls.Config, error) {
	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if caCertPath != "" {
		caCert, err := os.ReadFile(caCertPath)
		if err != nil {
			return nil, fmt.Errorf("reading CA cert: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate from %s", caCertPath)
		}
		tlsCfg.RootCAs = pool
	}
	return tlsCfg, nil
}

// NewClientFromConfig creates a LedgerClient using HMAC authentication with
// the settings from an AuditConfig struct. When cfg.Insecure is true, a
// plaintext connection is used (local development only). When cfg.Insecure is
// false, the connection uses TLS with the system CA pool by default, or a
// custom CA from cfg.CACertPath for self-signed certificates.
func NewClientFromConfig(cfg config.AuditConfig) (*LedgerClient, error) {
	if cfg.SecretKey == "" {
		return nil, fmt.Errorf("audit.secret_key is required")
	}
	signer := NewHMACSigner(cfg.SecretKey)

	if cfg.Insecure {
		return NewLedgerClientInsecure(cfg.Server, cfg.PrivilegedServer,
			cfg.EntityID, cfg.KeyVersion, signer)
	}

	// Server-only TLS (SECR-06): system CA pool by default,
	// custom CA from CACertPath for self-signed certificates.
	// CertPath/KeyPath are mTLS fields for ECDSA auth -- not used here.
	tlsCfg, err := buildTLSConfig(cfg.CACertPath)
	if err != nil {
		return nil, fmt.Errorf("building TLS config: %w", err)
	}

	return NewLedgerClient(cfg.Server, cfg.PrivilegedServer, tlsCfg,
		cfg.EntityID, cfg.KeyVersion, signer)
}

// formatArgument wraps a contract argument in the ScalarDL V2 envelope that
// the server expects. The format is:
//
//	"V2" \x01 nonce \x03 functionIDs \x03 jsonArgument
//
// functionIDs is empty for direct contract calls (no functions). The same
// formatted string must be used both as the ContractArgument proto field AND
// as the contractArgument input to the signer, because the server validates
// the signature against the formatted (not raw) argument.
func formatArgument(jsonArg string, nonce string) string {
	return "V2\x01" + nonce + "\x03\x03" + jsonArg
}

// Put calls the object.Put contract on the ledger to store objectID → hashValue.
// A UUID v4 nonce is generated per request. The request is signed with the
// configured Signer before transmission.
func (c *LedgerClient) Put(ctx context.Context, objectID, hashValue string) error {
	contractID := "object.v1_0_0.Put"
	arg, err := json.Marshal(map[string]string{
		"object_id":  objectID,
		"hash_value": hashValue,
	})
	if err != nil {
		return fmt.Errorf("marshalling Put argument: %w", err)
	}

	nonce := uuid.New().String()
	formatted := formatArgument(string(arg), nonce)
	sig, err := c.signer.Sign(contractID, formatted, nonce, c.entityID, c.keyVersion)
	if err != nil {
		return fmt.Errorf("signing Put request: %w", err)
	}

	_, err = c.ledger.ExecuteContract(ctx, &rpc.ContractExecutionRequest{
		ContractId:       contractID,
		ContractArgument: formatted,
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

// PutWithMetadata stores an event with metadata in the ledger. The metadata
// contains human-readable event details (operation type, label, timestamp).
func (c *LedgerClient) PutWithMetadata(ctx context.Context, objectID, hashValue string, metadata map[string]interface{}) error {
	contractID := "object.v1_0_0.Put"
	argMap := map[string]interface{}{
		"object_id":  objectID,
		"hash_value": hashValue,
	}
	if metadata != nil {
		argMap["metadata"] = metadata
	}
	arg, err := json.Marshal(argMap)
	if err != nil {
		return fmt.Errorf("marshalling Put argument: %w", err)
	}

	nonce := uuid.New().String()
	formatted := formatArgument(string(arg), nonce)
	sig, err := c.signer.Sign(contractID, formatted, nonce, c.entityID, c.keyVersion)
	if err != nil {
		return fmt.Errorf("signing Put request: %w", err)
	}

	_, err = c.ledger.ExecuteContract(ctx, &rpc.ContractExecutionRequest{
		ContractId:       contractID,
		ContractArgument: formatted,
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

// CollectionCreate creates a new collection in the ledger.
func (c *LedgerClient) CollectionCreate(ctx context.Context, collectionID string, objectIDs []string) error {
	contractID := "collection.v1_0_0.Create"
	arg, err := json.Marshal(map[string]interface{}{
		"collection_id": collectionID,
		"object_ids":    objectIDs,
	})
	if err != nil {
		return fmt.Errorf("marshalling CollectionCreate argument: %w", err)
	}

	nonce := uuid.New().String()
	formatted := formatArgument(string(arg), nonce)
	sig, err := c.signer.Sign(contractID, formatted, nonce, c.entityID, c.keyVersion)
	if err != nil {
		return fmt.Errorf("signing CollectionCreate request: %w", err)
	}

	_, err = c.ledger.ExecuteContract(ctx, &rpc.ContractExecutionRequest{
		ContractId:       contractID,
		ContractArgument: formatted,
		EntityId:         c.entityID,
		KeyVersion:       c.keyVersion,
		Signature:        sig,
		Nonce:            nonce,
	})
	if err != nil {
		if s, ok := status.FromError(err); ok {
			switch s.Code() {
			case codes.AlreadyExists:
				return fmt.Errorf("%w: %s", errCollectionExists, err)
			case codes.Internal:
				if s.Message() == "" {
					return fmt.Errorf("%w: %s", errCollectionExists, err)
				}
			}
		}
		return fmt.Errorf("%w: CollectionCreate failed: %s", tegerrors.ErrNetworkFailed, err)
	}
	return nil
}

// CollectionAdd adds object IDs to an existing collection.
func (c *LedgerClient) CollectionAdd(ctx context.Context, collectionID string, objectIDs []string) error {
	contractID := "collection.v1_0_0.Add"
	arg, err := json.Marshal(map[string]interface{}{
		"collection_id": collectionID,
		"object_ids":    objectIDs,
	})
	if err != nil {
		return fmt.Errorf("marshalling CollectionAdd argument: %w", err)
	}

	nonce := uuid.New().String()
	formatted := formatArgument(string(arg), nonce)
	sig, err := c.signer.Sign(contractID, formatted, nonce, c.entityID, c.keyVersion)
	if err != nil {
		return fmt.Errorf("signing CollectionAdd request: %w", err)
	}

	_, err = c.ledger.ExecuteContract(ctx, &rpc.ContractExecutionRequest{
		ContractId:       contractID,
		ContractArgument: formatted,
		EntityId:         c.entityID,
		KeyVersion:       c.keyVersion,
		Signature:        sig,
		Nonce:            nonce,
	})
	if err != nil {
		if s, ok := status.FromError(err); ok {
			switch s.Code() {
			case codes.NotFound, codes.InvalidArgument:
				return fmt.Errorf("%w: %s", errCollectionNotFound, err)
			case codes.Internal:
				if s.Message() == "" {
					return fmt.Errorf("%w: %s", errCollectionNotFound, err)
				}
			}
		}
		return fmt.Errorf("%w: CollectionAdd failed: %s", tegerrors.ErrNetworkFailed, err)
	}
	return nil
}

// collectionGetResult is the JSON shape returned by collection.Get.
type collectionGetResult struct {
	ObjectIDs []string `json:"object_ids"`
}

// CollectionGet retrieves all object IDs in a collection.
func (c *LedgerClient) CollectionGet(ctx context.Context, collectionID string) ([]string, error) {
	contractID := "collection.v1_0_0.Get"
	arg, err := json.Marshal(map[string]string{"collection_id": collectionID})
	if err != nil {
		return nil, fmt.Errorf("marshalling CollectionGet argument: %w", err)
	}

	nonce := uuid.New().String()
	formatted := formatArgument(string(arg), nonce)
	sig, err := c.signer.Sign(contractID, formatted, nonce, c.entityID, c.keyVersion)
	if err != nil {
		return nil, fmt.Errorf("signing CollectionGet request: %w", err)
	}

	resp, err := c.ledger.ExecuteContract(ctx, &rpc.ContractExecutionRequest{
		ContractId:       contractID,
		ContractArgument: formatted,
		EntityId:         c.entityID,
		KeyVersion:       c.keyVersion,
		Signature:        sig,
		Nonce:            nonce,
	})
	if err != nil {
		// Detect when the collection doesn't exist (e.g. after a wipe).
		// ScalarDL HashStore returns Internal with an empty description when the
		// collection or its backing object does not exist; NotFound/InvalidArgument
		// cover other ledger implementations.
		if s, ok := status.FromError(err); ok {
			switch s.Code() {
			case codes.NotFound, codes.InvalidArgument:
				return nil, fmt.Errorf("%w: %s", errCollectionNotFound, err)
			case codes.Internal:
				if s.Message() == "" {
					return nil, fmt.Errorf("%w: %s", errCollectionNotFound, err)
				}
			}
		}
		return nil, fmt.Errorf("%w: CollectionGet failed: %s", tegerrors.ErrNetworkFailed, err)
	}

	var result collectionGetResult
	if raw := resp.GetContractResult(); raw != "" {
		if err := json.Unmarshal([]byte(raw), &result); err != nil {
			return nil, fmt.Errorf("parsing CollectionGet result: %w", err)
		}
	}
	return result.ObjectIDs, nil
}

// getResult is the JSON shape returned by the object.Get contract.
type getResult struct {
	ObjectID  string                 `json:"object_id"`
	HashValue string                 `json:"hash_value"`
	Age       int64                  `json:"age"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Get calls the object.Get contract and returns all records for objectID.
func (c *LedgerClient) Get(ctx context.Context, objectID string) ([]*EventRecord, error) {
	contractID := "object.v1_0_0.Get"
	arg, err := json.Marshal(map[string]string{"object_id": objectID})
	if err != nil {
		return nil, fmt.Errorf("marshalling Get argument: %w", err)
	}

	nonce := uuid.New().String()
	formatted := formatArgument(string(arg), nonce)
	sig, err := c.signer.Sign(contractID, formatted, nonce, c.entityID, c.keyVersion)
	if err != nil {
		return nil, fmt.Errorf("signing Get request: %w", err)
	}

	resp, err := c.ledger.ExecuteContract(ctx, &rpc.ContractExecutionRequest{
		ContractId:       contractID,
		ContractArgument: formatted,
		EntityId:         c.entityID,
		KeyVersion:       c.keyVersion,
		Signature:        sig,
		Nonce:            nonce,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: Get contract failed: %s", tegerrors.ErrNetworkFailed, err)
	}

	// Parse the contract_result JSON. The response may be a single object
	// (latest version) or an array of objects (all versions).
	var results []getResult
	if raw := resp.GetContractResult(); raw != "" {
		raw = strings.TrimSpace(raw)
		if strings.HasPrefix(raw, "[") {
			if err := json.Unmarshal([]byte(raw), &results); err != nil {
				return nil, fmt.Errorf("parsing Get result array: %w", err)
			}
		} else {
			var single getResult
			if err := json.Unmarshal([]byte(raw), &single); err != nil {
				return nil, fmt.Errorf("parsing Get result: %w", err)
			}
			results = []getResult{single}
		}
	}

	records := make([]*EventRecord, len(results))
	for i, r := range results {
		records[i] = &EventRecord{
			ObjectID:  r.ObjectID,
			HashValue: r.HashValue,
			Version:   r.Age,
			Metadata:  r.Metadata,
		}
	}
	return records, nil
}

// Validate verifies the integrity of objectID against the caller-supplied
// expectedHash from the vault. It fetches the stored record via Get and
// performs three local checks:
//
//  1. The stored hash_value must equal expectedHash — detects direct hash
//     manipulation in the ScalarDL database.
//  2. If a "content" field is present in the record metadata, its SHA-256
//     must equal expectedHash — detects content replacement.
//  3. Individual metadata display fields (operation, label_hash) must match
//     the authenticated content — detects targeted field tampering when
//     hash_value and content are left intact (e.g. changing
//     operation "hotp" → "totp" without touching the other fields).
//
// Events submitted before content storage was introduced pass check (1) only.
func (c *LedgerClient) Validate(ctx context.Context, objectID, expectedHash string) (*ValidationResult, error) {
	records, err := c.Get(ctx, objectID)
	if err != nil {
		return nil, fmt.Errorf("fetching record for validation: %w", err)
	}
	if len(records) == 0 {
		return &ValidationResult{
			Valid:       false,
			ErrorDetail: "no record found in ledger",
		}, nil
	}
	// Each event uses a UUID EventID written exactly once, so len > 1 is
	// unexpected and is itself evidence of tampering or a replay.
	if len(records) > 1 {
		return &ValidationResult{
			Valid:       false,
			ErrorDetail: "unexpected multiple versions for event record",
		}, nil
	}
	record := records[0]

	// Check 1: stored hash_value must match the vault hash.
	if record.HashValue != expectedHash {
		return &ValidationResult{
			Valid:       false,
			ErrorDetail: "record hash has been altered",
		}, nil
	}

	// Checks 2 & 3: only run when content is present in metadata (events
	// submitted after D-17). Check 2 recomputes SHA-256(content) to confirm
	// content itself has not been replaced. Check 3 cross-checks individual
	// display fields against the now-authenticated content to catch targeted
	// field tampering (e.g. operation changed while content is left intact).
	if content, ok := record.Metadata["content"].(string); ok && content != "" {
		// Check 2: recompute SHA-256 of stored event content and compare.
		sum := sha256.Sum256([]byte(content))
		if hex.EncodeToString(sum[:]) != expectedHash {
			return &ValidationResult{
				Valid:       false,
				ErrorDetail: "event content has been altered",
			}, nil
		}

		// Check 3: individual display fields must match the authenticated content.
		var evt AuthEvent
		if jsonErr := json.Unmarshal([]byte(content), &evt); jsonErr == nil {
			if op, ok := record.Metadata["operation"].(string); ok && op != evt.OperationType {
				return &ValidationResult{
					Valid:       false,
					ErrorDetail: "event type field has been altered",
				}, nil
			}
			if lh, ok := record.Metadata["label_hash"].(string); ok && lh != evt.LabelHash {
				return &ValidationResult{
					Valid:       false,
					ErrorDetail: "credential field has been altered",
				}, nil
			}
		}
	}

	return &ValidationResult{Valid: true}, nil
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

// RegisterSecret registers an HMAC secret key with the ScalarDL Ledger for
// the given entity and key version. Used with HMAC authentication mode.
// Idempotent — AlreadyExists is treated as success.
func (c *LedgerClient) RegisterSecret(ctx context.Context, entityID string, keyVersion uint32, secretKey string) error {
	_, err := c.privileged.RegisterSecret(ctx, &rpc.SecretRegistrationRequest{
		EntityId:   entityID,
		KeyVersion: keyVersion,
		SecretKey:  secretKey,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.AlreadyExists {
			return nil
		}
		return fmt.Errorf("%w: RegisterSecret failed: %s", tegerrors.ErrNetworkFailed, err)
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

// Close releases the underlying gRPC connections and zeros signer key material.
func (c *LedgerClient) Close() error {
	if z, ok := c.signer.(interface{ Zero() }); ok {
		z.Zero()
	}
	err := c.conn.Close()
	if c.privilegedConn != nil {
		if err2 := c.privilegedConn.Close(); err2 != nil && err == nil {
			err = err2
		}
	}
	return err
}

// Submit implements the Submitter interface. Each event is stored as its own
// ScalarDL object with metadata (operation, label hash, timestamp) and added
// to a per-entity collection for listing. Returns the computed hash value
// (hex-encoded SHA-256 of the event JSON) so callers can store it in the vault.
func (c *LedgerClient) Submit(ctx context.Context, entry QueueEntry) (string, error) {
	entryJSON, err := json.Marshal(entry.Event)
	if err != nil {
		return "", fmt.Errorf("serialising queue entry for submission: %w", err)
	}
	sum := sha256.Sum256(entryJSON)
	// Capture the serialised event before zeroing. Stored as "content" in
	// ScalarDL metadata so validation can recompute SHA-256(content) and
	// detect metadata tampering even when hash_value is left intact (D-17).
	// The event JSON contains only hashed/derived fields, not raw secrets.
	eventContent := string(entryJSON)
	for i := range entryJSON {
		entryJSON[i] = 0
	}
	hashValue := hex.EncodeToString(sum[:])

	metadata := map[string]interface{}{
		"operation":  entry.Event.OperationType,
		"label_hash": entry.Event.LabelHash,
		"timestamp":  entry.Event.Timestamp.Unix(),
		"content":    eventContent,
	}

	objectID := entry.Event.EventID
	if err := c.PutWithMetadata(ctx, objectID, hashValue, metadata); err != nil {
		return "", err
	}

	// Add the event to the entity's audit collection. If the collection
	// doesn't exist yet, create it with this event as the first member.
	// A race between concurrent Submit calls can cause CollectionCreate to
	// return AlreadyExists — in that case retry CollectionAdd.
	collectionID := CollectionID(c.entityID)
	if err := c.CollectionAdd(ctx, collectionID, []string{objectID}); err != nil {
		// Only fall through to CollectionCreate when the collection does
		// not exist. Transient or other errors should propagate immediately.
		if !tegerrors.Is(err, errCollectionNotFound) {
			return "", fmt.Errorf("adding event to collection: %w", err)
		}
		createErr := c.CollectionCreate(ctx, collectionID, []string{objectID})
		if createErr != nil {
			if tegerrors.Is(createErr, errCollectionExists) {
				// Another caller created the collection between our Add
				// and Create — retry Add now that the collection exists.
				if retryErr := c.CollectionAdd(ctx, collectionID, []string{objectID}); retryErr != nil {
					return "", fmt.Errorf("adding event to collection after retry: %w", retryErr)
				}
			} else {
				return "", fmt.Errorf("creating audit collection: %w", createErr)
			}
		}
	}

	return hashValue, nil
}

