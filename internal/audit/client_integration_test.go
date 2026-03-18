//go:build integration

// Integration tests for the ScalarDL Ledger gRPC client. These tests require
// a live ScalarDL Ledger 3.12 instance and a registered client certificate.
//
// To run:
//
//	export SCALARDL_ADDR=localhost:50051
//	export SCALARDL_ENTITY_ID=test-entity
//	export SCALARDL_CERT_PATH=/path/to/client.pem
//	export SCALARDL_KEY_PATH=/path/to/client-key.pem
//	go test -tags integration ./internal/audit/... -v -timeout 60s
//
// Use deployments/docker-compose/docker-compose.yml to start a local instance.
package audit_test

import (
	"context"
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

// TestIntegration_SignatureByteLayout explicitly tests the ECDSA signature
// byte serialization against a live ScalarDL 3.12 instance. The ECDSASigner
// stub may produce UNAUTHENTICATED errors if the byte layout is incorrect.
//
// If UNAUTHENTICATED is returned:
//  1. Inspect the Java ClientService.RequestBuilder source to determine the
//     correct concatenation order.
//  2. Update ECDSASigner.Sign in signer.go with the correct byte layout.
//  3. Re-run this test to confirm.
//
// Current stub byte layout:
// nonce + entityID + strconv.Itoa(keyVersion) + contractID + contractArgument
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
				"Current layout: nonce+entityID+keyVersion+contractID+contractArgument\n"+
				"Inspect Java ClientService.RequestBuilder for the correct layout.\n"+
				"Error: %v", err)
		} else {
			t.Errorf("Put returned unexpected error: %v", err)
		}
		return
	}
	t.Log("SignatureByteLayout test passed — current byte layout is accepted by the server")
}
