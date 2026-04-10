package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/josh-wong/tegata/internal/config"
)

// setupConfigTestDir creates a temp directory with a placeholder vault file and
// sets TEGATA_VAULT so resolveVaultPath can locate it without the --vault flag.
func setupConfigTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	vaultFile := filepath.Join(dir, "vault.tegata")
	if err := os.WriteFile(vaultFile, []byte{}, 0600); err != nil {
		t.Fatalf("creating placeholder vault: %v", err)
	}
	t.Setenv("TEGATA_VAULT", vaultFile)
	return dir
}

// TestConfigSetAutoStart verifies that 'tegata config set audit.auto_start'
// writes the expected value to tegata.toml and that loading it back reflects
// the change.
func TestConfigSetAutoStart(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  bool
	}{
		{"true", true},
		{"false", false},
		{"True", true},
		{"False", false},
	} {
		t.Run(tc.value, func(t *testing.T) {
			dir := setupConfigTestDir(t)

			// Seed an initial config with AutoStart opposite to what we will set.
			initial := config.AuditConfig{Enabled: true, AutoStart: !tc.want}
			if err := config.WriteAuditSection(dir, initial); err != nil {
				t.Fatalf("seeding config: %v", err)
			}

			cmd := newConfigCmd()
			cmd.SetArgs([]string{"set", "audit.auto_start", tc.value})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("config set: %v", err)
			}

			cfg, err := config.Load(dir)
			if err != nil {
				t.Fatalf("loading config: %v", err)
			}
			if cfg.Audit.AutoStart != tc.want {
				t.Errorf("AutoStart = %v, want %v", cfg.Audit.AutoStart, tc.want)
			}
		})
	}
}

// TestConfigSetAutoStart_InvalidValue verifies that an unrecognised value
// returns an error and leaves the config unchanged.
func TestConfigSetAutoStart_InvalidValue(t *testing.T) {
	dir := setupConfigTestDir(t)

	initial := config.AuditConfig{Enabled: true, AutoStart: true}
	if err := config.WriteAuditSection(dir, initial); err != nil {
		t.Fatalf("seeding config: %v", err)
	}

	cmd := newConfigCmd()
	cmd.SetArgs([]string{"set", "audit.auto_start", "Yes"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for invalid value 'Yes', got nil")
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	if !cfg.Audit.AutoStart {
		t.Error("AutoStart was modified despite invalid value")
	}
}

// TestConfigSetAutoStart_UnknownKey verifies that an unrecognised key returns
// an error.
func TestConfigSetAutoStart_UnknownKey(t *testing.T) {
	setupConfigTestDir(t)

	cmd := newConfigCmd()
	cmd.SetArgs([]string{"set", "audit.unknown", "true"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for unknown key, got nil")
	}
}
