package main

import (
	"fmt"
	"os"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/i18n"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "import <backup-file>",
		Short:   i18n.T("cmd.import.short"),
		Long:    i18n.T("cmd.import.long"),
		Example: i18n.T("cmd.import.example"),
		Args:    cobra.ExactArgs(1),
		RunE:    runImport,
	}

	cmd.Flags().String("vault", "", i18n.T("cmd.import.flag.vault"))
	return cmd
}

func runImport(cmd *cobra.Command, args []string) error {
	backupPath := args[0]

	vaultPath, err := resolveVaultPath(cmd)
	if err != nil {
		return err
	}

	vaultPass, err := promptPassphrase(i18n.T("cmd.import.prompt.vaultPass"))
	if err != nil {
		return fmt.Errorf("%s", i18n.Tf("cmd.import.error.readVaultPass", map[string]any{"Err": err}))
	}
	defer zeroBytes(vaultPass)

	mgr, err := openAndUnlock(vaultPath, vaultPass)
	if err != nil {
		return fmt.Errorf("%s", i18n.Tf("cmd.import.error.unlockVault", map[string]any{"Err": err}))
	}
	defer mgr.Close()

	builder := setupAuditBuilder(cmd.ErrOrStderr(), vaultDir(vaultPath), vaultPass, mgr)
	if builder != nil {
		defer func() { _ = builder.Close() }()
	}

	const maxImportSize = 10 << 20
	info, err := os.Stat(backupPath)
	if err != nil {
		return fmt.Errorf("%s", i18n.Tf("cmd.import.error.readFile", map[string]any{"Path": backupPath, "Err": err}))
	}
	if info.Size() > maxImportSize {
		return fmt.Errorf("%s",
			i18n.Tf("cmd.import.error.fileTooLarge",
				map[string]any{"Size": info.Size(), "Max": maxImportSize}))
	}
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("%s", i18n.Tf("cmd.import.error.readFile", map[string]any{"Path": backupPath, "Err": err}))
	}

	var importPass []byte
	if envPass := os.Getenv("TEGATA_BACKUP_PASSPHRASE"); envPass != "" {
		fmt.Fprintln(os.Stderr, i18n.T("cmd.import.info.envPass"))
		importPass = []byte(envPass)
	} else {
		fmt.Fprint(os.Stderr, i18n.T("cmd.import.prompt.backupPass"))
		importPass, err = term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return fmt.Errorf("%s", i18n.Tf("cmd.import.error.readBackupPass", map[string]any{"Err": err}))
		}
	}
	defer zeroBytes(importPass)

	imported, skipped, err := mgr.ImportCredentials(data, importPass)
	if err != nil {
		return fmt.Errorf("%s", i18n.Tf("cmd.import.error.import", map[string]any{"Err": err}))
	}

	if builder != nil && imported > 0 {
		if logErr := builder.LogEvent("credential-import", "", "", audit.Hostname(), true); logErr != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s",
				i18n.Tf("cmd.import.warn.auditFailed", map[string]any{"Err": logErr}))
		}
	}

	fmt.Print(i18n.Tf("cmd.import.success", map[string]any{"Imported": imported, "Skipped": skipped}))
	return nil
}
