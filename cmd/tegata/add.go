package main

import (
	"fmt"
	"strings"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/auth"
	"github.com/josh-wong/tegata/internal/config"
	"github.com/josh-wong/tegata/internal/errors"
	pkgmodel "github.com/josh-wong/tegata/pkg/model"
	"github.com/spf13/cobra"
)

func newAddCmd() *cobra.Command {
	var (
		scan      bool
		credType  string
		issuer    string
		algorithm string
		digits    int
		period    int
		tags      []string
	)

	cmd := &cobra.Command{
		Use:   "add <label>",
		Short: "Add a credential to the vault",
		Args:  cobra.ExactArgs(1),
		Example: `  tegata add GitHub --scan
  tegata add GitHub --type totp --issuer GitHub`,
		RunE: func(cmd *cobra.Command, args []string) error {
			label := args[0]

			if digits < 1 || digits > 10 {
				return fmt.Errorf("--digits must be between 1 and 10: %w", errors.ErrInvalidInput)
			}
			if period < 15 || period > 120 {
				return fmt.Errorf("--period must be between 15 and 120 seconds: %w", errors.ErrInvalidInput)
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

			cfg, _ := config.Load(vaultDir(vaultPath))
			builder, err := newEventBuilder(cfg, vaultDir(vaultPath), passphrase)
			if err != nil {
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

			var cred pkgmodel.Credential

			if scan {
				// Prompt for otpauth:// URI.
				uri, promptErr := promptSecret("Paste otpauth:// URI: ")
				if promptErr != nil {
					return promptErr
				}
				parsed, parseErr := auth.ParseOTPAuthURI(strings.TrimSpace(uri))
				if parseErr != nil {
					return fmt.Errorf("parsing URI: %w", parseErr)
				}
				cred = *parsed
				cred.Label = label
				cred.Tags = tags
			} else {
				// Validate type.
				ct := pkgmodel.CredentialType(credType)
				switch ct {
				case pkgmodel.CredentialTOTP, pkgmodel.CredentialHOTP, pkgmodel.CredentialStatic,
					pkgmodel.CredentialChallengeResponse:
				default:
					return fmt.Errorf("invalid credential type %q (use totp, hotp, static, or challenge-response): %w",
						credType, errors.ErrInvalidInput)
				}

				var secretPrompt string
				switch ct {
				case pkgmodel.CredentialStatic:
					secretPrompt = "Password: "
				case pkgmodel.CredentialChallengeResponse:
					secretPrompt = "Shared secret key: "
				default:
					secretPrompt = "Secret: "
				}

				secret, promptErr := promptSecret(secretPrompt)
				if promptErr != nil {
					return promptErr
				}

				trimmedSecret := strings.TrimSpace(secret)

				switch ct {
				case pkgmodel.CredentialTOTP, pkgmodel.CredentialHOTP:
					if _, decErr := decodeBase32Secret(trimmedSecret); decErr != nil {
						return fmt.Errorf("secret is not valid base32 — TOTP and HOTP secrets use characters A-Z and 2-7 only: %w", errors.ErrInvalidInput)
					}
				}

				cred = pkgmodel.Credential{
					Label:     label,
					Issuer:    issuer,
					Type:      ct,
					Algorithm: algorithm,
					Digits:    digits,
					Period:    period,
					Secret:    trimmedSecret,
					Tags:      tags,
				}
			}

			id, err := mgr.AddCredential(cred)
			if err != nil {
				return err
			}
			_ = id

			if builder != nil {
				if logErr := builder.LogEvent("credential-add", cred.Label, cred.Issuer, audit.Hostname(), true); logErr != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Audit log failed: %v\n", logErr)
				}
			}

			displayIssuer := cred.Issuer
			if displayIssuer == "" {
				displayIssuer = "--"
			}
			fmt.Printf("Added %s credential: %s (%s)\n", cred.Type, cred.Label, displayIssuer)

			return nil
		},
	}

	cmd.Flags().BoolVar(&scan, "scan", false, "paste an otpauth:// URI")
	cmd.Flags().StringVar(&credType, "type", "totp", "credential type (totp, hotp, static, challenge-response)")
	cmd.Flags().StringVar(&issuer, "issuer", "", "credential issuer")
	cmd.Flags().StringVar(&algorithm, "algorithm", "SHA1", "HMAC algorithm (SHA1, SHA256, SHA512)")
	cmd.Flags().IntVar(&digits, "digits", 6, "number of digits in generated code")
	cmd.Flags().IntVar(&period, "period", 30, "TOTP period in seconds")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "tag to apply (repeatable, e.g. --tag work --tag totp)")

	return cmd
}
