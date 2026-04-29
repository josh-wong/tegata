package main

import (
	"fmt"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/config"
	"github.com/spf13/cobra"
)

func newRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "remove <label>",
		Short:   "Remove a credential from the vault",
		Args:    cobra.ExactArgs(1),
		Example: `  tegata remove old-service`,
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
				if logErr := builder.LogEvent("vault-unlock", "", "", audit.Hostname(), true); logErr != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: audit log failed: %v\n", logErr)
				}
			}

			cred, err := mgr.GetCredential(label)
			if err != nil {
				return err
			}

			prompt := fmt.Sprintf("Remove %q (%s)? This cannot be undone. [y/N]: ", label, cred.Type)
			if !promptConfirmation(prompt) {
				fmt.Println("Canceled.")
				return nil
			}

			if err := mgr.RemoveCredential(cred.ID); err != nil {
				return err
			}

			if builder != nil {
				if logErr := builder.LogEvent("credential-remove", cred.Label, cred.Issuer, audit.Hostname(), true); logErr != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: audit log failed: %v\n", logErr)
				}
			}

			fmt.Printf("Removed: %s\n", label)
			return nil
		},
	}
}
