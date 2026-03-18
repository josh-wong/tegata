package main

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

// newUICmd returns the `tegata ui` cobra command that launches the interactive
// full-screen terminal UI.
func newUICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Interactive terminal UI",
		Long: `Launch the interactive terminal user interface for Tegata.

If no vault is found, the first-time setup wizard guides you through vault
creation. If a vault exists, you will be prompted to unlock it.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve vault path optionally — absence is OK here; the TUI
			// wizard handles new-vault creation.
			vaultPath := resolveVaultPathOptional(cmd)
			m := initialModel(vaultPath)
			// Do NOT redirect stdout before p.Run() (pitfall 3 from RESEARCH.md).
			p := tea.NewProgram(m, tea.WithAltScreen())
			_, err := p.Run()
			return err
		},
	}
	cmd.Flags().String("vault", "", "Path to vault file or directory")
	return cmd
}

// resolveVaultPathOptional resolves the vault path using the standard resolution
// order but returns an empty string rather than an error when no vault is found.
// The TUI handles the no-vault case by launching the setup wizard.
//
// Resolution order:
//  1. --vault flag (directory or file path)
//  2. TEGATA_VAULT env var (directory or file path)
//  3. ./vault.tegata — present only if the file exists
func resolveVaultPathOptional(cmd *cobra.Command) string {
	if flagVal, _ := cmd.Flags().GetString("vault"); flagVal != "" {
		p, err := resolvePathArg(flagVal)
		if err == nil {
			return p
		}
	}

	if envVal := os.Getenv("TEGATA_VAULT"); envVal != "" {
		p, err := resolvePathArg(envVal)
		if err == nil {
			return p
		}
	}

	// Check for vault.tegata in the current working directory.
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	p, err := resolvePathArg(cwd)
	if err != nil {
		return ""
	}
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}
