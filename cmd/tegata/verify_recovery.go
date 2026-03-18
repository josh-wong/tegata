package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVerifyRecoveryCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "verify-recovery",
		Short:   "Verify a recovery key against the vault",
		Args:    cobra.NoArgs,
		Example: `  tegata verify-recovery`,
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

			// Recovery keys are displayed in base32 (with dashes) at vault creation.
			// Prompt for the recovery key string, decode it, and verify the hash.
			recoveryKeyStr, err := promptSecret("Recovery key: ")
			if err != nil {
				return err
			}

			rawKey, err := decodeBase32Secret(recoveryKeyStr)
			if err != nil {
				return fmt.Errorf("decoding recovery key: %w", err)
			}
			defer zeroBytes(rawKey)

			ok, err := mgr.VerifyRecoveryKey(rawKey)
			if err != nil {
				return err
			}

			if ok {
				fmt.Println("Recovery key: VALID")
			} else {
				fmt.Println("Recovery key: INVALID")
			}
			return nil
		},
	}
}
