package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAuditConfig_Defaults(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Audit.Enabled {
		t.Error("Audit.Enabled should default to false when [audit] section is absent")
	}
	if cfg.Audit.Server != "" {
		t.Errorf("Audit.Server should default to empty, got %q", cfg.Audit.Server)
	}
	if cfg.Audit.QueueMaxEvents != 10000 {
		t.Errorf("Audit.QueueMaxEvents should default to 10000, got %d", cfg.Audit.QueueMaxEvents)
	}
}

func TestAuditConfig_ParseTOML(t *testing.T) {
	dir := t.TempDir()
	content := `[audit]
enabled = true
server = "localhost:50051"
entity_id = "test"
key_version = 1
queue_max_events = 500
`
	if err := os.WriteFile(filepath.Join(dir, "tegata.toml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !cfg.Audit.Enabled {
		t.Error("Audit.Enabled should be true")
	}
	if cfg.Audit.Server != "localhost:50051" {
		t.Errorf("Audit.Server = %q, want %q", cfg.Audit.Server, "localhost:50051")
	}
	if cfg.Audit.EntityID != "test" {
		t.Errorf("Audit.EntityID = %q, want %q", cfg.Audit.EntityID, "test")
	}
	if cfg.Audit.KeyVersion != 1 {
		t.Errorf("Audit.KeyVersion = %d, want 1", cfg.Audit.KeyVersion)
	}
	if cfg.Audit.QueueMaxEvents != 500 {
		t.Errorf("Audit.QueueMaxEvents = %d, want 500", cfg.Audit.QueueMaxEvents)
	}
}

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

func TestAutoStart_ExplicitTrue(t *testing.T) {
	dir := t.TempDir()
	content := "[audit]\nauto_start = true\n"
	if err := os.WriteFile(filepath.Join(dir, "tegata.toml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !cfg.Audit.AutoStart {
		t.Error("AutoStart should be true when auto_start = true in TOML")
	}
}

func TestAutoStart_ExplicitFalse(t *testing.T) {
	dir := t.TempDir()
	content := "[audit]\nauto_start = false\n"
	if err := os.WriteFile(filepath.Join(dir, "tegata.toml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Audit.AutoStart {
		t.Error("AutoStart should be false when auto_start = false in TOML")
	}
}

func TestAutoStart_DefaultWithDockerComposePath(t *testing.T) {
	dir := t.TempDir()
	content := "[audit]\ndocker_compose_path = \"/some/path/docker-compose.yml\"\n"
	if err := os.WriteFile(filepath.Join(dir, "tegata.toml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	// D-06: when docker_compose_path is set but auto_start is absent, default to true
	if !cfg.Audit.AutoStart {
		t.Error("AutoStart should default to true when docker_compose_path is set and auto_start is absent")
	}
}

func TestAutoStart_DefaultWithoutDockerComposePath(t *testing.T) {
	dir := t.TempDir()
	content := "[audit]\nenabled = false\n"
	if err := os.WriteFile(filepath.Join(dir, "tegata.toml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	// D-06: when neither docker_compose_path nor auto_start is set, default to false
	if cfg.Audit.AutoStart {
		t.Error("AutoStart should default to false when docker_compose_path is not set")
	}
}
