package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteUI writes or replaces the [ui] section in tegata.toml located in dir.
// All other content is preserved. If tegata.toml does not exist, it is created.
func WriteUI(dir string, unlockCount int, bannerDismissed bool) error {
	path := filepath.Join(dir, configFileName)
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	block := fmt.Sprintf("[ui]\nunlock_count = %d\nsupport_banner_dismissed = %t\n", unlockCount, bannerDismissed)
	content := rewriteSection(existing, "ui", block)
	return os.WriteFile(path, []byte(content), 0600)
}
