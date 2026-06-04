package main

import (
	"fmt"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/auth"
	"github.com/josh-wong/tegata/internal/clipboard"
	"github.com/josh-wong/tegata/internal/config"
	"github.com/josh-wong/tegata/internal/i18n"
	"github.com/spf13/cobra"
)

func newGetCmd() *cobra.Command {
	var show bool

	cmd := &cobra.Command{
		Use:     "get <label>",
		Short:   i18n.T("cmd.get.short"),
		Args:    cobra.ExactArgs(1),
		Example: i18n.T("cmd.get.example"),
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

			password, err := auth.GetStaticPassword(cred)
			if err != nil {
				return err
			}
			defer zeroBytes(password)

			if builder != nil {
				if logErr := builder.LogEvent("static", cred.Label, cred.Issuer, audit.Hostname(), true); logErr != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s",
						i18n.Tf("cmd.get.warn.auditFailed", map[string]any{"Err": logErr}))
				}
			}

			cm := clipboard.NewManager()
			defer cm.Close()
			if err := cm.CopyWithAutoClear(string(password), cfg.ClipboardTimeout); err != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s",
					i18n.Tf("cmd.get.warn.clipboardFailed", map[string]any{"Err": err}))
			} else {
				fmt.Print(i18n.Tf("cmd.get.copied",
					map[string]any{"Secs": int(cfg.ClipboardTimeout.Seconds())}))
			}

			if show {
				fmt.Print(i18n.Tf("cmd.get.password", map[string]any{"Password": string(password)}))
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&show, "show", false, i18n.T("cmd.get.flag.show"))

	return cmd
}
