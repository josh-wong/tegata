package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteLanguage writes or replaces the top-level language key in tegata.toml
// located in dir. All other content is preserved. If tegata.toml does not
// exist, it is created.
func WriteLanguage(dir, lang string) error {
	path := filepath.Join(dir, configFileName)
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	block := fmt.Sprintf("language = %q\n", lang)
	content := rewriteTopLevelKey(existing, "language", block)
	return os.WriteFile(path, []byte(content), 0600)
}
