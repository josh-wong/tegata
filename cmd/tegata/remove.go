package main

import (
	"fmt"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/i18n"
	"github.com/spf13/cobra"
)

func newRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "remove <label>",
		Short:   i18n.T("cmd.remove.short"),
		Args:    cobra.ExactArgs(1),
		Example: i18n.T("cmd.remove.example"),
		RunE: func(cmd *cobra.Command, args []string) error {
			label := args[0]

			vaultPath, err := resolveVaultPath(cmd)
			if err != nil {
				return err
			}

			passphrase, err := promptPassphrase(i18n.T("cmd.verifyRecovery.prompt.passphrase"))
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

			prompt := i18n.Tf("cmd.remove.prompt.confirm",
				map[string]any{"Label": label, "Type": cred.Type})
			if !promptConfirmation(prompt) {
				fmt.Println(i18n.T("cmd.remove.canceled"))
				return nil
			}

			if err := mgr.RemoveCredential(cred.ID); err != nil {
				return err
			}

			if builder != nil {
				if logErr := builder.LogEvent("credential-remove", cred.Label, cred.Issuer, audit.Hostname(), true); logErr != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s",
						i18n.Tf("cmd.remove.warn.auditFailed", map[string]any{"Err": logErr}))
				}
			}

			fmt.Print(i18n.Tf("cmd.remove.success", map[string]any{"Label": label}))
			return nil
		},
	}
}
