// Package config handles loading and managing Tegata configuration from
// tegata.toml files. Configuration travels with the vault on USB drives.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// Config holds the runtime configuration for Tegata.
type Config struct {
	ClipboardTimeout time.Duration
	IdleTimeout      time.Duration
}

// tomlConfig is the intermediate deserialization struct. Pointer fields
// distinguish "not set" from "zero value".
type tomlConfig struct {
	Clipboard struct {
		Timeout *int `toml:"timeout"`
	} `toml:"clipboard"`
	Vault struct {
		IdleTimeout *int `toml:"idle_timeout"`
	} `toml:"vault"`
}

const (
	defaultClipboardTimeout = 45
	defaultIdleTimeout      = 300
	configFileName          = "tegata.toml"
)

// DefaultConfig returns a Config with the default values: 45-second clipboard
// timeout and 300-second (5-minute) idle timeout.
func DefaultConfig() Config {
	return Config{
		ClipboardTimeout: time.Duration(defaultClipboardTimeout) * time.Second,
		IdleTimeout:      time.Duration(defaultIdleTimeout) * time.Second,
	}
}

// Load reads tegata.toml from dir. If the file does not exist, it returns
// DefaultConfig with a nil error. Missing keys use default values.
func Load(dir string) (Config, error) {
	path := filepath.Join(dir, configFileName)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return Config{}, fmt.Errorf("reading config: %w", err)
	}

	var tc tomlConfig
	if err := toml.Unmarshal(data, &tc); err != nil {
		return Config{}, fmt.Errorf("parsing %s: %w", configFileName, err)
	}

	cfg := DefaultConfig()
	if tc.Clipboard.Timeout != nil {
		cfg.ClipboardTimeout = time.Duration(*tc.Clipboard.Timeout) * time.Second
	}
	if tc.Vault.IdleTimeout != nil {
		cfg.IdleTimeout = time.Duration(*tc.Vault.IdleTimeout) * time.Second
	}

	return cfg, nil
}

// WriteDefaults creates a tegata.toml in dir with all settings as commented
// lines, serving as a documented template for users.
func WriteDefaults(dir string) error {
	content := `# Tegata configuration
# Settings travel with the vault on USB.

[clipboard]
# Auto-clear timeout in seconds (default: 45)
# timeout = 45

[vault]
# Auto-lock timeout in seconds (default: 300)
# idle_timeout = 300
`
	path := filepath.Join(dir, configFileName)
	return os.WriteFile(path, []byte(content), 0644)
}

// FormatEffective returns a human-readable display of the effective
// configuration. When hasFile is false, values are annotated with "(default)".
func FormatEffective(cfg Config, hasFile bool) string {
	var b strings.Builder

	clipSec := int(cfg.ClipboardTimeout.Seconds())
	idleSec := int(cfg.IdleTimeout.Seconds())

	suffix := ""
	if !hasFile {
		suffix = "  (default)"
	}

	fmt.Fprintf(&b, "clipboard.timeout = %d%s\n", clipSec, suffix)
	fmt.Fprintf(&b, "vault.idle_timeout = %d%s\n", idleSec, suffix)

	return b.String()
}
