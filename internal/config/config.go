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
	// Audit holds optional ScalarDL Ledger integration settings. When
	// Audit.Enabled is false (the default) the audit layer is inactive.
	Audit AuditConfig
}

// AuditConfig holds settings for the optional ScalarDL Ledger audit layer.
// All fields default to safe/disabled values when the [audit] section is
// absent from tegata.toml.
type AuditConfig struct {
	// Enabled controls whether audit events are submitted to the ledger.
	Enabled bool
	// Server is the gRPC address of the ScalarDL Ledger (e.g. "localhost:50051").
	Server string
	// CertPath, KeyPath, and CACertPath are optional TLS certificate paths for
	// mutual TLS authentication with the ledger.
	CertPath   string
	KeyPath    string
	CACertPath string
	// EntityID is the ScalarDL entity identifier for this vault.
	EntityID string
	// KeyVersion is the ScalarDL key version to use when submitting contracts.
	KeyVersion uint32
	// QueueMaxEvents is the maximum number of events kept in the offline queue
	// before the oldest entries are dropped. Default: 10000.
	QueueMaxEvents int
}

// tomlAuditConfig is the TOML deserialization intermediate for [audit].
// Pointer fields distinguish "not set" from "zero value".
type tomlAuditConfig struct {
	Enabled        *bool   `toml:"enabled"`
	Server         *string `toml:"server"`
	CertPath       *string `toml:"cert_path"`
	KeyPath        *string `toml:"key_path"`
	CACertPath     *string `toml:"ca_cert_path"`
	EntityID       *string `toml:"entity_id"`
	KeyVersion     *uint32 `toml:"key_version"`
	QueueMaxEvents *int    `toml:"queue_max_events"`
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
	Audit tomlAuditConfig `toml:"audit"`
}

const (
	defaultClipboardTimeout = 45
	defaultIdleTimeout      = 300
	configFileName          = "tegata.toml"
)

// DefaultConfig returns a Config with the default values: 45-second clipboard
// timeout and 300-second (5-minute) idle timeout. The audit layer is disabled
// by default with a 10,000-event offline queue capacity.
func DefaultConfig() Config {
	return Config{
		ClipboardTimeout: time.Duration(defaultClipboardTimeout) * time.Second,
		IdleTimeout:      time.Duration(defaultIdleTimeout) * time.Second,
		Audit: AuditConfig{
			Enabled:        false,
			QueueMaxEvents: 10000,
		},
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

	// Apply audit settings — only override when explicitly set in TOML.
	a := &tc.Audit
	if a.Enabled != nil {
		cfg.Audit.Enabled = *a.Enabled
	}
	if a.Server != nil {
		cfg.Audit.Server = *a.Server
	}
	if a.CertPath != nil {
		cfg.Audit.CertPath = *a.CertPath
	}
	if a.KeyPath != nil {
		cfg.Audit.KeyPath = *a.KeyPath
	}
	if a.CACertPath != nil {
		cfg.Audit.CACertPath = *a.CACertPath
	}
	if a.EntityID != nil {
		cfg.Audit.EntityID = *a.EntityID
	}
	if a.KeyVersion != nil {
		cfg.Audit.KeyVersion = *a.KeyVersion
	}
	if a.QueueMaxEvents != nil {
		cfg.Audit.QueueMaxEvents = *a.QueueMaxEvents
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

[audit]
# Enable tamper-evident audit logging via ScalarDL Ledger (default: false)
# enabled = false
#
# gRPC address of the ScalarDL Ledger server
# server = "localhost:50051"
#
# TLS certificate paths for mutual TLS (optional — omit for plaintext)
# cert_path = ""
# key_path = ""
# ca_cert_path = ""
#
# ScalarDL entity identifier for this vault
# entity_id = ""
#
# ScalarDL key version to use when submitting contracts
# key_version = 1
#
# Maximum events retained in the offline queue before oldest are dropped
# queue_max_events = 10000
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
