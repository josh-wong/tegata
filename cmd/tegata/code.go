package main

import (
	"fmt"
	"time"

	"github.com/josh-wong/tegata/internal/auth"
	"github.com/josh-wong/tegata/internal/clipboard"
	"github.com/josh-wong/tegata/internal/config"
	"github.com/josh-wong/tegata/internal/errors"
	"github.com/spf13/cobra"
)

func newCodeCmd() *cobra.Command {
	var (
		clip bool
		show bool
	)

	cmd := &cobra.Command{
		Use:   "code <label>",
		Short: "Generate a TOTP or HOTP code",
		Args:  cobra.ExactArgs(1),
		Example: `  tegata code GitHub
  tegata code --no-clip GitHub`,
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

			cred, err := mgr.GetCredential(label)
			if err != nil {
				return err
			}

			if cred.Type != "totp" && cred.Type != "hotp" {
				return fmt.Errorf("credential %q is type %s, expected totp or hotp: %w",
					label, cred.Type, errors.ErrInvalidInput)
			}

			// Decode the base32 secret.
			secret, err := decodeBase32Secret(cred.Secret)
			if err != nil {
				return fmt.Errorf("decoding secret for %q: %w", label, err)
			}
			defer zeroBytes(secret)

			// Load config for clipboard timeout.
			cfg, _ := config.Load(vaultDir(vaultPath))

			var code string

			switch cred.Type {
			case "totp":
				var remaining int
				code, remaining = auth.GenerateTOTP(secret, time.Now(), cred.Period, cred.Digits, cred.Algorithm)
				if show {
					fmt.Println(code)
				}
				fmt.Printf("Expires in %ds\n", remaining)

			case "hotp":
				// Counter-before-code: save the incremented counter BEFORE
				// displaying the code to prevent counter desync on crash.
				code = auth.GenerateHOTP(secret, cred.Counter, cred.Digits, cred.Algorithm)
				cred.Counter++
				if err := mgr.UpdateCredential(cred); err != nil {
					return fmt.Errorf("saving counter: %w", err)
				}
				if show {
					fmt.Println(code)
				}
			}

			if clip {
				cm := clipboard.NewManager()
				defer cm.Close()
				if err := cm.CopyWithAutoClear(code, cfg.ClipboardTimeout); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %v\n", err)
				} else {
					fmt.Printf("Copied to clipboard (auto-clear in %ds)\n",
						int(cfg.ClipboardTimeout.Seconds()))
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&clip, "clip", true, "copy code to clipboard")
	cmd.Flags().BoolVar(&show, "show", true, "display code in terminal")

	return cmd
}
