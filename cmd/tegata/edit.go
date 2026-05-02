package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/errors"
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
		Use:   "edit <label>",
		Short: "Edit a credential's metadata (label, issuer, category, tags)",
		Args:  cobra.ExactArgs(1),
		Example: `  tegata edit github --issuer "GitHub Inc"
  tegata edit github --tags "work, totp"
  tegata edit github --category "work"
  tegata edit github --label "github-personal"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			credLabel := args[0]

			// Validate that at least one field is being updated.
			if label == "" && issuer == "" && tags == "" && category == "" {
				return fmt.Errorf(
					"at least one of --label, --issuer, --tags, or --category must be provided: %w",
					errors.ErrInvalidInput,
				)
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

			builder := setupAuditBuilder(cmd.ErrOrStderr(), vaultDir(vaultPath), passphrase, mgr)
			if builder != nil {
				defer func() { _ = builder.Close() }()
			}

			cred, err := mgr.GetCredential(credLabel)
			if err != nil {
				return err
			}

			// Track the original values for audit event decision.
			origLabel := cred.Label
			origIssuer := cred.Issuer
			origTags := slices.Clone(cred.Tags)
			origCategory := cred.Category

			// Apply updates from flags.
			if label != "" {
				label = strings.TrimSpace(label)
				if label == "" {
					return fmt.Errorf("label cannot be empty or whitespace-only: %w",
						errors.ErrInvalidInput)
				}
				// Check for duplicate label.
				for _, c := range mgr.ListCredentials() {
					if strings.EqualFold(c.Label, label) && c.ID != cred.ID {
						return fmt.Errorf("a credential with label %q already exists: %w",
							label, errors.ErrInvalidInput)
					}
				}
				cred.Label = label
			}

			if issuer != "" {
				cred.Issuer = issuer
			}

			if tags != "" {
				// Parse tags: split by comma, trim whitespace, filter empty strings, and normalize to lowercase.
				var newTags []string
				for _, t := range strings.Split(tags, ",") {
					if t = strings.TrimSpace(t); t != "" {
						newTags = append(newTags, strings.ToLower(t))
					}
				}

				// Check for duplicates.
				seen := make(map[string]struct{})
				for _, t := range newTags {
					if _, exists := seen[t]; exists {
						return fmt.Errorf("duplicate tag %q: %w", t, errors.ErrInvalidInput)
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

			// Log one audit event per changed field.
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
							_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Audit log failed: %v\n", logErr)
						}
					}
				}
			}

			fmt.Printf("Updated %q\n", cred.Label)
			if cred.Category != "" {
				fmt.Printf("  Category: %s\n", cred.Category)
			}
			if len(cred.Tags) == 0 {
				fmt.Printf("  Tags: (none)\n")
			} else {
				fmt.Printf("  Tags: %s\n", strings.Join(cred.Tags, ", "))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&label, "label", "", "new label for the credential")
	cmd.Flags().StringVar(&issuer, "issuer", "", "new issuer for the credential")
	cmd.Flags().StringVar(&tags, "tags", "", "comma-separated replacement tag list")
	cmd.Flags().StringVar(&category, "category", "", "category for sidebar grouping (empty string clears it)")

	return cmd
}
