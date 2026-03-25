// Package audit — Signer interface for signing ScalarDL contract execution requests.
package audit

import (
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

// Signer computes a signature over the fields of a ContractExecutionRequest.
// The exact byte layout must match what the ScalarDL Java
// ClientService.RequestBuilder produces.
type Signer interface {
	// Sign returns the signature bytes for a contract execution request.
	// Parameters correspond directly to ContractExecutionRequest fields.
	Sign(contractID, contractArgument, nonce, entityID string, keyVersion uint32) ([]byte, error)
}

// ECDSASigner implements Signer using SHA256withECDSA.
//
// Byte serialization matches ContractExecutionRequest.serialize() from
// scalar-labs/scalardl (common/src/main/java/com/scalar/dl/ledger/model/):
//
//	contractId (UTF-8) || contractArgument (UTF-8) || entityId (UTF-8) || keyVersion (4-byte big-endian int)
//
// No length prefixes, no delimiters, no nonce. The nonce field in the request
// is for replay prevention and is not included in the signed bytes.
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
// The byte layout matches ContractExecutionRequest.serialize() in the ScalarDL
// Java SDK: contractId || contractArgument || entityId || keyVersion (4-byte big-endian).
func (s *ECDSASigner) Sign(contractID, contractArgument, nonce, entityID string, keyVersion uint32) ([]byte, error) {
	// Serialize per scalar-labs/scalardl ContractExecutionRequest.serialize():
	// contractId (UTF-8) + contractArgument (UTF-8) + entityId (UTF-8) + keyVersion (4-byte big-endian int).
	// The nonce parameter is transmitted in the request but is not part of the signed payload.
	cid := []byte(contractID)
	arg := []byte(contractArgument)
	eid := []byte(entityID)

	msg := make([]byte, 0, len(cid)+len(arg)+len(eid)+4)
	msg = append(msg, cid...)
	msg = append(msg, arg...)
	msg = append(msg, eid...)
	msg = append(msg, byte(keyVersion>>24), byte(keyVersion>>16), byte(keyVersion>>8), byte(keyVersion))

	digest := sha256.Sum256(msg)
	for i := range msg {
		msg[i] = 0
	}

	sig, err := ecdsa.SignASN1(rand.Reader, s.privateKey, digest[:])
	if err != nil {
		return nil, fmt.Errorf("signing contract execution request: %w", err)
	}
	return sig, nil
}

// HMACSigner implements Signer using HMAC-SHA256. Used with ScalarDL HMAC
// authentication mode instead of digital-signature (ECDSA).
//
// Byte serialization matches ContractExecutionRequest.serialize() from
// scalar-labs/scalardl:
//
//	contractId (UTF-8) || contractArgument (UTF-8) || entityId (UTF-8) || keyVersion (4-byte big-endian int)
type HMACSigner struct {
	secretKey []byte
}

// NewHMACSigner creates a signer from the HMAC secret key string.
func NewHMACSigner(secretKey string) *HMACSigner {
	return &HMACSigner{secretKey: []byte(secretKey)}
}

// Sign computes an HMAC-SHA256 over the serialized contract execution request.
func (s *HMACSigner) Sign(contractID, contractArgument, nonce, entityID string, keyVersion uint32) ([]byte, error) {
	cid := []byte(contractID)
	arg := []byte(contractArgument)
	eid := []byte(entityID)

	msg := make([]byte, 0, len(cid)+len(arg)+len(eid)+4)
	msg = append(msg, cid...)
	msg = append(msg, arg...)
	msg = append(msg, eid...)
	msg = append(msg, byte(keyVersion>>24), byte(keyVersion>>16), byte(keyVersion>>8), byte(keyVersion))

	mac := hmac.New(sha256.New, s.secretKey)
	mac.Write(msg)

	// Zero the message buffer.
	for i := range msg {
		msg[i] = 0
	}

	return mac.Sum(nil), nil
}

// NoOpSigner always returns a nil signature. Use for initial connectivity
// testing only — a live ScalarDL instance will reject nil signatures if
// authentication is enabled.
type NoOpSigner struct{}

// Sign returns nil, nil — the server may accept or reject depending on auth config.
func (n *NoOpSigner) Sign(contractID, contractArgument, nonce, entityID string, keyVersion uint32) ([]byte, error) {
	return nil, nil
}
