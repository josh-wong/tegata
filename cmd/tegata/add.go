package main

import (
	"fmt"
	"strings"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/auth"
	"github.com/josh-wong/tegata/internal/errors"
	"github.com/josh-wong/tegata/internal/i18n"
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
		Use:     "add <label>",
		Short:   i18n.T("cmd.add.short"),
		Args:    cobra.ExactArgs(1),
		Example: i18n.T("cmd.add.example"),
		RunE: func(cmd *cobra.Command, args []string) error {
			label := args[0]

			if digits < 1 || digits > 10 {
				return fmt.Errorf("%s: %w", i18n.T("cmd.add.error.digits"), errors.ErrInvalidInput)
			}
			if period < 15 || period > 120 {
				return fmt.Errorf("%s: %w", i18n.T("cmd.add.error.period"), errors.ErrInvalidInput)
			}

			vaultPath, err := resolveVaultPath(cmd)
			if err != nil {
				return err
			}

			passphrase, err := promptPassphrase(i18n.T("cmd.add.prompt.secret"))
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

			var cred pkgmodel.Credential

			if scan {
				uri, promptErr := promptSecret(i18n.T("cmd.add.prompt.uri"))
				if promptErr != nil {
					return promptErr
				}
				parsed, parseErr := auth.ParseOTPAuthURI(strings.TrimSpace(uri))
				if parseErr != nil {
					return fmt.Errorf("%s", i18n.Tf("cmd.add.error.parseURI", map[string]any{"Err": parseErr}))
				}
				cred = *parsed
				cred.Label = label
				cred.Tags = tags
			} else {
				ct := pkgmodel.CredentialType(credType)
				switch ct {
				case pkgmodel.CredentialTOTP, pkgmodel.CredentialHOTP, pkgmodel.CredentialStatic,
					pkgmodel.CredentialChallengeResponse:
				default:
					return fmt.Errorf("%s", i18n.Tf("cmd.add.error.invalidType",
						map[string]any{"Type": credType, "Err": errors.ErrInvalidInput}))
				}

				requestedAlgorithm := algorithm
				algorithm = resolveAlgorithm(ct, cmd.Flags().Changed("algorithm"), algorithm)
				if ct == pkgmodel.CredentialHOTP && cmd.Flags().Changed("algorithm") && !strings.EqualFold(requestedAlgorithm, "SHA1") {
					return fmt.Errorf("%s", i18n.Tf("cmd.add.error.hotpAlgorithm", map[string]any{"Err": errors.ErrInvalidInput}))
				}

				var secretPrompt string
				switch ct {
				case pkgmodel.CredentialStatic:
					secretPrompt = i18n.T("cmd.add.prompt.password")
				case pkgmodel.CredentialChallengeResponse:
					secretPrompt = i18n.T("cmd.add.prompt.sharedSecret")
				default:
					secretPrompt = i18n.T("cmd.add.prompt.secret")
				}

				secret, promptErr := promptSecret(secretPrompt)
				if promptErr != nil {
					return promptErr
				}

				trimmedSecret := strings.TrimSpace(secret)

				switch ct {
				case pkgmodel.CredentialTOTP, pkgmodel.CredentialHOTP:
					if _, decErr := decodeBase32Secret(trimmedSecret); decErr != nil {
						return fmt.Errorf("%s", i18n.Tf("cmd.add.error.invalidBase32", map[string]any{"Err": errors.ErrInvalidInput}))
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
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s",
						i18n.Tf("cmd.add.warn.auditFailed", map[string]any{"Err": logErr}))
				}
			}

			displayIssuer := cred.Issuer
			if displayIssuer == "" {
				displayIssuer = i18n.T("cmd.add.noIssuer")
			}
			fmt.Print(i18n.Tf("cmd.add.success", map[string]any{
				"Type":   cred.Type,
				"Label":  cred.Label,
				"Issuer": displayIssuer,
			}))

			return nil
		},
	}

	cmd.Flags().BoolVar(&scan, "scan", false, i18n.T("cmd.add.flag.scan"))
	cmd.Flags().StringVar(&credType, "type", "totp", i18n.T("cmd.add.flag.type"))
	cmd.Flags().StringVar(&issuer, "issuer", "", i18n.T("cmd.add.flag.issuer"))
	cmd.Flags().StringVar(&algorithm, "algorithm", "SHA1", i18n.T("cmd.add.flag.algorithm"))
	cmd.Flags().IntVar(&digits, "digits", 6, i18n.T("cmd.add.flag.digits"))
	cmd.Flags().IntVar(&period, "period", 30, i18n.T("cmd.add.flag.period"))
	cmd.Flags().StringArrayVar(&tags, "tag", nil, i18n.T("cmd.add.flag.tag"))

	return cmd
}

// resolveAlgorithm returns the effective algorithm for a new credential.
// Challenge-response defaults to SHA256 when the user has not explicitly set
// --algorithm. HOTP is always SHA1 per RFC 4226. All other types use
// flagValue (flag default is SHA1 per RFC 6238/4226). flagChanged must be the
// result of cmd.Flags().Changed("algorithm").
func resolveAlgorithm(ct pkgmodel.CredentialType, flagChanged bool, flagValue string) string {
	if ct == pkgmodel.CredentialHOTP {
		return "SHA1"
	}
	if ct == pkgmodel.CredentialChallengeResponse && !flagChanged {
		return "SHA256"
	}
	return flagValue
}
