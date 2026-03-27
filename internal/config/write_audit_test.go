package config

import (
	"os"
	"strings"
	"testing"
)

func TestWriteAuditSection_NewFile(t *testing.T) {
	dir := t.TempDir()
	cfg := AuditConfig{
		Enabled:           true,
		Server:            "127.0.0.1:50051",
		PrivilegedServer:  "127.0.0.1:50052",
		EntityID:          "tegata-a3f2c810",
		KeyVersion:        1,
		SecretKey:         "deadbeef",
		Insecure:          true,
		DockerComposePath: "/home/user/.tegata/docker/docker-compose.yml",
	}
	if err := WriteAuditSection(dir, cfg); err != nil {
		t.Fatalf("WriteAuditSection: %v", err)
	}
	data, _ := os.ReadFile(dir + "/tegata.toml")
	content := string(data)
	if !strings.Contains(content, "[audit]") {
		t.Error("written file does not contain [audit]")
	}
	if !strings.Contains(content, "docker_compose_path") {
		t.Error("written file does not contain docker_compose_path")
	}
	if !strings.Contains(content, "tegata-a3f2c810") {
		t.Error("written file does not contain entity_id value")
	}
}

func TestWriteAuditSection_Append(t *testing.T) {
	dir := t.TempDir()
	existing := "[clipboard]\ntimeout = 45\n"
	os.WriteFile(dir+"/tegata.toml", []byte(existing), 0600)

	cfg := AuditConfig{Enabled: true, Server: "127.0.0.1:50051", EntityID: "tegata-test"}
	if err := WriteAuditSection(dir, cfg); err != nil {
		t.Fatalf("WriteAuditSection: %v", err)
	}
	data, _ := os.ReadFile(dir + "/tegata.toml")
	content := string(data)
	if !strings.Contains(content, "[clipboard]") {
		t.Error("append destroyed existing [clipboard] section")
	}
	if !strings.Contains(content, "[audit]") {
		t.Error("append did not add [audit] section")
	}
	if count := strings.Count(content, "[audit]"); count != 1 {
		t.Errorf("[audit] appears %d times, want 1", count)
	}
}

func TestWriteAuditSection_Replace(t *testing.T) {
	dir := t.TempDir()
	existing := "[clipboard]\ntimeout = 45\n\n[audit]\nenabled = false\nserver = \"old:50051\"\n"
	os.WriteFile(dir+"/tegata.toml", []byte(existing), 0600)

	cfg := AuditConfig{Enabled: true, Server: "127.0.0.1:50051", EntityID: "tegata-new"}
	if err := WriteAuditSection(dir, cfg); err != nil {
		t.Fatalf("WriteAuditSection: %v", err)
	}
	data, _ := os.ReadFile(dir + "/tegata.toml")
	content := string(data)
	if count := strings.Count(content, "[audit]"); count != 1 {
		t.Errorf("[audit] appears %d times after replace, want exactly 1", count)
	}
	if strings.Contains(content, "old:50051") {
		t.Error("old server address still present after replace")
	}
	if !strings.Contains(content, "127.0.0.1:50051") {
		t.Error("new server address not present after replace")
	}
	if !strings.Contains(content, "[clipboard]") {
		t.Error("replace destroyed existing [clipboard] section")
	}
}

func TestWriteAuditSection_NoDuplicateHeaders(t *testing.T) {
	dir := t.TempDir()
	cfg := AuditConfig{Enabled: true, EntityID: "tegata-x"}
	_ = WriteAuditSection(dir, cfg)
	_ = WriteAuditSection(dir, cfg)
	data, _ := os.ReadFile(dir + "/tegata.toml")
	if count := strings.Count(string(data), "[audit]"); count != 1 {
		t.Errorf("two calls produced %d [audit] headers, want 1", count)
	}
}

func TestWriteAuditSection_DockerComposePath(t *testing.T) {
	dir := t.TempDir()
	cfg := AuditConfig{
		Enabled:           true,
		DockerComposePath: "/home/user/.tegata/docker/docker-compose.yml",
	}
	_ = WriteAuditSection(dir, cfg)
	data, _ := os.ReadFile(dir + "/tegata.toml")
	if !strings.Contains(string(data), "docker_compose_path") {
		t.Error("docker_compose_path not written")
	}
}

func TestDockerComposePath_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfg := AuditConfig{
		Enabled:           true,
		Server:            "127.0.0.1:50051",
		PrivilegedServer:  "127.0.0.1:50052",
		EntityID:          "tegata-abc12345",
		KeyVersion:        1,
		SecretKey:         "secret",
		Insecure:          true,
		DockerComposePath: "/home/user/.tegata/docker/docker-compose.yml",
	}
	_ = WriteAuditSection(dir, cfg)
	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Audit.DockerComposePath != cfg.DockerComposePath {
		t.Errorf("DockerComposePath round-trip: got %q, want %q",
			loaded.Audit.DockerComposePath, cfg.DockerComposePath)
	}
}
