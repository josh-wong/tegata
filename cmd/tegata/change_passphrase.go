package main

import (
	"fmt"

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

			newPassphrase, err := promptNewPassphrase()
			if err != nil {
				return err
			}
			defer zeroBytes(newPassphrase)

			if err := mgr.ChangePassphrase(newPassphrase); err != nil {
				return fmt.Errorf("changing passphrase: %w", err)
			}

			fmt.Println("Passphrase changed successfully.")
			return nil
		},
	}
}
