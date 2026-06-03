package main

import (
	"fmt"

	"github.com/josh-wong/tegata/internal/i18n"
	"github.com/spf13/cobra"
)

func newVerifyRecoveryCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "verify-recovery",
		Short:   i18n.T("cmd.verifyRecovery.short"),
		Args:    cobra.NoArgs,
		Example: i18n.T("cmd.verifyRecovery.example"),
		RunE: func(cmd *cobra.Command, args []string) error {
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

			recoveryKeyStr, err := promptSecret(i18n.T("cmd.verifyRecovery.prompt.recoveryKey"))
			if err != nil {
				return err
			}

			rawKey, err := decodeBase32Secret(recoveryKeyStr)
			if err != nil {
				return fmt.Errorf("%s",
					i18n.Tf("cmd.verifyRecovery.error.decodeKey", map[string]any{"Err": err}))
			}
			defer zeroBytes(rawKey)

			ok, err := mgr.VerifyRecoveryKey(rawKey)
			if err != nil {
				return err
			}

			if ok {
				fmt.Println(i18n.T("cmd.verifyRecovery.success"))
			} else {
				fmt.Println(i18n.T("cmd.verifyRecovery.failure"))
			}
			return nil
		},
	}
}
