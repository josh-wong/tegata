// Package errors defines sentinel errors and exit code mappings for the Tegata
// CLI. All error handling throughout the application uses these sentinels with
// fmt.Errorf("%w", err) wrapping for context propagation.
//
// This package re-exports standard library error functions so callers can use
// errors.Is, errors.As, etc. without importing both packages.
package errors

import (
	stderrors "errors"
	"fmt"
)

// Sentinel errors used throughout Tegata. Each maps to a unique exit code via
// the ExitCode function.
var (
	// ErrAuthFailed indicates passphrase or recovery key authentication failed.
	ErrAuthFailed = stderrors.New("authentication failed")

	// ErrVaultCorrupt indicates the vault file is corrupted or has an invalid
	// format (bad magic bytes, version mismatch, nonce inconsistency).
	ErrVaultCorrupt = stderrors.New("vault file is corrupted")

	// ErrDecryptFailed indicates a decryption operation failed (GCM tag
	// mismatch from wrong key or tampered ciphertext).
	ErrDecryptFailed = stderrors.New("decryption failed")

	// ErrNotFound indicates a requested credential was not found in the vault.
	ErrNotFound = stderrors.New("credential not found")

	// ErrVaultLocked indicates the vault is locked and must be unlocked before
	// the requested operation.
	ErrVaultLocked = stderrors.New("vault is locked")

	// ErrInvalidInput indicates the user provided invalid input (malformed
	// label, bad URI format, etc.).
	ErrInvalidInput = stderrors.New("invalid input")

	// ErrNetworkFailed indicates a network operation failed when communicating
	// with the ScalarDL Ledger (gRPC connection error, timeout, etc.).
	ErrNetworkFailed = stderrors.New("network operation failed")

	// ErrIntegrityViolation indicates a hash-chain integrity check failed in
	// the offline queue or a ScalarDL Ledger audit verification mismatch.
	ErrIntegrityViolation = stderrors.New("integrity violation detected")
)

// ExitCode maps an error to a CLI process exit code. Nil errors return 0.
// Unknown errors return 1. Each sentinel error maps to a unique non-zero code.
func ExitCode(err error) int {
	switch {
	case err == nil:
		return 0
	case stderrors.Is(err, ErrAuthFailed):
		return 2
	case stderrors.Is(err, ErrVaultCorrupt):
		return 3
	case stderrors.Is(err, ErrDecryptFailed):
		return 4
	case stderrors.Is(err, ErrNotFound):
		return 5
	case stderrors.Is(err, ErrVaultLocked):
		return 6
	case stderrors.Is(err, ErrInvalidInput):
		return 7
	case stderrors.Is(err, ErrNetworkFailed):
		return 8
	case stderrors.Is(err, ErrIntegrityViolation):
		return 9
	default:
		return 1
	}
}

// UserMessage formats an error for display to the user. The format follows the
// convention: "Error: <what happened>. <what to do>."
func UserMessage(what, howToFix string) string {
	return fmt.Sprintf("Error: %s. %s.", what, howToFix)
}

// Re-export standard library error functions so callers can use this package
// without also importing "errors".
var (
	Is     = stderrors.Is
	As     = stderrors.As
	New    = stderrors.New
	Unwrap = stderrors.Unwrap
)
