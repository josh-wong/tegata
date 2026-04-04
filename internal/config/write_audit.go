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
	existing, _ := os.ReadFile(path) // ignore not-found

	block := formatAuditSection(cfg)

	// Check for existing [audit] block.
	if bytes.Contains(existing, []byte("\n[audit]")) || bytes.HasPrefix(existing, []byte("[audit]")) {
		return rewriteAuditSection(path, existing, block)
	}

	// Append new audit section.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = f.WriteString(block)
	return err
}

// formatAuditSection renders the [audit] TOML block from cfg.
func formatAuditSection(cfg AuditConfig) string {
	var buf bytes.Buffer
	buf.WriteString("\n[audit]\n")
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

// rewriteAuditSection replaces the existing [audit] section in the file
// content with the new block, preserving all other sections.
func rewriteAuditSection(path string, existing []byte, newBlock string) error {
	content := string(existing)

	// Find the start of [audit].
	start := bytes.Index(existing, []byte("[audit]"))
	if start > 0 && existing[start-1] != '\n' {
		// Shouldn't happen, but handle edge case.
		start = bytes.Index(existing[1:], []byte("\n[audit]"))
		if start >= 0 {
			start += 2 // skip past the \n
		}
	}
	if start < 0 {
		// No [audit] section found; just append.
		return os.WriteFile(path, append(existing, []byte(newBlock)...), 0600)
	}

	// Find the end of the [audit] section: the next [section] header or EOF.
	rest := content[start:]
	end := len(content)
	// Look for the next section header after [audit].
	lines := bytes.Split([]byte(rest), []byte("\n"))
	offset := start
	foundHeader := false
	for i, line := range lines {
		if i == 0 {
			offset += len(line) + 1
			continue
		}
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) > 0 && trimmed[0] == '[' && trimmed[len(trimmed)-1] == ']' {
			end = offset
			foundHeader = true
			break
		}
		offset += len(line) + 1
	}
	_ = foundHeader

	// Reconstruct: before [audit] + new block + after [audit] section.
	before := content[:start]
	after := ""
	if end < len(content) {
		after = content[end:]
	}

	// Ensure before ends with at most one newline before the new block.
	before = trimTrailingNewlines(before)

	result := before + newBlock
	if after != "" {
		result += "\n" + after
	}

	return os.WriteFile(path, []byte(result), 0600)
}

// trimTrailingNewlines removes trailing newlines from a string, keeping at most one.
func trimTrailingNewlines(s string) string {
	for len(s) > 1 && s[len(s)-1] == '\n' && s[len(s)-2] == '\n' {
		s = s[:len(s)-1]
	}
	return s
}
