package main

import (
	"fmt"
	"os"

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

	configCmd.AddCommand(showCmd)
	return configCmd
}
