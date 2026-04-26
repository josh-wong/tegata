package main

import (
	"fmt"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/auth"
	"github.com/josh-wong/tegata/internal/clipboard"
	"github.com/josh-wong/tegata/internal/config"
	"github.com/spf13/cobra"
)

func newGetCmd() *cobra.Command {
	var show bool

	cmd := &cobra.Command{
		Use:   "get <label>",
		Short: "Retrieve a static password",
		Args:  cobra.ExactArgs(1),
		Example: `  tegata get backup-key
  tegata get --show backup-key`,
		RunE: func(cmd *cobra.Command, args []string) error {
			label := args[0]

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

			// Load config and build audit builder while passphrase is in scope.
			cfg, _ := config.Load(vaultDir(vaultPath))
			builder, err := newEventBuilder(cfg, vaultDir(vaultPath), passphrase)
			if err != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: audit unavailable: %v\n", err)
			}
			if builder != nil {
				defer func() { _ = builder.Close() }()
				builder.OnHashStored = func(eventID, hashValue string) {
					if err := mgr.SetAuditHash(eventID, hashValue); err != nil {
						_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to store audit hash: %v\n", err)
					}
				}
			}

			cred, err := mgr.GetCredential(label)
			if err != nil {
				return err
			}

			password, err := auth.GetStaticPassword(cred)
			if err != nil {
				return err
			}
			defer zeroBytes(password)

			// Emit audit event after successful static password retrieval.
			if builder != nil {
				if logErr := builder.LogEvent("static", cred.Label, cred.Issuer, audit.Hostname(), true); logErr != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: audit log failed: %v\n", logErr)
				}
			}

			cm := clipboard.NewManager()
			defer cm.Close()
			if err := cm.CopyWithAutoClear(string(password), cfg.ClipboardTimeout); err != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: clipboard copy failed: %v\n", err)
			} else {
				fmt.Printf("Copied to clipboard (auto-clear in %ds)\n",
					int(cfg.ClipboardTimeout.Seconds()))
			}

			if show {
				fmt.Printf("Password: %s\n", password)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&show, "show", false, "display password in terminal")

	return cmd
}
