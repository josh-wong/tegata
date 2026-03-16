// Package config manages the tegata.toml configuration file that lives
// alongside the vault on the USB drive. It handles loading, parsing, and
// writing default configuration values.
//
// This is a stub implementation providing the API surface for CLI commands.
// Full implementation is delivered by Plan 02-03.
package config

import "fmt"

// Config holds the parsed configuration values from tegata.toml.
type Config struct {
	Clipboard ClipboardConfig
	Vault     VaultConfig
}

// ClipboardConfig holds clipboard-related settings.
type ClipboardConfig struct {
	Timeout int // Seconds before auto-clear (default 45).
}

// VaultConfig holds vault-related settings.
type VaultConfig struct {
	IdleTimeout int // Seconds before auto-lock (default 300).
}

// Load reads and parses the tegata.toml file from the given directory.
// If the file does not exist, built-in defaults are returned.
func Load(dir string) (Config, error) {
	return Config{
		Clipboard: ClipboardConfig{Timeout: 45},
		Vault:     VaultConfig{IdleTimeout: 300},
	}, nil
}

// WriteDefaults creates a tegata.toml file in the given directory with all
// default values as commented lines.
func WriteDefaults(dir string) error {
	// Stub: will be implemented by Plan 02-03
	return fmt.Errorf("config.WriteDefaults: not yet implemented")
}
