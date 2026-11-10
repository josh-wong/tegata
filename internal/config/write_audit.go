package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

// WriteAuditSection writes or replaces the [audit] section in tegata.toml
// located in dir. If tegata.toml does not exist it is created. If an [audit]
// section already exists it is replaced; otherwise the section is appended.
// Existing non-audit sections are preserved.
func WriteAuditSection(dir string, cfg AuditConfig) error {
	path := filepath.Join(dir, configFileName)
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	block := formatAuditSection(cfg)
	content := rewriteSection(existing, "audit", block)
	return os.WriteFile(path, []byte(content), 0600)
}

// formatAuditSection renders the [audit] TOML block from cfg.
// Note: secret_key is omitted since it is stored encrypted in the vault,
// not in plaintext tegata.toml.
func formatAuditSection(cfg AuditConfig) string {
	var buf bytes.Buffer
	buf.WriteString("[audit]\n")
	fmt.Fprintf(&buf, "enabled = %t\n", cfg.Enabled)
	if cfg.Server != "" {
		fmt.Fprintf(&buf, "server = %q\n", cfg.Server)
	}
	if cfg.PrivilegedServer != "" {
		fmt.Fprintf(&buf, "privileged_server = %q\n", cfg.PrivilegedServer)
	}
	if cfg.EntityID != "" {
		fmt.Fprintf(&buf, "entity_id = %q\n", cfg.EntityID)
	}
	if cfg.KeyVersion > 0 {
		fmt.Fprintf(&buf, "key_version = %d\n", cfg.KeyVersion)
	}
	if cfg.Insecure {
		buf.WriteString("insecure = true\n")
	}
	if cfg.DockerComposePath != "" {
		fmt.Fprintf(&buf, "docker_compose_path = %q\n", cfg.DockerComposePath)
	}
	if cfg.DockerProjectName != "" {
		fmt.Fprintf(&buf, "docker_project_name = %q\n", cfg.DockerProjectName)
	}
	fmt.Fprintf(&buf, "auto_start = %t\n", cfg.AutoStart)
	return buf.String()
}
