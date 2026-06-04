package main

import (
	"fmt"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/config"
	"github.com/josh-wong/tegata/internal/i18n"
	"github.com/spf13/cobra"
)

func newChangePassphraseCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "change-passphrase",
		Short:   i18n.T("cmd.changePassphrase.short"),
		Args:    cobra.NoArgs,
		Example: i18n.T("cmd.changePassphrase.example"),
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultPath, err := resolveVaultPath(cmd)
			if err != nil {
				return err
			}

			passphrase, err := promptPassphrase(i18n.T("cmd.prompt.passphrase"))
			if err != nil {
				return err
			}
			defer zeroBytes(passphrase)

			mgr, err := openAndUnlock(vaultPath, passphrase)
			if err != nil {
				return err
			}
			defer mgr.Close()

			// Use an in-memory queue rather than the disk-backed queue: after the
			// passphrase changes, the queue key (derived from the old passphrase)
			// becomes invalid, so writing it to disk would leave an unreadable file.
			cfg, _ := config.Load(vaultDir(vaultPath))

			// Load HMAC secret from vault and inject into config.
			secretFromVault := mgr.GetSecret("audit.secret_key")
			if secretFromVault != "" {
				cfg.Audit.SecretKey = secretFromVault
			}
			defer func() { cfg.Audit.SecretKey = "" }()

			var builder *audit.EventBuilder
			if cfg.Audit.Enabled {
				auditClient, clientErr := audit.NewClientFromConfig(cfg.Audit)
				if clientErr != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s",
						i18n.Tf("cmd.changePassphrase.warn.auditUnavailable", map[string]any{"Err": clientErr}))
				} else {
					defer func() { _ = auditClient.Close() }()
					var memErr error
					builder, memErr = audit.NewEventBuilderMemQueue(auditClient)
					if memErr != nil {
						_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s",
							i18n.Tf("cmd.changePassphrase.warn.auditUnavailable", map[string]any{"Err": memErr}))
					}
				}
			}
			if builder != nil {
				// Deferred close acts as a safety net for early returns. The explicit
				// builder.Close() call below (after the passphrase-change event) is
				// the normal path; this only fires if an early return is hit first.
				defer func() {
					if builder != nil {
						_ = builder.Close()
					}
				}()
				builder.OnHashStored = func(eventID, hashValue string) {
					if err := mgr.SetAuditHash(eventID, hashValue); err != nil {
						_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s",
							i18n.Tf("cmd.changePassphrase.warn.auditHash", map[string]any{"Err": err}))
					}
				}
				if logErr := builder.LogEvent("vault-unlock", "", "", audit.Hostname(), true); logErr != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s",
						i18n.Tf("cmd.changePassphrase.warn.auditFailed", map[string]any{"Err": logErr}))
				}
			}

			newPassphrase, err := promptNewPassphrase()
			if err != nil {
				return err
			}
			defer zeroBytes(newPassphrase)

			if err := mgr.ChangePassphrase(newPassphrase); err != nil {
				return fmt.Errorf("%s: %w", i18n.T("cmd.changePassphrase.error.change"), err)
			}

			if builder != nil {
				if logErr := builder.LogEvent("vault-passphrase-change", "", "", audit.Hostname(), true); logErr != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s",
						i18n.Tf("cmd.changePassphrase.warn.auditFailed", map[string]any{"Err": logErr}))
				}
				_ = builder.Close()
				builder = nil // prevent deferred safety-net close from double-closing
			}

			fmt.Println(i18n.T("cmd.changePassphrase.success"))
			return nil
		},
	}
}
