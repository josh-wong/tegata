package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/config"
	tegerrors "github.com/josh-wong/tegata/internal/errors"
	"github.com/josh-wong/tegata/internal/i18n"
	"github.com/josh-wong/tegata/internal/vault"
	"github.com/spf13/cobra"
)

func newVerifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "verify",
		Short:   i18n.T("cmd.verify.short"),
		Long:    i18n.T("cmd.verify.long"),
		Example: i18n.T("cmd.verify.example"),
		Args:    cobra.NoArgs,
		RunE:    runVerify,
	}
}

// formatFault converts an "id: detail" fault string into a readable sentence.
func formatFault(f string) string {
	idx := strings.Index(f, ": ")
	if idx < 0 {
		return f
	}
	id, detail := f[:idx], f[idx+2:]
	if strings.HasPrefix(detail, "error: ") {
		return i18n.Tf("cmd.verify.error.record",
			map[string]any{"Hash": id, "Msg": strings.TrimPrefix(detail, "error: ")})
	}
	return i18n.Tf("cmd.verify.error.field", map[string]any{"Field": detail, "Hash": id})
}

func runVerify(cmd *cobra.Command, _ []string) error {
	vaultPath, err := resolveVaultPath(cmd)
	if err != nil {
		return err
	}

	cfg, err := config.Load(vaultDir(vaultPath))
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.T("cmd.verify.error.loadConfig"), err)
	}

	if !cfg.Audit.Enabled {
		printAuditNotEnabledHint(os.Stderr)
		return nil
	}

	passphrase, err := promptPassphrase(i18n.T("cmd.prompt.passphrase"))
	if err != nil {
		return err
	}
	mgr, err := openAndUnlock(vaultPath, passphrase)
	zeroBytes(passphrase)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.T("cmd.verify.error.unlockVault"), err)
	}
	defer mgr.Close()

	secretFromVault := mgr.GetSecret("audit.secret_key")
	if secretFromVault != "" {
		cfg.Audit.SecretKey = secretFromVault
	}
	defer func() { cfg.Audit.SecretKey = "" }()

	hashes := mgr.AuditHashes()
	defer vault.ZeroAuditHashes(hashes)

	if err := audit.CheckLedgerAvailability(cfg.Audit); err != nil {
		return err
	}
	client, err := audit.NewClientFromConfig(cfg.Audit)
	if err != nil {
		return fmt.Errorf("%w: connecting to ledger: %s", tegerrors.ErrNetworkFailed, err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := audit.VerifyAll(ctx, client, cfg.Audit.EntityID, hashes)
	if err != nil {
		return err
	}

	if result.EventCount == 0 && result.Skipped == 0 {
		_, _ = fmt.Fprintln(os.Stdout, i18n.T("cmd.verify.empty"))
		return nil
	}

	if result.Skipped > 0 {
		fmt.Fprintf(os.Stderr, "%s",
			i18n.Tf("cmd.verify.note.unverified", map[string]any{"Count": result.Skipped}))
	}

	if result.Valid {
		if result.EventCount > 0 {
			fmt.Print(i18n.Tf("cmd.verify.success", map[string]any{"Count": result.EventCount}))
		} else {
			fmt.Print(i18n.Tf("cmd.verify.warn.allUnverified", map[string]any{"Count": result.Skipped}))
		}
		return nil
	}

	fmt.Fprint(os.Stderr, i18n.T("cmd.verify.tampered"))
	for _, f := range result.Faults {
		fmt.Fprintf(os.Stderr, "%s", i18n.Tf("cmd.verify.detail", map[string]any{"Detail": formatFault(f)}))
	}
	return reportedError{tegerrors.ErrIntegrityViolation}
}
