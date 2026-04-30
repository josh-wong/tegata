package main

import (
	"bytes"
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

// TestPrintAuditNotEnabledHint verifies that the hint text written to the
// io.Writer contains the expected actionable substrings.
func TestPrintAuditNotEnabledHint(t *testing.T) {
	expectedSubstrings := []string{
		"Audit logging is not enabled",
		"tegata ledger start",
		"tegata ledger setup",
		"tegata.toml",
	}

	var buf bytes.Buffer
	printAuditNotEnabledHint(&buf)
	got := buf.String()

	for _, want := range expectedSubstrings {
		if !strings.Contains(got, want) {
			t.Errorf("printAuditNotEnabledHint output missing %q\ngot:\n%s", want, got)
		}
	}
}

// contains is a simple helper for substring matching.
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// TestTruncateVaultPath tests smart path truncation for display.
func TestTruncateVaultPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		maxWidth int
		check    func(string, int, int) bool // custom validation: (result, maxWidth, pathLen) -> isValid
	}{
		{
			name:     "short path fits",
			path:     "vault.tegata",
			maxWidth: 50,
			check: func(got string, maxWidth, pathLen int) bool {
				return got == "vault.tegata"
			},
		},
		{
			name:     "path exactly fits",
			path:     "12345",
			maxWidth: 5,
			check: func(got string, maxWidth, pathLen int) bool {
				return got == "12345"
			},
		},
		{
			name:     "long path truncated with ellipsis",
			path:     "/Volumes/ExternalDrive/path/to/my-vault.tegata",
			maxWidth: 30,
			check: func(got string, maxWidth, pathLen int) bool {
				// Should contain ellipsis and not exceed maxWidth
				return strings.Contains(got, "...") && len(got) <= maxWidth
			},
		},
		{
			name:     "narrow width uses minimal fallback",
			path:     "vault.tegata",
			maxWidth: 9,
			check: func(got string, maxWidth, pathLen int) bool {
				// Very narrow width returns "vault" fallback
				return got == "vault"
			},
		},
		{
			name:     "result respects maxWidth constraint",
			path:     "/some/very/long/path/that/should/be/truncated",
			maxWidth: 20,
			check: func(got string, maxWidth, pathLen int) bool {
				return len(got) <= maxWidth
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateVaultPath(tt.path, tt.maxWidth)
			if !tt.check(got, tt.maxWidth, len(tt.path)) {
				t.Errorf("truncateVaultPath(%q, %d) = %q, validation failed", tt.path, tt.maxWidth, got)
			}
		})
	}
}
