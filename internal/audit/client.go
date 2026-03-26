package audit

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
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
	// PutWithMetadata stores an event with metadata (operation, label, timestamp).
	PutWithMetadata(ctx context.Context, objectID, hashValue string, metadata map[string]interface{}) error
	// Get retrieves all event records for objectID from the ledger.
	Get(ctx context.Context, objectID string) ([]*EventRecord, error)
	// Validate verifies the integrity of all records for objectID on the ledger.
	Validate(ctx context.Context, objectID string) (*ValidationResult, error)
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
	// adds it to a per-entity collection.
	Submit(ctx context.Context, entry QueueEntry) error
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

// NewClientFromConfig creates a LedgerClient using HMAC authentication.
// Returns an error if secretKey is empty or if insecure is false (TLS mode is
// not yet supported with HMAC auth).
//
// TODO(#22): Add TLS support for HMAC authentication. Currently only insecure
// (plaintext) connections work with HMAC. The ECDSA path supports TLS via
// NewLedgerClient, but HMAC+TLS requires building a tls.Config without client
// certificates (server-side TLS only). Until this is implemented, production
// deployments should use network-level encryption (e.g. VPN, SSH tunnel).
func NewClientFromConfig(server, privilegedServer, entityID string, keyVersion uint32, secretKey string, insecure bool) (*LedgerClient, error) {
	if secretKey == "" {
		return nil, fmt.Errorf("audit.secret_key is required")
	}
	signer := NewHMACSigner(secretKey)

	if insecure {
		return NewLedgerClientInsecure(server, privilegedServer, entityID, keyVersion, signer)
	}

	return nil, fmt.Errorf("TLS mode not yet supported with HMAC auth — set insecure = true in tegata.toml (see #22)")
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
	})
	if err != nil {
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
	})
	if err != nil {
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
	})
	if err != nil {
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

// validateResult is the JSON shape returned by the object.Validate contract.
// Matches the official ScalarDL generic contracts response schema.
type validateResult struct {
	Status         string   `json:"status"`
	Details        string   `json:"details"`
	FaultyVersions []string `json:"faulty_versions"`
}

// Validate verifies the integrity of all records for objectID on the ledger.
// It uses a two-step flow: first calls object.Get to retrieve stored records,
// then calls object.Validate with a versions array built from those records.
// If Get returns no records, it returns Valid=true with EventCount=0 without
// calling object.Validate (nothing to validate).
func (c *LedgerClient) Validate(ctx context.Context, objectID string) (*ValidationResult, error) {
	// Step 1: Retrieve all stored records via object.Get.
	records, err := c.Get(ctx, objectID)
	if err != nil {
		return nil, fmt.Errorf("retrieving records for validation: %w", err)
	}

	// If no records exist, there is nothing to validate.
	if len(records) == 0 {
		return &ValidationResult{Valid: true, EventCount: 0}, nil
	}

	// Step 2: Build versions array from retrieved records.
	versions := make([]map[string]string, len(records))
	for i, r := range records {
		versions[i] = map[string]string{
			"version_id": r.ObjectID,
			"hash_value": r.HashValue,
		}
	}

	// Step 3: Marshal the validate argument with object_id and versions.
	contractID := "object.v1_0_0.Validate"
	arg, err := json.Marshal(map[string]interface{}{
		"object_id": objectID,
		"versions":  versions,
	})
	if err != nil {
		return nil, fmt.Errorf("marshalling Validate argument: %w", err)
	}

	// Step 4: Sign and execute object.Validate.
	nonce := uuid.New().String()
	formatted := formatArgument(string(arg), nonce)
	sig, err := c.signer.Sign(contractID, formatted, nonce, c.entityID, c.keyVersion)
	if err != nil {
		return nil, fmt.Errorf("signing Validate request: %w", err)
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
		return nil, fmt.Errorf("%w: Validate contract failed: %s", tegerrors.ErrNetworkFailed, err)
	}

	// Step 5: Parse and map the response.
	var vr validateResult
	if resp.GetContractResult() != "" {
		if err := json.Unmarshal([]byte(resp.GetContractResult()), &vr); err != nil {
			return nil, fmt.Errorf("parsing Validate result: %w", err)
		}
	}

	return &ValidationResult{
		Valid:       vr.Status == "correct",
		EventCount:  len(records),
		ErrorDetail: vr.Details,
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
// to a per-entity collection for listing.
func (c *LedgerClient) Submit(ctx context.Context, entry QueueEntry) error {
	entryJSON, err := json.Marshal(entry.Event)
	if err != nil {
		return fmt.Errorf("serialising queue entry for submission: %w", err)
	}
	sum := sha256.Sum256(entryJSON)
	for i := range entryJSON {
		entryJSON[i] = 0
	}
	hashValue := hex.EncodeToString(sum[:])

	metadata := map[string]interface{}{
		"operation":  entry.Event.OperationType,
		"label_hash": entry.Event.LabelHash,
		"timestamp":  entry.Event.Timestamp.Unix(),
	}

	objectID := entry.Event.EventID
	if err := c.PutWithMetadata(ctx, objectID, hashValue, metadata); err != nil {
		return err
	}

	// Add the event to the entity's audit collection. If the collection
	// doesn't exist yet, create it with this event as the first member.
	collectionID := CollectionID(c.entityID)
	if err := c.CollectionAdd(ctx, collectionID, []string{objectID}); err != nil {
		// Collection might not exist yet — try creating it.
		if createErr := c.CollectionCreate(ctx, collectionID, []string{objectID}); createErr != nil {
			return fmt.Errorf("adding event to collection: add=%v, create=%v", err, createErr)
		}
	}

	return nil
}

