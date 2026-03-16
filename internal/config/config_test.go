package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.ClipboardTimeout != 45*time.Second {
		t.Errorf("ClipboardTimeout = %v, want 45s", cfg.ClipboardTimeout)
	}
	if cfg.IdleTimeout != 300*time.Second {
		t.Errorf("IdleTimeout = %v, want 300s", cfg.IdleTimeout)
	}
}

func TestLoadMissingFile(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load returned error for missing file: %v", err)
	}
	want := DefaultConfig()
	if cfg != want {
		t.Errorf("Load(missing) = %+v, want %+v", cfg, want)
	}
}

func TestLoadValidTOML(t *testing.T) {
	dir := t.TempDir()
	content := `[clipboard]
timeout = 30

[vault]
idle_timeout = 600
`
	if err := os.WriteFile(filepath.Join(dir, "tegata.toml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.ClipboardTimeout != 30*time.Second {
		t.Errorf("ClipboardTimeout = %v, want 30s", cfg.ClipboardTimeout)
	}
	if cfg.IdleTimeout != 600*time.Second {
		t.Errorf("IdleTimeout = %v, want 600s", cfg.IdleTimeout)
	}
}

func TestLoadPartialConfig(t *testing.T) {
	dir := t.TempDir()
	content := `[clipboard]
timeout = 10
`
	if err := os.WriteFile(filepath.Join(dir, "tegata.toml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.ClipboardTimeout != 10*time.Second {
		t.Errorf("ClipboardTimeout = %v, want 10s", cfg.ClipboardTimeout)
	}
	if cfg.IdleTimeout != 300*time.Second {
		t.Errorf("IdleTimeout = %v, want 300s (default)", cfg.IdleTimeout)
	}
}

func TestLoadMalformedTOML(t *testing.T) {
	dir := t.TempDir()
	content := `[clipboard
broken toml
`
	if err := os.WriteFile(filepath.Join(dir, "tegata.toml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(dir)
	if err == nil {
		t.Fatal("Load should return error for malformed TOML")
	}
}

func TestWriteDefaults(t *testing.T) {
	dir := t.TempDir()
	if err := WriteDefaults(dir); err != nil {
		t.Fatalf("WriteDefaults returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "tegata.toml"))
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}

	content := string(data)
	// All settings should be commented out
	if !strings.Contains(content, "# timeout = 45") {
		t.Error("WriteDefaults should contain commented timeout = 45")
	}
	if !strings.Contains(content, "# idle_timeout = 300") {
		t.Error("WriteDefaults should contain commented idle_timeout = 300")
	}
	if !strings.Contains(content, "[clipboard]") {
		t.Error("WriteDefaults should contain [clipboard] section")
	}
	if !strings.Contains(content, "[vault]") {
		t.Error("WriteDefaults should contain [vault] section")
	}
}

func TestFormatEffectiveDefaults(t *testing.T) {
	cfg := DefaultConfig()
	out := FormatEffective(cfg, false)
	if !strings.Contains(out, "clipboard.timeout = 45") {
		t.Errorf("FormatEffective missing clipboard.timeout, got: %s", out)
	}
	if !strings.Contains(out, "(default)") {
		t.Errorf("FormatEffective should show (default) when no file, got: %s", out)
	}
	if !strings.Contains(out, "vault.idle_timeout = 300") {
		t.Errorf("FormatEffective missing vault.idle_timeout, got: %s", out)
	}
}

func TestFormatEffectiveFromFile(t *testing.T) {
	cfg := Config{
		ClipboardTimeout: 30 * time.Second,
		IdleTimeout:      600 * time.Second,
	}
	out := FormatEffective(cfg, true)
	if strings.Contains(out, "(default)") {
		t.Errorf("FormatEffective should not show (default) when file exists, got: %s", out)
	}
	if !strings.Contains(out, "clipboard.timeout = 30") {
		t.Errorf("FormatEffective missing clipboard.timeout = 30, got: %s", out)
	}
}
