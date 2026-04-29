package main

import (
	"fmt"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/config"
	"github.com/spf13/cobra"
)

func newChangePassphraseCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "change-passphrase",
		Short:   "Change the vault passphrase",
		Args:    cobra.NoArgs,
		Example: `  tegata change-passphrase`,
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultPath, err := resolveVaultPath(cmd)
			if err != nil {
				return err
			}

			passphrase, err := promptPassphrase("Passphrase: ")
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
			var builder *audit.EventBuilder
			if cfg.Audit.Enabled {
				auditClient, clientErr := audit.NewClientFromConfig(cfg.Audit)
				if clientErr != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Audit unavailable: %v\n", clientErr)
				} else {
					defer func() { _ = auditClient.Close() }()
					var memErr error
					builder, memErr = audit.NewEventBuilderMemQueue(auditClient)
					if memErr != nil {
						_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Audit unavailable: %v\n", memErr)
					}
				}
			}
			if builder != nil {
				builder.OnHashStored = func(eventID, hashValue string) {
					if err := mgr.SetAuditHash(eventID, hashValue); err != nil {
						_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to store audit hash: %v\n", err)
					}
				}
				if logErr := builder.LogEvent("vault-unlock", "", "", audit.Hostname(), true); logErr != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Audit log failed: %v\n", logErr)
				}
			}

			newPassphrase, err := promptNewPassphrase()
			if err != nil {
				return err
			}
			defer zeroBytes(newPassphrase)

			if err := mgr.ChangePassphrase(newPassphrase); err != nil {
				return fmt.Errorf("changing passphrase: %w", err)
			}

			if builder != nil {
				if logErr := builder.LogEvent("vault-passphrase-change", "", "", audit.Hostname(), true); logErr != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Audit log failed: %v\n", logErr)
				}
				_ = builder.Close()
			}

			fmt.Println("Passphrase changed successfully.")
			return nil
		},
	}
}
