package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/josh-wong/tegata/internal/config"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
	}

	showCmd := &cobra.Command{
		Use:     "show",
		Short:   "Display effective configuration",
		Args:    cobra.NoArgs,
		Example: `  tegata config show`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Try to find the vault directory for config location.
			dir := "."
			vaultPath, err := resolveVaultPath(cmd)
			if err == nil {
				dir = vaultDir(vaultPath)
			} else {
				// Fall back to current directory.
				if cwd, cwdErr := os.Getwd(); cwdErr == nil {
					dir = cwd
				}
			}

			cfg, loadErr := config.Load(dir)
			hasFile := loadErr == nil

			fmt.Print(config.FormatEffective(cfg, hasFile))
			return nil
		},
	}

	setCmd := &cobra.Command{
		Use:     "set <key> <value>",
		Short:   "Set a configuration value",
		Args:    cobra.ExactArgs(2),
		Example: `  tegata config set audit.auto_start true
  tegata config set audit.auto_start false`,
		RunE: func(cmd *cobra.Command, args []string) error {
			key, value := args[0], args[1]

			// Resolve vault directory.
			dir := "."
			vaultPath, err := resolveVaultPath(cmd)
			if err == nil {
				dir = vaultDir(vaultPath)
			} else {
				if cwd, cwdErr := os.Getwd(); cwdErr == nil {
					dir = cwd
				}
			}

			switch key {
			case "audit.auto_start":
				var autoStart bool
				switch strings.ToLower(value) {
				case "true":
					autoStart = true
				case "false":
					autoStart = false
				default:
					return fmt.Errorf("invalid value %q: expected true or false", value)
				}

				// Load full config to preserve existing audit fields.
				cfg, err := config.Load(dir)
				if err != nil {
					return fmt.Errorf("loading config: %w", err)
				}
				cfg.Audit.AutoStart = autoStart
				if err := config.WriteAuditSection(dir, cfg.Audit); err != nil {
					return fmt.Errorf("writing config: %w", err)
				}
				fmt.Printf("audit.auto_start set to %s\n", value)
				return nil
			default:
				return fmt.Errorf("unknown config key: %s (supported: audit.auto_start)", key)
			}
		},
	}

	configCmd.AddCommand(showCmd)
	configCmd.AddCommand(setCmd)
	return configCmd
}
