package main

import (
	"fmt"
	"os"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/auth"
	"github.com/josh-wong/tegata/internal/clipboard"
	"github.com/josh-wong/tegata/internal/config"
	"github.com/josh-wong/tegata/internal/errors"
	"github.com/josh-wong/tegata/internal/i18n"
	"github.com/spf13/cobra"
)

func newSignCmd() *cobra.Command {
	var (
		challenge string
		clip      bool
	)

	cmd := &cobra.Command{
		Use:     "sign <label>",
		Short:   i18n.T("cmd.sign.short"),
		Args:    cobra.ExactArgs(1),
		Example: i18n.T("cmd.sign.example"),
		RunE: func(cmd *cobra.Command, args []string) error {
			label := args[0]

			if challenge == "" {
				return fmt.Errorf("%s",
					i18n.Tf("cmd.sign.error.noChallenge", map[string]any{"Err": errors.ErrInvalidInput}))
			}

			vaultPath, err := resolveVaultPath(cmd)
			if err != nil {
				return err
			}

			passphrase, err := promptPassphrase(i18n.T("cmd.prompt.passphrase"))
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

			if builder != nil {
				if logErr := builder.LogEvent("challenge-response", cred.Label, cred.Issuer, audit.Hostname(), true); logErr != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s",
						i18n.Tf("cmd.sign.warn.auditFailed", map[string]any{"Err": logErr}))
				}
			}

			if clip {
				cm := clipboard.NewManager()
				defer cm.Close()
				if copyErr := cm.CopyWithAutoClear(hexResult, cfg.ClipboardTimeout); copyErr != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s",
						i18n.Tf("cmd.sign.warn.clipboardFailed", map[string]any{"Err": copyErr}))
				} else {
					_, _ = fmt.Fprintf(os.Stderr, "%s",
						i18n.Tf("cmd.sign.copied",
							map[string]any{"Secs": int(cfg.ClipboardTimeout.Seconds())}))
				}
				return nil
			}

			fmt.Println(hexResult)
			return nil
		},
	}

	cmd.Flags().StringVar(&challenge, "challenge", "", i18n.T("cmd.sign.flag.challenge"))
	cmd.Flags().BoolVar(&clip, "clip", false, i18n.T("cmd.sign.flag.clip"))

	return cmd
}
