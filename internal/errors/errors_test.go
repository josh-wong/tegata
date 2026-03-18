package errors_test

import (
	"fmt"
	"testing"

	"github.com/josh-wong/tegata/internal/errors"
)

func TestExitCode_SentinelErrors(t *testing.T) {
	sentinels := []error{
		errors.ErrAuthFailed,
		errors.ErrVaultCorrupt,
		errors.ErrDecryptFailed,
		errors.ErrNotFound,
		errors.ErrVaultLocked,
		errors.ErrInvalidInput,
		errors.ErrNetworkFailed,
		errors.ErrIntegrityViolation,
	}

	codes := make(map[int]error)
	for _, err := range sentinels {
		code := errors.ExitCode(err)
		if code == 0 {
			t.Errorf("ExitCode(%v) = 0, want non-zero", err)
		}
		if prev, ok := codes[code]; ok {
			t.Errorf("ExitCode(%v) = %d collides with ExitCode(%v)", err, code, prev)
		}
		codes[code] = err
	}
}

func TestExitCode_NilError(t *testing.T) {
	if code := errors.ExitCode(nil); code != 0 {
		t.Errorf("ExitCode(nil) = %d, want 0", code)
	}
}

func TestExitCode_WrappedError(t *testing.T) {
	wrapped := fmt.Errorf("context: %w", errors.ErrAuthFailed)
	direct := errors.ExitCode(errors.ErrAuthFailed)
	got := errors.ExitCode(wrapped)

	if got != direct {
		t.Errorf("ExitCode(wrapped ErrAuthFailed) = %d, want %d", got, direct)
	}
}

func TestExitCode_UnknownError(t *testing.T) {
	unknown := fmt.Errorf("some random error")
	if code := errors.ExitCode(unknown); code != 1 {
		t.Errorf("ExitCode(unknown) = %d, want 1", code)
	}
}

func TestExitCode_NetworkFailed(t *testing.T) {
	if code := errors.ExitCode(errors.ErrNetworkFailed); code != 8 {
		t.Errorf("ExitCode(ErrNetworkFailed) = %d, want 8", code)
	}
}

func TestExitCode_IntegrityViolation(t *testing.T) {
	if code := errors.ExitCode(errors.ErrIntegrityViolation); code != 9 {
		t.Errorf("ExitCode(ErrIntegrityViolation) = %d, want 9", code)
	}
}

func TestUserMessage_Format(t *testing.T) {
	got := errors.UserMessage("vault corrupted", "Run tegata init")
	want := "Error: vault corrupted. Run tegata init."
	if got != want {
		t.Errorf("UserMessage = %q, want %q", got, want)
	}
}
