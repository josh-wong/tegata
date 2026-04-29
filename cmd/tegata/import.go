package main

import (
	"fmt"
	"os"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import <backup-file>",
		Short: "Import credentials from an encrypted backup file",
		Long: `Import restores credentials from a .tegata-backup file into the current vault.
Credentials whose label already exists in the vault are skipped.
You will be prompted for the backup passphrase (the one set during export).

For scripted restore flows the backup passphrase may be supplied via the
TEGATA_BACKUP_PASSPHRASE environment variable. This variable is intentionally
separate from TEGATA_PASSPHRASE (the vault passphrase). Export never reads
this variable — an interactive prompt is always required to set a new backup
passphrase — so the variable is safe to set in restore automation without
risking accidental export with a known passphrase.`,
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
		return fmt.Errorf("could not unlock vault: %w", err)
	}
	defer mgr.Close()

	builder := setupAuditBuilder(cmd.ErrOrStderr(), vaultDir(vaultPath), vaultPass, mgr)
	if builder != nil {
		defer func() { _ = builder.Close() }()
	}

	// Read backup file with size guard (10 MB max, matching GUI).
	const maxImportSize = 10 << 20
	info, err := os.Stat(backupPath)
	if err != nil {
		return fmt.Errorf("reading backup file %q: %w", backupPath, err)
	}
	if info.Size() > maxImportSize {
		return fmt.Errorf("backup file too large (%d bytes, max %d)", info.Size(), maxImportSize)
	}
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("reading backup file %q: %w", backupPath, err)
	}

	// Prompt for the backup passphrase. TEGATA_BACKUP_PASSPHRASE can be set
	// for scripted restore flows, keeping it independent of TEGATA_PASSPHRASE
	// (the vault passphrase). Falls back to an interactive prompt.
	var importPass []byte
	if envPass := os.Getenv("TEGATA_BACKUP_PASSPHRASE"); envPass != "" {
		fmt.Fprintln(os.Stderr, "! Using backup passphrase from TEGATA_BACKUP_PASSPHRASE")
		importPass = []byte(envPass)
	} else {
		fmt.Fprint(os.Stderr, "Backup passphrase: ")
		importPass, err = term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return fmt.Errorf("reading backup passphrase: %w", err)
		}
	}
	defer zeroBytes(importPass)

	imported, skipped, err := mgr.ImportCredentials(data, importPass)
	if err != nil {
		return fmt.Errorf("import failed: %w", err)
	}

	if builder != nil && imported > 0 {
		if logErr := builder.LogEvent("credential-import", "", "", audit.Hostname(), true); logErr != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Audit log failed: %v\n", logErr)
		}
	}

	fmt.Printf("%d imported, %d skipped (duplicate label)\n", imported, skipped)
	return nil
}
