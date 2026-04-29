package main

import (
	"fmt"
	"os"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/auth"
	"github.com/josh-wong/tegata/internal/clipboard"
	"github.com/josh-wong/tegata/internal/config"
	"github.com/josh-wong/tegata/internal/errors"
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
				return fmt.Errorf("--challenge is required: %w", errors.ErrInvalidInput)
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

			// Load config for clipboard settings; build audit builder while passphrase is still in scope.
			cfg, _ := config.Load(vaultDir(vaultPath))
			builder := setupAuditBuilder(cmd.ErrOrStderr(), vaultDir(vaultPath), passphrase, mgr)
			if builder != nil {
				defer func() { _ = builder.Close() }()
			}

			cred, err := mgr.GetCredential(label)
			if err != nil {
				return err
			}

			// Try base32 first (standard for OTP secrets), fall back to raw
			// bytes for challenge-response since users may store plain text keys.
			secretBytes, err := decodeBase32Secret(cred.Secret)
			if err != nil {
				secretBytes = []byte(cred.Secret)
			}
			defer zeroBytes(secretBytes)

			hexResult, err := auth.SignChallenge(cred, secretBytes, []byte(challenge))
			if err != nil {
				return err
			}

			// Emit audit event after successful sign.
			if builder != nil {
				if logErr := builder.LogEvent("challenge-response", cred.Label, cred.Issuer, audit.Hostname(), true); logErr != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Audit log failed: %v\n", logErr)
				}
			}

			if clip {
				cm := clipboard.NewManager()
				defer cm.Close()
				if copyErr := cm.CopyWithAutoClear(hexResult, cfg.ClipboardTimeout); copyErr != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Clipboard copy failed: %v\n", copyErr)
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
