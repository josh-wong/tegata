package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestHumanizeError tests error translation for common filesystem errors.
func TestHumanizeError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		contains string // substring that should appear in humanized message
	}{
		{
			name:     "nil error",
			err:      nil,
			contains: "Unknown error",
		},
		{
			name:     "file not found (direct)",
			err:      os.ErrNotExist,
			contains: "Vault file not found",
		},
		{
			name:     "file not found (wrapped)",
			err:      fmt.Errorf("reading vault: %w", os.ErrNotExist),
			contains: "Vault file not found",
		},
		{
			name:     "file not found (text pattern)",
			err:      errors.New("no such file or directory"),
			contains: "Vault file not found",
		},
		{
			name:     "permission denied (direct)",
			err:      os.ErrPermission,
			contains: "Permission denied",
		},
		{
			name:     "permission denied (text pattern)",
			err:      errors.New("permission denied"),
			contains: "Permission denied",
		},
		{
			name:     "read-only filesystem",
			err:      errors.New("read-only file system"),
			contains: "read-only",
		},
		{
			name:     "invalid header (corrupt vault)",
			err:      errors.New("invalid header"),
			contains: "corrupt",
		},
		{
			name:     "corrupted vault file",
			err:      errors.New("vault is corrupted"),
			contains: "corrupt",
		},
		{
			name:     "unknown error (fallback)",
			err:      errors.New("some random error"),
			contains: "some random error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := humanizeError(tt.err)
			if !contains(got, tt.contains) {
				t.Errorf("humanizeError() = %q, want substring %q", got, tt.contains)
			}
		})
	}
}

// contains is a simple helper for substring matching.
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
