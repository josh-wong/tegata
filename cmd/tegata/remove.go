package main

import (
	"fmt"

	"github.com/josh-wong/tegata/internal/audit"
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

			builder := setupAuditBuilder(cmd.ErrOrStderr(), vaultDir(vaultPath), passphrase, mgr)
			if builder != nil {
				defer func() { _ = builder.Close() }()
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
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Audit log failed: %v\n", logErr)
				}
			}

			fmt.Printf("Removed: %s\n", label)
			return nil
		},
	}
}
