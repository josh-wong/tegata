package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/auth"
	"github.com/josh-wong/tegata/internal/errors"
	"github.com/josh-wong/tegata/internal/i18n"
	"github.com/spf13/cobra"
)

func newResyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "resync <label>",
		Short:   i18n.T("cmd.resync.short"),
		Args:    cobra.ExactArgs(1),
		Example: i18n.T("cmd.resync.example"),
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

			builder := setupAuditBuilder(cmd.ErrOrStderr(), vaultDir(vaultPath), passphrase, mgr)
			if builder != nil {
				defer func() { _ = builder.Close() }()
			}

			cred, err := mgr.GetCredential(label)
			if err != nil {
				return err
			}

			if cred.Type != "hotp" {
				return fmt.Errorf("%s: %w",
					i18n.Tf("cmd.resync.error.wrongType",
						map[string]any{"Label": label, "Type": cred.Type}), errors.ErrInvalidInput)
			}

			scanner := bufio.NewScanner(os.Stdin)

			fmt.Fprint(os.Stderr, i18n.T("cmd.resync.prompt.firstCode"))
			if !scanner.Scan() {
				return fmt.Errorf("%s: %w",
					i18n.T("cmd.resync.error.readFirstCode"), errors.ErrInvalidInput)
			}
			code1 := strings.TrimSpace(scanner.Text())

			fmt.Fprint(os.Stderr, i18n.T("cmd.resync.prompt.secondCode"))
			if !scanner.Scan() {
				return fmt.Errorf("%s: %w",
					i18n.T("cmd.resync.error.readSecondCode"), errors.ErrInvalidInput)
			}
			code2 := strings.TrimSpace(scanner.Text())

			secret, err := decodeBase32Secret(cred.Secret)
			if err != nil {
				return fmt.Errorf("%s: %w",
					i18n.Tf("cmd.resync.error.decodeSecret", map[string]any{"Label": label}), err)
			}
			defer zeroBytes(secret)

			newCounter, err := auth.ResyncHOTP(secret, code1, code2, cred.Counter, cred.Digits, cred.Algorithm)
			if err != nil {
				fmt.Println(i18n.T("cmd.resync.error.noMatch"))
				return err
			}

			cred.Counter = newCounter
			if err := mgr.UpdateCredential(cred); err != nil {
				return fmt.Errorf("%s: %w", i18n.T("cmd.resync.error.saveCounter"), err)
			}

			if builder != nil {
				if logErr := builder.LogEvent("hotp-resync", cred.Label, cred.Issuer, audit.Hostname(), true); logErr != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s",
						i18n.Tf("cmd.resync.warn.auditFailed", map[string]any{"Err": logErr}))
				}
			}

			fmt.Print(i18n.Tf("cmd.resync.success", map[string]any{"Counter": newCounter}))
			return nil
		},
	}

	return cmd
}
