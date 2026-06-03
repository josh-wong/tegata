package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/i18n"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "export",
		Short:   i18n.T("cmd.export.short"),
		Long:    i18n.T("cmd.export.long"),
		Example: i18n.T("cmd.export.example"),
		RunE:    runExport,
	}

	cmd.Flags().String("out", "", i18n.T("cmd.export.flag.out"))
	cmd.Flags().String("vault", "", i18n.T("cmd.export.flag.vault"))
	return cmd
}

func runExport(cmd *cobra.Command, args []string) error {
	vaultPath, err := resolveVaultPath(cmd)
	if err != nil {
		return err
	}

	outPath, _ := cmd.Flags().GetString("out")
	if outPath == "" {
		outPath = filepath.Join(filepath.Dir(vaultPath), "vault.tegata-backup")
	}

	vaultPass, err := promptPassphrase(i18n.T("cmd.export.prompt.vaultPass"))
	if err != nil {
		return fmt.Errorf("%s", i18n.Tf("cmd.export.error.readVaultPass", map[string]any{"Err": err}))
	}
	defer zeroBytes(vaultPass)

	mgr, err := openAndUnlock(vaultPath, vaultPass)
	if err != nil {
		return fmt.Errorf("%s", i18n.Tf("cmd.export.error.unlockVault", map[string]any{"Err": err}))
	}
	defer mgr.Close()

	builder := setupAuditBuilder(cmd.ErrOrStderr(), vaultDir(vaultPath), vaultPass, mgr)
	if builder != nil {
		defer func() { _ = builder.Close() }()
	}

	// Prompt for export passphrase directly via term.ReadPassword.
	// The export passphrase is a new credential and must never be read from
	// the TEGATA_PASSPHRASE environment variable.
	fmt.Fprintln(os.Stderr, i18n.T("cmd.export.info.choosePass"))

	var exportPass []byte
	for {
		fmt.Fprint(os.Stderr, i18n.T("cmd.export.prompt.exportPass"))
		exportPass, err = term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			zeroBytes(exportPass)
			return fmt.Errorf("%s", i18n.Tf("cmd.export.error.readExportPass", map[string]any{"Err": err}))
		}

		if len(exportPass) < 8 {
			zeroBytes(exportPass)
			fmt.Fprintln(os.Stderr, i18n.T("cmd.export.error.shortPass"))
			continue
		}

		displayStrengthMeter(exportPass)

		fmt.Fprint(os.Stderr, i18n.T("cmd.export.prompt.confirmPass"))
		confirm, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			zeroBytes(exportPass)
			return fmt.Errorf("%s", i18n.Tf("cmd.export.error.readConfirmPass", map[string]any{"Err": err}))
		}

		if !bytes.Equal(exportPass, confirm) {
			zeroBytes(confirm)
			zeroBytes(exportPass)
			fmt.Fprintln(os.Stderr, i18n.T("cmd.export.error.passMismatch"))
			continue
		}
		zeroBytes(confirm)
		break
	}
	defer zeroBytes(exportPass)

	data, err := mgr.ExportCredentials(exportPass)
	if err != nil {
		return fmt.Errorf("%s", i18n.Tf("cmd.export.error.export", map[string]any{"Err": err}))
	}
	defer zeroBytes(data)

	if err := os.WriteFile(outPath, data, 0600); err != nil {
		return fmt.Errorf("%s", i18n.Tf("cmd.export.error.writeFile", map[string]any{"Path": outPath, "Err": err}))
	}

	if builder != nil {
		if logErr := builder.LogEvent("credential-export", "", "", audit.Hostname(), true); logErr != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s",
				i18n.Tf("cmd.export.warn.auditFailed", map[string]any{"Err": logErr}))
		}
	}

	credCount := len(mgr.ListCredentials())
	fmt.Print(i18n.Tp("cmd.export.success", credCount, map[string]any{"Path": outPath}))
	return nil
}
