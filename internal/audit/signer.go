// Package audit — Signer interface for ECDSA SHA-256 signature computation.
//
// WARNING: The exact byte serialization for ContractExecutionRequest.Signature
// is LOW confidence (undocumented for Go clients). ECDSASigner.Sign is a STUB
// that must be validated via integration testing against a live ScalarDL 3.12
// instance. See docs/scalardl-setup.md "Known limitations" section.
package audit

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strconv"
)

// Signer computes an ECDSA-SHA256 signature over the fields of a
// ContractExecutionRequest. The exact byte layout must match what the
// ScalarDL Java ClientService.RequestBuilder produces.
type Signer interface {
	// Sign returns the ECDSA signature bytes for a contract execution request.
	// Parameters correspond directly to ContractExecutionRequest fields.
	Sign(contractID, contractArgument, nonce, entityID string, keyVersion uint32) ([]byte, error)
}

// ECDSASigner implements Signer using SHA256withECDSA.
//
// Current byte serialization (STUB — must be validated against live ScalarDL 3.12):
// UTF-8 concat of: nonce + entityID + strconv.Itoa(keyVersion) + contractID + contractArgument
//
// If UNAUTHENTICATED errors occur from the server, inspect the Java
// ClientService.RequestBuilder source for the correct byte layout and
// run the integration test to diagnose:
//
//	go test -tags integration ./internal/audit/... -run TestIntegration_SignatureByteLayout -v
type ECDSASigner struct {
	privateKey *ecdsa.PrivateKey
}

// NewECDSASigner parses a PEM-encoded EC private key and returns a signer.
// Supports PKCS#8 ("PRIVATE KEY") and SEC1 ("EC PRIVATE KEY") PEM blocks.
func NewECDSASigner(privateKeyPEM []byte) (*ECDSASigner, error) {
	block, _ := pem.Decode(privateKeyPEM)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in private key")
	}

	var key *ecdsa.PrivateKey
	switch block.Type {
	case "PRIVATE KEY":
		// PKCS#8 format.
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing PKCS#8 private key: %w", err)
		}
		var ok bool
		key, ok = parsed.(*ecdsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("PKCS#8 key is not an ECDSA key")
		}
	case "EC PRIVATE KEY":
		// SEC1 format.
		var err error
		key, err = x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing EC private key: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported PEM block type: %s", block.Type)
	}

	return &ECDSASigner{privateKey: key}, nil
}

// Sign computes an ECDSA-SHA256 signature for a ContractExecutionRequest.
//
// STUB: The byte concatenation order below is a best-effort approximation of
// the Java ClientService.RequestBuilder serialization. Validate against a live
// ScalarDL 3.12 instance before deploying. See docs/scalardl-setup.md.
func (s *ECDSASigner) Sign(contractID, contractArgument, nonce, entityID string, keyVersion uint32) ([]byte, error) {
	// Construct the message bytes: nonce + entityID + keyVersion + contractID + contractArgument.
	// This order is a stub — must be validated against ScalarDL 3.12 Java source.
	msg := nonce + entityID + strconv.Itoa(int(keyVersion)) + contractID + contractArgument
	digest := sha256.Sum256([]byte(msg))

	sig, err := ecdsa.SignASN1(rand.Reader, s.privateKey, digest[:])
	if err != nil {
		return nil, fmt.Errorf("signing contract execution request: %w", err)
	}
	return sig, nil
}

// NoOpSigner always returns a nil signature. Use for initial connectivity
// testing only — a live ScalarDL instance will reject nil signatures if
// authentication is enabled.
type NoOpSigner struct{}

// Sign returns nil, nil — the server may accept or reject depending on auth config.
func (n *NoOpSigner) Sign(contractID, contractArgument, nonce, entityID string, keyVersion uint32) ([]byte, error) {
	return nil, nil
}
