package config

import (
	"os"
	"strings"
	"testing"
)

func TestWriteClipboardVaultSections_NewFile(t *testing.T) {
	dir := t.TempDir()
	if err := WriteClipboardVaultSections(dir, 30, 120); err != nil {
		t.Fatalf("WriteClipboardVaultSections: %v", err)
	}
	data, _ := os.ReadFile(dir + "/tegata.toml")
	content := string(data)
	if !strings.Contains(content, "[clipboard]") {
		t.Error("written file does not contain [clipboard]")
	}
	if !strings.Contains(content, "timeout = 30") {
		t.Error("written file does not contain clipboard timeout value")
	}
	if !strings.Contains(content, "[vault]") {
		t.Error("written file does not contain [vault]")
	}
	if !strings.Contains(content, "idle_timeout = 120") {
		t.Error("written file does not contain idle_timeout value")
	}
}

func TestWriteClipboardVaultSections_PreservesAudit(t *testing.T) {
	dir := t.TempDir()
	existing := "[clipboard]\ntimeout = 45\n\n[vault]\nidle_timeout = 300\n\n[audit]\nenabled = true\nserver = \"localhost:50051\"\nentity_id = \"tegata-abc\"\n"
	if err := os.WriteFile(dir+"/tegata.toml", []byte(existing), 0600); err != nil {
		t.Fatalf("writing existing config: %v", err)
	}

	if err := WriteClipboardVaultSections(dir, 60, 600); err != nil {
		t.Fatalf("WriteClipboardVaultSections: %v", err)
	}

	data, _ := os.ReadFile(dir + "/tegata.toml")
	content := string(data)

	if !strings.Contains(content, "[audit]") {
		t.Error("[audit] section was destroyed")
	}
	if !strings.Contains(content, "enabled = true") {
		t.Error("[audit] enabled field was lost")
	}
	if !strings.Contains(content, "localhost:50051") {
		t.Error("[audit] server value was lost")
	}
	if !strings.Contains(content, "tegata-abc") {
		t.Error("[audit] entity_id value was lost")
	}
	if !strings.Contains(content, "timeout = 60") {
		t.Error("new clipboard timeout was not written")
	}
	if !strings.Contains(content, "idle_timeout = 600") {
		t.Error("new idle timeout was not written")
	}
}

func TestWriteClipboardVaultSections_NoAuditSection(t *testing.T) {
	dir := t.TempDir()
	existing := "[clipboard]\ntimeout = 45\n\n[vault]\nidle_timeout = 300\n"
	if err := os.WriteFile(dir+"/tegata.toml", []byte(existing), 0600); err != nil {
		t.Fatalf("writing existing config: %v", err)
	}

	if err := WriteClipboardVaultSections(dir, 30, 60); err != nil {
		t.Fatalf("WriteClipboardVaultSections: %v", err)
	}

	data, _ := os.ReadFile(dir + "/tegata.toml")
	content := string(data)

	if strings.Count(content, "[clipboard]") != 1 {
		t.Errorf("[clipboard] appears %d times, want 1", strings.Count(content, "[clipboard]"))
	}
	if strings.Count(content, "[vault]") != 1 {
		t.Errorf("[vault] appears %d times, want 1", strings.Count(content, "[vault]"))
	}
	if strings.Contains(content, "timeout = 45") {
		t.Error("old clipboard timeout still present")
	}
	if strings.Contains(content, "idle_timeout = 300") {
		t.Error("old idle timeout still present")
	}
}

func TestWriteClipboardVaultSections_NoDuplicateHeaders(t *testing.T) {
	dir := t.TempDir()
	_ = WriteClipboardVaultSections(dir, 45, 300)
	_ = WriteClipboardVaultSections(dir, 60, 600)
	data, _ := os.ReadFile(dir + "/tegata.toml")
	content := string(data)
	if strings.Count(content, "[clipboard]") != 1 {
		t.Errorf("[clipboard] appears %d times after two writes, want 1", strings.Count(content, "[clipboard]"))
	}
	if strings.Count(content, "[vault]") != 1 {
		t.Errorf("[vault] appears %d times after two writes, want 1", strings.Count(content, "[vault]"))
	}
}

func TestWriteClipboardVaultSections_AuditAddedAfter(t *testing.T) {
	// Simulate: user writes clipboard/vault first, then audit setup adds [audit].
	// Then user modifies timeouts — [audit] must survive.
	dir := t.TempDir()
	if err := WriteClipboardVaultSections(dir, 45, 300); err != nil {
		t.Fatalf("first WriteClipboardVaultSections: %v", err)
	}

	auditCfg := AuditConfig{
		Enabled:   true,
		Server:    "127.0.0.1:50051",
		EntityID:  "tegata-xyz",
		SecretKey: "supersecret",
	}
	if err := WriteAuditSection(dir, auditCfg); err != nil {
		t.Fatalf("WriteAuditSection: %v", err)
	}

	// Now modify timeouts — [audit] section must be preserved.
	if err := WriteClipboardVaultSections(dir, 90, 120); err != nil {
		t.Fatalf("second WriteClipboardVaultSections: %v", err)
	}

	data, _ := os.ReadFile(dir + "/tegata.toml")
	content := string(data)

	if !strings.Contains(content, "[audit]") {
		t.Error("[audit] section was lost after modifying timeouts")
	}
	if !strings.Contains(content, "tegata-xyz") {
		t.Error("[audit] entity_id was lost after modifying timeouts")
	}
	if !strings.Contains(content, "timeout = 90") {
		t.Error("updated clipboard timeout not present")
	}
	if !strings.Contains(content, "idle_timeout = 120") {
		t.Error("updated idle timeout not present")
	}
}

func TestWriteClipboardVaultSections_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := WriteClipboardVaultSections(dir, 60, 180); err != nil {
		t.Fatalf("WriteClipboardVaultSections: %v", err)
	}
	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if int(loaded.ClipboardTimeout.Seconds()) != 60 {
		t.Errorf("ClipboardTimeout: got %v, want 60s", loaded.ClipboardTimeout)
	}
	if int(loaded.IdleTimeout.Seconds()) != 180 {
		t.Errorf("IdleTimeout: got %v, want 180s", loaded.IdleTimeout)
	}
}
