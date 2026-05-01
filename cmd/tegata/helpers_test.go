package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

// TestResolvePathArg tests that resolvePathArg always returns an absolute path
// and correctly appends the vault filename when given a directory.
func TestResolvePathArg(t *testing.T) {
	t.Run("relative file path becomes absolute", func(t *testing.T) {
		got, err := resolvePathArg("tmp/vault.tegata")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !filepath.IsAbs(got) {
			t.Errorf("expected absolute path, got %q", got)
		}
		if !strings.HasSuffix(got, "tmp/vault.tegata") {
			t.Errorf("expected path to end with tmp/vault.tegata, got %q", got)
		}
	})

	t.Run("non-existent relative path becomes absolute", func(t *testing.T) {
		got, err := resolvePathArg("does-not-exist.tegata")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !filepath.IsAbs(got) {
			t.Errorf("expected absolute path, got %q", got)
		}
	})

	t.Run("existing directory gets vault filename appended", func(t *testing.T) {
		dir := t.TempDir()
		got, err := resolvePathArg(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !filepath.IsAbs(got) {
			t.Errorf("expected absolute path, got %q", got)
		}
		if filepath.Base(got) != vaultFilename {
			t.Errorf("expected filename %q, got %q", vaultFilename, filepath.Base(got))
		}
	})

	t.Run("path ending with separator gets vault filename appended", func(t *testing.T) {
		// Non-existent directory path ending with separator.
		// Note: the leading slash makes this an absolute path on Unix only;
		// on Windows filepath.Abs would prepend the CWD drive letter instead.
		got, err := resolvePathArg("/nonexistent/dir" + string(filepath.Separator))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if filepath.Base(got) != vaultFilename {
			t.Errorf("expected filename %q, got %q", vaultFilename, filepath.Base(got))
		}
	})

	t.Run("absolute file path is returned as-is", func(t *testing.T) {
		abs := "/tmp/my-vault.tegata"
		got, err := resolvePathArg(abs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != abs {
			t.Errorf("expected %q, got %q", abs, got)
		}
	})
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
		{
			name:     "multi-byte path truncated by rune count not byte count",
			path:     "/ボリューム/認証/保管庫/vault.tegata",
			maxWidth: 15,
			check: func(got string, maxWidth, pathLen int) bool {
				// Result must contain ellipsis, and rune count must not exceed maxWidth.
				runes := []rune(got)
				return strings.Contains(got, "...") && len(runes) <= maxWidth
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

// TestFormatVaultPathWithBoldFilename tests that the vault path is rendered
// with the filename bold and the directory prefix faint.
func TestFormatVaultPathWithBoldFilename(t *testing.T) {
	t.Run("path with directory contains filename", func(t *testing.T) {
		got := formatVaultPathWithBoldFilename("/path/to/vault.tegata")
		if !strings.Contains(got, "vault.tegata") {
			t.Errorf("expected output to contain filename, got %q", got)
		}
		if !strings.Contains(got, "Vault: ") {
			t.Errorf("expected output to contain 'Vault: ' prefix, got %q", got)
		}
	})

	t.Run("filename-only path skips directory prefix", func(t *testing.T) {
		got := formatVaultPathWithBoldFilename("vault.tegata")
		if !strings.Contains(got, "vault.tegata") {
			t.Errorf("expected output to contain filename, got %q", got)
		}
		if strings.Contains(got, "Vault: ") {
			t.Errorf("expected no 'Vault: ' prefix for bare filename, got %q", got)
		}
	})

	t.Run("output is non-empty for any non-empty path", func(t *testing.T) {
		paths := []string{
			"/tmp/my-vault.tegata",
			"vault.tegata",
			"/Volumes/USB/auth.tegata",
		}
		for _, p := range paths {
			got := formatVaultPathWithBoldFilename(p)
			if got == "" {
				t.Errorf("formatVaultPathWithBoldFilename(%q) returned empty string", p)
			}
		}
	})
}
