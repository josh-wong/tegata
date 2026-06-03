package main

import (
	"fmt"
	"time"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/auth"
	"github.com/josh-wong/tegata/internal/clipboard"
	"github.com/josh-wong/tegata/internal/config"
	"github.com/josh-wong/tegata/internal/errors"
	"github.com/josh-wong/tegata/internal/i18n"
	pkgmodel "github.com/josh-wong/tegata/pkg/model"
	"github.com/spf13/cobra"
)

func newCodeCmd() *cobra.Command {
	var (
		clip bool
		show bool
	)

	cmd := &cobra.Command{
		Use:     "code <label>",
		Short:   i18n.T("cmd.code.short"),
		Args:    cobra.ExactArgs(1),
		Example: i18n.T("cmd.code.example"),
		RunE: func(cmd *cobra.Command, args []string) error {
			label := args[0]

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

			if cred.Type != pkgmodel.CredentialTOTP && cred.Type != pkgmodel.CredentialHOTP {
				return fmt.Errorf("%s: %w", i18n.Tf("cmd.code.error.wrongType",
					map[string]any{"Label": label, "Type": cred.Type}), errors.ErrInvalidInput)
			}

			secret, err := decodeBase32Secret(cred.Secret)
			if err != nil {
				return fmt.Errorf("%s: %w", i18n.Tf("cmd.code.error.decodeSecret",
					map[string]any{"Label": label}), err)
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
				fmt.Print(i18n.Tf("cmd.code.expires", map[string]any{"Secs": remaining}))

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

			if builder != nil {
				opType := string(cred.Type)
				if logErr := builder.LogEvent(opType, cred.Label, cred.Issuer, audit.Hostname(), true); logErr != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s",
						i18n.Tf("cmd.code.warn.auditFailed", map[string]any{"Err": logErr}))
				}
			}

			if clip {
				cm := clipboard.NewManager()
				defer cm.Close()
				if err := cm.CopyWithAutoClear(code, cfg.ClipboardTimeout); err != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s",
						i18n.Tf("cmd.code.warn.clipboard", map[string]any{"Err": err}))
				} else {
					fmt.Print(i18n.Tf("cmd.code.copied",
						map[string]any{"Secs": int(cfg.ClipboardTimeout.Seconds())}))
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&clip, "clip", true, i18n.T("cmd.code.flag.noClip"))
	cmd.Flags().BoolVar(&show, "show", true, i18n.T("cmd.code.flag.show"))

	return cmd
}
