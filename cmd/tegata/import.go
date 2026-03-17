package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import <backup-file>",
		Short: "Import credentials from an encrypted backup file",
		Long: `Import restores credentials from a .tegata-backup file into the current vault.
Credentials whose label already exists in the vault are skipped.
You will be prompted for the backup passphrase (the one set during export).`,
		Example: "  tegata import ~/backups/vault.tegata-backup",
		Args:    cobra.ExactArgs(1),
		RunE:    runImport,
	}

	cmd.Flags().String("vault", "", "path to vault file or directory")
	return cmd
}

func runImport(cmd *cobra.Command, args []string) error {
	backupPath := args[0]

	vaultPath, err := resolveVaultPath(cmd)
	if err != nil {
		return err
	}

	// Unlock vault with vault passphrase.
	vaultPass, err := promptPassphrase("Vault passphrase: ")
	if err != nil {
		return fmt.Errorf("reading vault passphrase: %w", err)
	}
	defer zeroBytes(vaultPass)

	mgr, err := openAndUnlock(vaultPath, vaultPass)
	if err != nil {
		return fmt.Errorf("Error: could not unlock vault. Check your passphrase and try again.")
	}
	defer mgr.Close()

	// Read backup file.
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("Error: could not read backup file %q. Check the path and try again.", backupPath)
	}

	// Prompt for the backup passphrase via promptPassphrase. The backup
	// passphrase is independent of the vault passphrase and is read using
	// the standard passphrase prompt (supports env var and piped input for
	// scripted restore flows).
	importPass, err := promptPassphrase("Backup passphrase: ")
	if err != nil {
		return fmt.Errorf("reading backup passphrase: %w", err)
	}
	defer zeroBytes(importPass)

	imported, skipped, err := mgr.ImportCredentials(data, importPass)
	if err != nil {
		return fmt.Errorf("Error: import failed. Check the backup passphrase and try again.")
	}

	fmt.Printf("%d imported, %d skipped (duplicate label)\n", imported, skipped)
	return nil
}
