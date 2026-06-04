package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/errors"
	"github.com/josh-wong/tegata/internal/i18n"
	"github.com/spf13/cobra"
)

func newEditCmd() *cobra.Command {
	var (
		label    string
		issuer   string
		tags     string
		category string
	)

	cmd := &cobra.Command{
		Use:     "edit <label>",
		Short:   i18n.T("cmd.edit.short"),
		Args:    cobra.ExactArgs(1),
		Example: i18n.T("cmd.edit.example"),
		RunE: func(cmd *cobra.Command, args []string) error {
			credLabel := args[0]

			if label == "" && issuer == "" && tags == "" && category == "" {
				return fmt.Errorf("%s",
					i18n.Tf("cmd.edit.error.noFlags", map[string]any{"Err": errors.ErrInvalidInput}))
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

			cred, err := mgr.GetCredential(credLabel)
			if err != nil {
				return err
			}

			origLabel := cred.Label
			origIssuer := cred.Issuer
			origTags := slices.Clone(cred.Tags)
			origCategory := cred.Category

			if label != "" {
				label = strings.TrimSpace(label)
				if label == "" {
					return fmt.Errorf("%s",
						i18n.Tf("cmd.edit.error.emptyLabel", map[string]any{"Err": errors.ErrInvalidInput}))
				}
				for _, c := range mgr.ListCredentials() {
					if strings.EqualFold(c.Label, label) && c.ID != cred.ID {
						return fmt.Errorf("%s",
							i18n.Tf("cmd.edit.error.duplicateLabel",
								map[string]any{"Label": label, "Err": errors.ErrInvalidInput}))
					}
				}
				cred.Label = label
			}

			cred.Issuer = issuer

			if tags != "" {
				var newTags []string
				for _, t := range strings.Split(tags, ",") {
					if t = strings.TrimSpace(t); t != "" {
						newTags = append(newTags, strings.ToLower(t))
					}
				}

				seen := make(map[string]struct{})
				for _, t := range newTags {
					if _, exists := seen[t]; exists {
						return fmt.Errorf("%s",
							i18n.Tf("cmd.edit.error.duplicateTag",
								map[string]any{"Tag": t, "Err": errors.ErrInvalidInput}))
					}
					seen[t] = struct{}{}
				}

				cred.Tags = newTags
			}

			if cmd.Flags().Changed("category") {
				cred.Category = strings.ToLower(strings.TrimSpace(category))
			}

			if err := mgr.UpdateCredential(cred); err != nil {
				return err
			}

			if builder != nil {
				type fieldEvent struct {
					changed bool
					opType  string
				}
				events := []fieldEvent{
					{cred.Label != origLabel, "credential-label-update"},
					{cred.Issuer != origIssuer, "credential-issuer-update"},
					{cred.Category != origCategory, "credential-category-update"},
					{!slices.Equal(origTags, cred.Tags), "credential-tag-update"},
				}
				for _, fe := range events {
					if fe.changed {
						if logErr := builder.LogEvent(fe.opType, cred.Label, cred.Issuer, audit.Hostname(), true); logErr != nil {
							_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s",
								i18n.Tf("cmd.edit.warn.auditFailed", map[string]any{"Err": logErr}))
						}
					}
				}
			}

			fmt.Print(i18n.Tf("cmd.edit.success", map[string]any{"Label": cred.Label}))
			if cred.Category != "" {
				fmt.Print(i18n.Tf("cmd.edit.category", map[string]any{"Category": cred.Category}))
			}
			if len(cred.Tags) == 0 {
				fmt.Print(i18n.T("cmd.edit.tagsNone"))
			} else {
				fmt.Print(i18n.Tf("cmd.edit.tags", map[string]any{"Tags": strings.Join(cred.Tags, ", ")}))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&label, "label", "", i18n.T("cmd.edit.flag.label"))
	cmd.Flags().StringVar(&issuer, "issuer", "", i18n.T("cmd.edit.flag.issuer"))
	cmd.Flags().StringVar(&tags, "tags", "", i18n.T("cmd.edit.flag.tags"))
	cmd.Flags().StringVar(&category, "category", "", i18n.T("cmd.edit.flag.category"))

	return cmd
}
