package main

import (
	"fmt"
	"strings"

	"github.com/josh-wong/tegata/internal/auth"
	"github.com/josh-wong/tegata/internal/errors"
	"github.com/josh-wong/tegata/pkg/model"
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

			if digits < 1 || digits > 10 {
				return fmt.Errorf("--digits must be between 1 and 10: %w", errors.ErrInvalidInput)
			}
			if period < 1 {
				return fmt.Errorf("--period must be at least 1 second: %w", errors.ErrInvalidInput)
			}

			var cred model.Credential

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
				ct := model.CredentialType(credType)
				switch ct {
				case model.CredentialTOTP, model.CredentialHOTP, model.CredentialStatic,
					model.CredentialChallengeResponse:
				default:
					return fmt.Errorf("invalid credential type %q (use totp, hotp, static, or challenge-response): %w",
						credType, errors.ErrInvalidInput)
				}

				secret, promptErr := promptSecret("Secret: ")
				if promptErr != nil {
					return promptErr
				}

				cred = model.Credential{
					Label:     label,
					Issuer:    issuer,
					Type:      ct,
					Algorithm: algorithm,
					Digits:    digits,
					Period:    period,
					Secret:    strings.TrimSpace(secret),
					Tags:      tags,
				}
			}

			id, err := mgr.AddCredential(cred)
			if err != nil {
				return err
			}
			_ = id

			displayIssuer := cred.Issuer
			if displayIssuer == "" {
				displayIssuer = "--"
			}
			fmt.Printf("Added %s credential: %s (%s)\n", cred.Type, cred.Label, displayIssuer)

			return nil
		},
	}

	cmd.Flags().BoolVar(&scan, "scan", false, "paste an otpauth:// URI")
	cmd.Flags().StringVar(&credType, "type", "totp", "credential type (totp, hotp, static)")
	cmd.Flags().StringVar(&issuer, "issuer", "", "credential issuer")
	cmd.Flags().StringVar(&algorithm, "algorithm", "SHA1", "HMAC algorithm (SHA1, SHA256, SHA512)")
	cmd.Flags().IntVar(&digits, "digits", 6, "number of digits in generated code")
	cmd.Flags().IntVar(&period, "period", 30, "TOTP period in seconds")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "tag to apply (repeatable, e.g. --tag work --tag totp)")

	return cmd
}
