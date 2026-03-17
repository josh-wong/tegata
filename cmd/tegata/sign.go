package main

import (
	"fmt"
	"os"

	"github.com/josh-wong/tegata/internal/auth"
	"github.com/josh-wong/tegata/internal/clipboard"
	"github.com/josh-wong/tegata/internal/config"
	"github.com/spf13/cobra"
)

func newSignCmd() *cobra.Command {
	var (
		challenge string
		clip      bool
	)

	cmd := &cobra.Command{
		Use:   "sign <label>",
		Short: "Sign a challenge string with a stored HMAC secret",
		Args:  cobra.ExactArgs(1),
		Example: `  tegata sign github --challenge abc123
  tegata sign github --challenge abc123 --clip`,
		RunE: func(cmd *cobra.Command, args []string) error {
			label := args[0]

			if challenge == "" {
				return fmt.Errorf("--challenge is required: %w",
					fmt.Errorf("provide a challenge string with --challenge"))
			}

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

			cred, err := mgr.GetCredential(label)
			if err != nil {
				return err
			}

			// Decode the base32-encoded secret stored in the credential.
			secretBytes, err := decodeBase32Secret(cred.Secret)
			if err != nil {
				return fmt.Errorf("decoding credential secret for %q: %w", label, err)
			}
			defer zeroBytes(secretBytes)

			hexResult, err := auth.SignChallenge(cred, secretBytes, []byte(challenge))
			if err != nil {
				return err
			}

			if clip {
				// Load config for clipboard timeout.
				cfg, _ := config.Load(vaultDir(vaultPath))
				cm := clipboard.NewManager()
				defer cm.Close()
				if copyErr := cm.CopyWithAutoClear(hexResult, cfg.ClipboardTimeout); copyErr != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: clipboard copy failed: %v\n", copyErr)
				} else {
					_, _ = fmt.Fprintf(os.Stderr, "Copied to clipboard (auto-clear in %ds)\n",
						int(cfg.ClipboardTimeout.Seconds()))
				}
				return nil
			}

			fmt.Println(hexResult)
			return nil
		},
	}

	cmd.Flags().StringVar(&challenge, "challenge", "", "challenge string to sign (required)")
	cmd.Flags().BoolVar(&clip, "clip", false, "copy the signed response to clipboard instead of printing")

	return cmd
}
