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
func formatAuditSection(cfg AuditConfig) string {
	var buf bytes.Buffer
	buf.WriteString("[audit]\n")
	buf.WriteString(fmt.Sprintf("enabled = %t\n", cfg.Enabled))
	if cfg.Server != "" {
		buf.WriteString(fmt.Sprintf("server = %q\n", cfg.Server))
	}
	if cfg.PrivilegedServer != "" {
		buf.WriteString(fmt.Sprintf("privileged_server = %q\n", cfg.PrivilegedServer))
	}
	if cfg.EntityID != "" {
		buf.WriteString(fmt.Sprintf("entity_id = %q\n", cfg.EntityID))
	}
	if cfg.KeyVersion > 0 {
		buf.WriteString(fmt.Sprintf("key_version = %d\n", cfg.KeyVersion))
	}
	if cfg.SecretKey != "" {
		buf.WriteString(fmt.Sprintf("secret_key = %q\n", cfg.SecretKey))
	}
	if cfg.Insecure {
		buf.WriteString("insecure = true\n")
	}
	if cfg.DockerComposePath != "" {
		buf.WriteString(fmt.Sprintf("docker_compose_path = %q\n", cfg.DockerComposePath))
	}
	buf.WriteString(fmt.Sprintf("auto_start = %t\n", cfg.AutoStart))
	return buf.String()
}
