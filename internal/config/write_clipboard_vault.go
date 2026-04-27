package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteClipboardVaultSections writes or replaces the [clipboard] and [vault]
// sections in tegata.toml located in dir. Any other sections (e.g. [audit])
// are preserved unchanged. If tegata.toml does not exist, it is created. If a
// section already exists, it is replaced in place; otherwise it is appended.
func WriteClipboardVaultSections(dir string, clipboardTimeout, idleTimeout int) error {
	path := filepath.Join(dir, configFileName)
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	clipBlock := fmt.Sprintf("[clipboard]\ntimeout = %d\n", clipboardTimeout)
	vaultBlock := fmt.Sprintf("[vault]\nidle_timeout = %d\n", idleTimeout)

	content := rewriteSection(existing, "clipboard", clipBlock)
	content = rewriteSection([]byte(content), "vault", vaultBlock)

	return os.WriteFile(path, []byte(content), 0600)
}

// rewriteSection replaces the named TOML section in existing with newBlock,
// or appends newBlock if the section is absent. All other sections are
// preserved. newBlock must start with "[name]\n" and end with "\n".
func rewriteSection(existing []byte, name, newBlock string) string {
	header := []byte("[" + name + "]")

	hasSection := bytes.HasPrefix(existing, header) ||
		bytes.Contains(existing, append([]byte("\n"), header...))

	if !hasSection {
		content := strings.TrimRight(string(existing), "\n")
		if content != "" {
			return content + "\n\n" + newBlock
		}
		return newBlock
	}

	// Find the start of the section header.
	start := bytes.Index(existing, header)
	if start > 0 && existing[start-1] != '\n' {
		// The first match was mid-line (e.g. a value containing "[name]").
		// Find the newline-prefixed occurrence instead.
		idx := bytes.Index(existing[1:], append([]byte("\n"), header...))
		if idx >= 0 {
			start = idx + 2 // skip past the leading \n
		}
	}

	// Find the end of this section: the start of the next [section] header or EOF.
	rest := existing[start:]
	end := len(existing)
	lines := bytes.Split(rest, []byte("\n"))
	offset := start
	for i, line := range lines {
		if i == 0 {
			offset += len(line) + 1
			continue
		}
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) > 0 && trimmed[0] == '[' && trimmed[len(trimmed)-1] == ']' {
			end = offset
			break
		}
		offset += len(line) + 1
	}

	before := strings.TrimRight(string(existing[:start]), "\n")
	after := ""
	if end < len(existing) {
		after = string(existing[end:])
	}

	result := ""
	if before != "" {
		result = before + "\n\n"
	}
	result += newBlock
	if after != "" {
		result += "\n" + after
	}
	return result
}
