package main

import (
	"fmt"
	"time"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/auth"
	"github.com/josh-wong/tegata/internal/clipboard"
	"github.com/josh-wong/tegata/internal/config"
	"github.com/josh-wong/tegata/internal/errors"
	pkgmodel "github.com/josh-wong/tegata/pkg/model"
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

			// Load config before zeroing passphrase; build audit builder while
			// passphrase is still available.
			cfg, _ := config.Load(vaultDir(vaultPath))
			builder, err := newEventBuilder(cfg, vaultDir(vaultPath), passphrase)
			if err != nil {
				// Non-fatal: log and continue without audit.
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Audit unavailable: %v\n", err)
			}
			if builder != nil {
				defer func() { _ = builder.Close() }()
				builder.OnHashStored = func(eventID, hashValue string) {
					if err := mgr.SetAuditHash(eventID, hashValue); err != nil {
						_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to store audit hash: %v\n", err)
					}
				}
				if logErr := builder.LogEvent("vault-unlock", "", "", audit.Hostname(), true); logErr != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Audit log failed: %v\n", logErr)
				}
			}

			cred, err := mgr.GetCredential(label)
			if err != nil {
				return err
			}

			if cred.Type != pkgmodel.CredentialTOTP && cred.Type != pkgmodel.CredentialHOTP {
				return fmt.Errorf("credential %q is type %s, expected totp or hotp: %w",
					label, cred.Type, errors.ErrInvalidInput)
			}

			// Decode the base32 secret.
			secret, err := decodeBase32Secret(cred.Secret)
			if err != nil {
				return fmt.Errorf("decoding secret for %q: %w", label, err)
			}
			defer zeroBytes(secret)

			var code string

			switch cred.Type {
			case pkgmodel.CredentialTOTP:
				var remaining int
				code, remaining = auth.GenerateTOTP(secret, time.Now(), cred.Period, cred.Digits, cred.Algorithm)
				if show {
					fmt.Println(code)
				}
				fmt.Printf("Expires in %ds\n", remaining)

			case pkgmodel.CredentialHOTP:
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

			// Emit audit event (non-fatal: errors are logged, not propagated).
			if builder != nil {
				opType := string(cred.Type)
				if logErr := builder.LogEvent(opType, cred.Label, cred.Issuer, audit.Hostname(), true); logErr != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Audit log failed: %v\n", logErr)
				}
			}

			if clip {
				cm := clipboard.NewManager()
				defer cm.Close()
				if err := cm.CopyWithAutoClear(code, cfg.ClipboardTimeout); err != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %v\n", err)
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
