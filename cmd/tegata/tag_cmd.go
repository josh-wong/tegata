package main

import (
	"fmt"
	"strings"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/config"
	"github.com/josh-wong/tegata/internal/errors"
	"github.com/spf13/cobra"
)

func newTagCmd() *cobra.Command {
	var (
		addTags    []string
		removeTags []string
	)

	cmd := &cobra.Command{
		Use:   "tag <label>",
		Short: "Add or remove tags on an existing credential",
		Args:  cobra.ExactArgs(1),
		Example: `  tegata tag github --add work --add totp
  tegata tag github --remove personal`,
		RunE: func(cmd *cobra.Command, args []string) error {
			label := args[0]

			if len(addTags) == 0 && len(removeTags) == 0 {
				return fmt.Errorf(
					"at least one of --add or --remove must be provided: %w",
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

			cfg, _ := config.Load(vaultDir(vaultPath))
			builder, err := newEventBuilder(cfg, vaultDir(vaultPath), passphrase)
			if err != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: audit unavailable: %v\n", err)
			}
			if builder != nil {
				defer func() { _ = builder.Close() }()
				builder.OnHashStored = func(eventID, hashValue string) {
					if err := mgr.SetAuditHash(eventID, hashValue); err != nil {
						_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to store audit hash: %v\n", err)
					}
				}
				if logErr := builder.LogEvent("vault-unlock", "", "", audit.Hostname(), true); logErr != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: audit log failed: %v\n", logErr)
				}
			}

			cred, err := mgr.GetCredential(label)
			if err != nil {
				return err
			}

			// Build a set of current tags for efficient lookup.
			tagSet := make(map[string]struct{}, len(cred.Tags))
			for _, t := range cred.Tags {
				tagSet[t] = struct{}{}
			}

			// Add new tags that are not already present (case-sensitive).
			for _, t := range addTags {
				if _, exists := tagSet[t]; !exists {
					cred.Tags = append(cred.Tags, t)
					tagSet[t] = struct{}{}
				}
			}

			// Remove tags that match (case-sensitive).
			removeSet := make(map[string]struct{}, len(removeTags))
			for _, t := range removeTags {
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
				if logErr := builder.LogEvent("credential-update", cred.Label, cred.Issuer, audit.Hostname(), true); logErr != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: audit log failed: %v\n", logErr)
				}
			}

			if len(cred.Tags) == 0 {
				fmt.Printf("Tags for %s: (none)\n", cred.Label)
			} else {
				fmt.Printf("Tags for %s: %s\n", cred.Label, strings.Join(cred.Tags, ", "))
			}
			return nil
		},
	}

	cmd.Flags().StringArrayVar(&addTags, "add", nil, "tag to add (repeatable)")
	cmd.Flags().StringArrayVar(&removeTags, "remove", nil, "tag to remove (repeatable)")

	return cmd
}
