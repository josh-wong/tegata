package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export all credentials to an encrypted backup file",
		Long: `Export encrypts all credentials in the vault to a portable .tegata-backup file.
The backup is protected by a new export passphrase that you choose — it is
independent of your vault passphrase. Anyone who has the backup file and its
passphrase can restore your credentials.`,
		Example: "  tegata export --out ~/backups/vault.tegata-backup",
		RunE:    runExport,
	}

	cmd.Flags().String("out", "", "output path for the backup file (default: vault.tegata-backup in vault directory)")
	cmd.Flags().String("vault", "", "path to vault file or directory")
	return cmd
}

func runExport(cmd *cobra.Command, args []string) error {
	vaultPath, err := resolveVaultPath(cmd)
	if err != nil {
		return err
	}

	// Determine output path.
	outPath, _ := cmd.Flags().GetString("out")
	if outPath == "" {
		outPath = filepath.Join(filepath.Dir(vaultPath), "vault.tegata-backup")
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

	// Prompt for export passphrase directly via term.ReadPassword.
	// The export passphrase is a new credential and must never be read from
	// the TEGATA_PASSPHRASE environment variable.
	fmt.Fprintln(os.Stderr, "Choose a passphrase for the backup file (separate from your vault passphrase).")

	var exportPass []byte
	for {
		fmt.Fprint(os.Stderr, "Export passphrase: ")
		exportPass, err = term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return fmt.Errorf("reading export passphrase: %w", err)
		}

		if len(exportPass) < 8 {
			fmt.Fprintln(os.Stderr, "Error: export passphrase must be at least 8 characters.")
			continue
		}

		displayStrengthMeter(len(exportPass))

		fmt.Fprint(os.Stderr, "Confirm export passphrase: ")
		confirm, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			zeroBytes(exportPass)
			return fmt.Errorf("reading export passphrase confirmation: %w", err)
		}

		if !bytes.Equal(exportPass, confirm) {
			zeroBytes(confirm)
			zeroBytes(exportPass)
			fmt.Fprintln(os.Stderr, "Error: passphrases do not match. Try again.")
			continue
		}
		zeroBytes(confirm)
		break
	}
	defer zeroBytes(exportPass)

	data, err := mgr.ExportCredentials(exportPass)
	if err != nil {
		return fmt.Errorf("Error: export failed. %w", err)
	}

	if err := os.WriteFile(outPath, data, 0600); err != nil {
		return fmt.Errorf("Error: could not write backup file. Check the output path and try again.")
	}

	credCount := len(mgr.ListCredentials())
	fmt.Printf("Exported %d credential", credCount)
	if credCount != 1 {
		fmt.Print("s")
	}
	fmt.Printf(" to %s\n", outPath)
	return nil
}
