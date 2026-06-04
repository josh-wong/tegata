package main

import (
	"fmt"
	"strings"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/errors"
	"github.com/josh-wong/tegata/internal/i18n"
	"github.com/spf13/cobra"
)

func newTagCmd() *cobra.Command {
	var (
		addTags    []string
		removeTags []string
	)

	cmd := &cobra.Command{
		Use:     "tag <label>",
		Short:   i18n.T("cmd.tag.short"),
		Args:    cobra.ExactArgs(1),
		Example: i18n.T("cmd.tag.example"),
		RunE: func(cmd *cobra.Command, args []string) error {
			label := args[0]

			if len(addTags) == 0 && len(removeTags) == 0 {
				return fmt.Errorf("%s",
					i18n.Tf("cmd.tag.error.noFlags", map[string]any{"Err": errors.ErrInvalidInput}))
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

			builder := setupAuditBuilder(cmd.ErrOrStderr(), vaultDir(vaultPath), passphrase, mgr)
			if builder != nil {
				defer func() { _ = builder.Close() }()
			}

			cred, err := mgr.GetCredential(label)
			if err != nil {
				return err
			}

			var normalizedAdd []string
			for _, t := range addTags {
				normalizedAdd = append(normalizedAdd, strings.ToLower(t))
			}
			var normalizedRemove []string
			for _, t := range removeTags {
				normalizedRemove = append(normalizedRemove, strings.ToLower(t))
			}

			tagSet := make(map[string]struct{}, len(cred.Tags))
			for _, t := range cred.Tags {
				tagSet[t] = struct{}{}
			}

			for _, t := range normalizedAdd {
				if _, exists := tagSet[t]; !exists {
					cred.Tags = append(cred.Tags, t)
					tagSet[t] = struct{}{}
				}
			}

			removeSet := make(map[string]struct{}, len(normalizedRemove))
			for _, t := range normalizedRemove {
				removeSet[t] = struct{}{}
			}
			filtered := cred.Tags[:0]
			for _, t := range cred.Tags {
				if _, remove := removeSet[t]; !remove {
					filtered = append(filtered, t)
				}
			}
			cred.Tags = filtered

			if err := mgr.UpdateCredential(cred); err != nil {
				return err
			}

			if builder != nil {
				if logErr := builder.LogEvent("credential-tag-update", cred.Label, cred.Issuer, audit.Hostname(), true); logErr != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s",
						i18n.Tf("cmd.tag.warn.auditFailed", map[string]any{"Err": logErr}))
				}
			}

			if len(cred.Tags) == 0 {
				fmt.Print(i18n.Tf("cmd.tag.tagsNone", map[string]any{"Label": cred.Label}))
			} else {
				fmt.Print(i18n.Tf("cmd.tag.tags",
					map[string]any{"Label": cred.Label, "Tags": strings.Join(cred.Tags, ", ")}))
			}
			return nil
		},
	}

	cmd.Flags().StringArrayVar(&addTags, "add", nil, i18n.T("cmd.tag.flag.add"))
	cmd.Flags().StringArrayVar(&removeTags, "remove", nil, i18n.T("cmd.tag.flag.remove"))

	return cmd
}
