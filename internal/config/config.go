// Package config handles loading and managing Tegata configuration from
// tegata.toml files. Configuration travels with the vault on USB drives.
package config

import (
	"time"
)

// Config holds the runtime configuration for Tegata.
type Config struct {
	ClipboardTimeout time.Duration
	IdleTimeout      time.Duration
}

// DefaultConfig returns a Config with the default values.
func DefaultConfig() Config {
	return Config{}
}

// Load reads tegata.toml from dir. If the file does not exist, it returns
// DefaultConfig with a nil error. Missing keys use default values.
func Load(dir string) (Config, error) {
	return Config{}, nil
}

// WriteDefaults creates a tegata.toml in dir with all settings as commented
// lines, serving as a documented template for users.
func WriteDefaults(dir string) error {
	return nil
}

// FormatEffective returns a human-readable display of the effective
// configuration. When hasFile is false, values are annotated with "(default)".
func FormatEffective(cfg Config, hasFile bool) string {
	return ""
}
