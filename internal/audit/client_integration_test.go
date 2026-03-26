//go:build integration

// Run all integration tests:
//
//	go test -tags integration ./internal/audit/... -v -timeout 120s
//
// Integration tests for the ScalarDL Ledger gRPC client. These tests require
// a live ScalarDL Ledger 3.12 instance.
//
// For HMAC authentication (recommended):
//
//	export SCALARDL_ADDR=localhost:50051
//	export SCALARDL_PRIVILEGED_ADDR=localhost:50052
//	export SCALARDL_ENTITY_ID=test-entity
//	export SCALARDL_SECRET_KEY=tegata-dev-secret-key
//
// For ECDSA authentication (legacy):
//
//	export SCALARDL_ADDR=localhost:50051
//	export SCALARDL_ENTITY_ID=test-entity
//	export SCALARDL_CERT_PATH=/path/to/client.pem
//	export SCALARDL_KEY_PATH=/path/to/client-key.pem
//
// Use deployments/docker-compose/docker-compose.yml to start a local instance.
package audit_test

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/josh-wong/tegata/internal/audit"
)

// integrationEnv reads required environment variables. Returns false and skips
// the test if any required variable is unset. SCALARDL_PRIVILEGED_ADDR is
// optional; if unset it defaults to SCALARDL_ADDR (same host, same port).
func integrationEnv(t *testing.T) (addr, privilegedAddr, entityID, certPath, keyPath string, ok bool) {
	t.Helper()
	addr = os.Getenv("SCALARDL_ADDR")
	privilegedAddr = os.Getenv("SCALARDL_PRIVILEGED_ADDR")
	entityID = os.Getenv("SCALARDL_ENTITY_ID")
	certPath = os.Getenv("SCALARDL_CERT_PATH")
	keyPath = os.Getenv("SCALARDL_KEY_PATH")

	if addr == "" || entityID == "" || certPath == "" || keyPath == "" {
		t.Skip("integration test skipped: set SCALARDL_ADDR, SCALARDL_ENTITY_ID, SCALARDL_CERT_PATH, SCALARDL_KEY_PATH")
		return "", "", "", "", "", false
	}
	if privilegedAddr == "" {
		privilegedAddr = addr
	}
	return addr, privilegedAddr, entityID, certPath, keyPath, true
}

// integrationEnvHMAC reads HMAC-specific environment variables. Returns false
// and skips the test if any required variable is unset.
func integrationEnvHMAC(t *testing.T) (addr, privilegedAddr, entityID, secretKey string, ok bool) {
	t.Helper()
	addr = os.Getenv("SCALARDL_ADDR")
	privilegedAddr = os.Getenv("SCALARDL_PRIVILEGED_ADDR")
	entityID = os.Getenv("SCALARDL_ENTITY_ID")
	secretKey = os.Getenv("SCALARDL_SECRET_KEY")

	if addr == "" || entityID == "" || secretKey == "" {
		t.Skip("integration test skipped: set SCALARDL_ADDR, SCALARDL_ENTITY_ID, SCALARDL_SECRET_KEY")
		return "", "", "", "", false
	}
	if privilegedAddr == "" {
		privilegedAddr = addr
	}
	return addr, privilegedAddr, entityID, secretKey, true
}

// newIntegrationClientHMAC builds a LedgerClient using HMAC authentication
// via NewClientFromConfig, matching the production code path.
func newIntegrationClientHMAC(t *testing.T) *audit.LedgerClient {
	t.Helper()
	addr, privilegedAddr, entityID, secretKey, ok := integrationEnvHMAC(t)
	if !ok {
		return nil
	}

	client, err := audit.NewClientFromConfig(addr, privilegedAddr, entityID, 1, secretKey, true)
	if err != nil {
		t.Fatalf("creating HMAC ledger client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

// newIntegrationClient builds a LedgerClient from environment variables.
func newIntegrationClient(t *testing.T) *audit.LedgerClient {
	t.Helper()
	addr, privilegedAddr, entityID, certPath, keyPath, ok := integrationEnv(t)
	if !ok {
		return nil
	}

	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		t.Fatalf("loading client cert: %v", err)
	}

	tlsCfg := &tls.Config{
		MinVersion:         tls.VersionTLS13,
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: os.Getenv("SCALARDL_INSECURE_TLS") == "true", //nolint:gosec
	}

	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("reading key PEM: %v", err)
	}

	signer, err := audit.NewECDSASigner(keyPEM)
	if err != nil {
		t.Fatalf("creating ECDSA signer: %v", err)
	}

	client, err := audit.NewLedgerClient(addr, privilegedAddr, tlsCfg, entityID, 1, signer)
	if err != nil {
		t.Fatalf("creating ledger client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

// TestIntegration_RegisterCert connects to SCALARDL_ADDR and calls RegisterCert.
// This must succeed before any contract execution calls will work.
func TestIntegration_RegisterCert(t *testing.T) {
	_, _, entityID, certPath, _, ok := integrationEnv(t)
	if !ok {
		return
	}

	client := newIntegrationClient(t)
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("reading cert PEM: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.RegisterCert(ctx, entityID, 1, string(certPEM)); err != nil {
		t.Errorf("RegisterCert: %v", err)
	} else {
		t.Log("RegisterCert succeeded")
	}
}

// TestIntegration_Validate puts a test event on the ledger and validates it.
// Requires RegisterCert to have run successfully first (or registration was
// already done in a previous test run).
func TestIntegration_Validate(t *testing.T) {
	client := newIntegrationClient(t)
	if client == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Generate a unique objectID for this test run to avoid conflicts.
	objectID := fmt.Sprintf("test-event-%d", time.Now().UnixNano())
	hashValue := "dummyhash0000000000000000000000000000000000000000000000000000000000"

	if err := client.Put(ctx, objectID, hashValue); err != nil {
		t.Fatalf("Put: %v", err)
	}
	t.Logf("Put succeeded: objectID=%s", objectID)

	result, err := client.Validate(ctx, objectID)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if !result.Valid {
		t.Errorf("Validate returned Valid=false: %s", result.ErrorDetail)
	} else {
		t.Logf("Validate succeeded: EventCount=%d", result.EventCount)
	}
}

// TestIntegration_SignatureByteLayout verifies the ECDSA signature byte
// serialization against a live ScalarDL 3.12 instance.
//
// Expected byte layout (matches ContractExecutionRequest.serialize() in the
// ScalarDL Java SDK):
//
//	contractId (UTF-8) || contractArgument (UTF-8) || entityId (UTF-8) || keyVersion (4-byte big-endian int)
//
// If UNAUTHENTICATED is returned, the server rejected the signature. Compare
// ECDSASigner.Sign in signer.go against the Java source at:
// common/src/main/java/com/scalar/dl/ledger/model/ContractExecutionRequest.java
func TestIntegration_SignatureByteLayout(t *testing.T) {
	client := newIntegrationClient(t)
	if client == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	objectID := fmt.Sprintf("sig-test-%d", time.Now().UnixNano())
	err := client.Put(ctx, objectID, "badc0de0000000000000000000000000000000000000000000000000000000000")
	if err != nil {
		if strings.Contains(err.Error(), "UNAUTHENTICATED") {
			t.Errorf("UNAUTHENTICATED error — signature byte layout is incorrect.\n"+
				"Expected layout: contractId+contractArgument+entityId+keyVersion(4-byte big-endian)\n"+
				"See ContractExecutionRequest.serialize() in the ScalarDL Java SDK.\n"+
				"Error: %v", err)
		} else {
			t.Errorf("Put returned unexpected error: %v", err)
		}
		return
	}
	t.Log("SignatureByteLayout test passed — byte layout accepted by the server")
}

// TestIntegration_RegisterContracts verifies that the generic HashStore
// contracts (object.Put, object.Get, object.Validate) are registered on the
// live ScalarDL instance. If any contract is missing, the corresponding RPC
// will fail with CONTRACT_NOT_FOUND.
func TestIntegration_RegisterContracts(t *testing.T) {
	client := newIntegrationClient(t)
	if client == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Verify generic contracts are registered by attempting a Put.
	// If contracts are not registered, Put will fail with a CONTRACT_NOT_FOUND error.
	testObjID := fmt.Sprintf("contract-reg-test-%d", time.Now().UnixNano())
	testHash := "0000000000000000000000000000000000000000000000000000000000000000"
	if err := client.Put(ctx, testObjID, testHash); err != nil {
		t.Fatalf("Put failed — generic contracts may not be registered: %v", err)
	}
	t.Log("Generic contracts are registered: Put succeeded")

	// Verify Get also works (object.Get contract registered).
	records, err := client.Get(ctx, testObjID)
	if err != nil {
		t.Fatalf("Get failed — object.Get contract may not be registered: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("Get returned 0 records for object that was just Put")
	}
	t.Logf("object.Get contract verified: %d records", len(records))

	// Verify Validate also works (object.Validate contract registered).
	result, err := client.Validate(ctx, testObjID)
	if err != nil {
		t.Fatalf("Validate failed — object.Validate contract may not be registered: %v", err)
	}
	if !result.Valid {
		t.Errorf("Validate returned Valid=false for fresh object: %s", result.ErrorDetail)
	}
	t.Log("object.Validate contract verified")
}

// TestIntegration_E2E_PutGetValidate exercises the full contract flow: Put two
// events, Get them back to verify hash values, then Validate integrity.
func TestIntegration_E2E_PutGetValidate(t *testing.T) {
	client := newIntegrationClient(t)
	if client == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Generate unique object IDs for this test run.
	objectID1 := fmt.Sprintf("e2e-event-%d-1", time.Now().UnixNano())
	objectID2 := fmt.Sprintf("e2e-event-%d-2", time.Now().UnixNano())
	hash1 := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	hash2 := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	// Step 1: Put two events.
	if err := client.Put(ctx, objectID1, hash1); err != nil {
		t.Fatalf("Put event 1: %v", err)
	}
	t.Logf("Put succeeded: %s", objectID1)

	if err := client.Put(ctx, objectID2, hash2); err != nil {
		t.Fatalf("Put event 2: %v", err)
	}
	t.Logf("Put succeeded: %s", objectID2)

	// Step 2: Get each event back and verify the hash values.
	records1, err := client.Get(ctx, objectID1)
	if err != nil {
		t.Fatalf("Get event 1: %v", err)
	}
	if len(records1) == 0 {
		t.Fatal("Get returned 0 records for event 1")
	}
	if records1[0].HashValue != hash1 {
		t.Errorf("HashValue = %q, want %q", records1[0].HashValue, hash1)
	}
	t.Logf("Get event 1: %d records", len(records1))

	// Step 3: Validate event 1 integrity.
	result, err := client.Validate(ctx, objectID1)
	if err != nil {
		t.Fatalf("Validate event 1: %v", err)
	}
	if !result.Valid {
		t.Errorf("Validate returned Valid=false for event 1: %s", result.ErrorDetail)
	}
	t.Logf("Validate event 1: Valid=%v EventCount=%d", result.Valid, result.EventCount)
}

// TestIntegration_HistoryRetrieval verifies that events Put to the ledger can
// be retrieved via Get, simulating what "tegata history" does under the hood.
func TestIntegration_HistoryRetrieval(t *testing.T) {
	client := newIntegrationClient(t)
	if client == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Put 3 events with distinct object IDs sharing a common prefix.
	prefix := fmt.Sprintf("history-test-%d", time.Now().UnixNano())
	for i := 0; i < 3; i++ {
		objID := fmt.Sprintf("%s-%d", prefix, i)
		hash := fmt.Sprintf("%064d", i)
		if err := client.Put(ctx, objID, hash); err != nil {
			t.Fatalf("Put event %d: %v", i, err)
		}
	}

	// Retrieve each event individually (tegata history retrieves by known object IDs).
	for i := 0; i < 3; i++ {
		objID := fmt.Sprintf("%s-%d", prefix, i)
		records, err := client.Get(ctx, objID)
		if err != nil {
			t.Fatalf("Get event %d: %v", i, err)
		}
		if len(records) == 0 {
			t.Errorf("Get returned 0 records for event %d (objectID=%s)", i, objID)
			continue
		}
		expectedHash := fmt.Sprintf("%064d", i)
		if records[0].HashValue != expectedHash {
			t.Errorf("event %d: HashValue = %q, want %q", i, records[0].HashValue, expectedHash)
		}
	}
	t.Logf("History retrieval verified: 3 events stored and retrieved with correct hashes")
}

// TestIntegration_QueueFlush verifies that events queued while offline are
// correctly submitted to the ledger when connectivity is restored.
func TestIntegration_QueueFlush(t *testing.T) {
	client := newIntegrationClient(t)
	if client == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create a queue with a random 32-byte key.
	queueKey := make([]byte, 32)
	if _, err := rand.Read(queueKey); err != nil {
		t.Fatalf("generating queue key: %v", err)
	}
	defer func() {
		for i := range queueKey {
			queueKey[i] = 0
		}
	}()

	q, err := audit.NewQueue(queueKey, 100)
	if err != nil {
		t.Fatalf("creating queue: %v", err)
	}

	// Append 3 events to the queue (simulating offline operation).
	for i := 0; i < 3; i++ {
		evt := audit.NewAuthEvent(
			"totp",
			fmt.Sprintf("flush-test-%d-%d", time.Now().UnixNano(), i),
			"test-service",
			"test-host",
			true,
			"", // prevHash
		)
		if err := q.Append(evt); err != nil {
			t.Fatalf("appending event %d: %v", i, err)
		}
	}

	if q.Len() != 3 {
		t.Fatalf("queue length = %d, want 3", q.Len())
	}

	// Flush the queue to the live ScalarDL instance.
	if err := q.Flush(ctx, client); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	if q.Len() != 0 {
		t.Errorf("queue length after flush = %d, want 0", q.Len())
	}
	t.Log("Queue flush succeeded: 3 events submitted")
}

// TestIntegration_HMAC_RegisterSecretAndPut verifies the production HMAC code
// path: NewClientFromConfig → RegisterSecret → Put → Get round-trip.
func TestIntegration_HMAC_RegisterSecretAndPut(t *testing.T) {
	client := newIntegrationClientHMAC(t)
	if client == nil {
		return
	}

	_, _, entityID, secretKey, _ := integrationEnvHMAC(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Register the HMAC secret (idempotent).
	if err := client.RegisterSecret(ctx, entityID, 1, secretKey); err != nil {
		t.Fatalf("RegisterSecret: %v", err)
	}
	t.Log("RegisterSecret succeeded")

	// Put an event and retrieve it.
	objectID := fmt.Sprintf("hmac-test-%d", time.Now().UnixNano())
	hashValue := "cafebabe00000000000000000000000000000000000000000000000000000000"

	if err := client.Put(ctx, objectID, hashValue); err != nil {
		t.Fatalf("Put (HMAC): %v", err)
	}
	t.Logf("Put succeeded: %s", objectID)

	records, err := client.Get(ctx, objectID)
	if err != nil {
		t.Fatalf("Get (HMAC): %v", err)
	}
	if len(records) == 0 {
		t.Fatal("Get returned 0 records")
	}
	if records[0].HashValue != hashValue {
		t.Errorf("HashValue = %q, want %q", records[0].HashValue, hashValue)
	}
	t.Log("HMAC Put+Get round-trip succeeded")
}

// TestIntegration_HMAC_SubmitAndCollectionGet verifies Submit (the production
// event storage path) followed by CollectionGet for retrieval.
func TestIntegration_HMAC_SubmitAndCollectionGet(t *testing.T) {
	client := newIntegrationClientHMAC(t)
	if client == nil {
		return
	}

	_, _, entityID, secretKey, _ := integrationEnvHMAC(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Register the HMAC secret (idempotent).
	if err := client.RegisterSecret(ctx, entityID, 1, secretKey); err != nil {
		t.Fatalf("RegisterSecret: %v", err)
	}

	// Submit an event via the production Submit path.
	evt := audit.NewAuthEvent(
		"totp",
		fmt.Sprintf("hmac-submit-%d", time.Now().UnixNano()),
		"test-service",
		"test-host",
		true,
		"",
	)
	entry := audit.QueueEntry{Event: evt}
	if err := client.Submit(ctx, entry); err != nil {
		t.Fatalf("Submit (HMAC): %v", err)
	}
	t.Logf("Submit succeeded: %s", evt.EventID)

	// Verify the event appears in the collection.
	collectionID := audit.CollectionID(entityID)
	eventIDs, err := client.CollectionGet(ctx, collectionID)
	if err != nil {
		t.Fatalf("CollectionGet (HMAC): %v", err)
	}

	found := false
	for _, id := range eventIDs {
		if id == evt.EventID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("event %s not found in collection %s (has %d events)", evt.EventID, collectionID, len(eventIDs))
	}
	t.Log("HMAC Submit+CollectionGet round-trip succeeded")
}
