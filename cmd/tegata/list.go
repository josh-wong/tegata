package main

import (
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/josh-wong/tegata/internal/i18n"
	pkgmodel "github.com/josh-wong/tegata/pkg/model"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var tagFilter string

	cmd := &cobra.Command{
		Use:     "list",
		Short:   i18n.T("cmd.list.short"),
		Args:    cobra.NoArgs,
		Example: i18n.T("cmd.list.example"),
		RunE: func(cmd *cobra.Command, args []string) error {
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

			creds := mgr.ListCredentials()
			if len(creds) == 0 {
				fmt.Println(i18n.T("cmd.list.empty"))
				return nil
			}

			out := cmd.OutOrStdout()

			if tagFilter != "" {
				// Flat filtered list: only credentials with an exact case-sensitive
				// match against the requested tag.
				w := tabwriter.NewWriter(out, 0, 2, 3, ' ', 0)
				_, _ = fmt.Fprintln(w, i18n.T("cmd.list.header"))
				matched := 0
				for _, c := range creds {
					for _, t := range c.Tags {
						if t == tagFilter {
							issuer := c.Issuer
							if issuer == "" {
								issuer = i18n.T("cmd.list.emptyIssuer")
							}
							_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", c.Label, issuer, c.Type)
							matched++
							break
						}
					}
				}
				if matched == 0 {
					if _, err := fmt.Fprint(out,
						i18n.Tf("cmd.list.emptyTag", map[string]any{"Tag": tagFilter})); err != nil {
						return err
					}
				}
				return w.Flush()
			}

			// Default grouped output: credentials appear under each tag they
			// belong to. Credentials with no tags appear under [untagged].
			tagSet := map[string][]pkgmodel.Credential{}
			var untagged []pkgmodel.Credential
			for _, c := range creds {
				if len(c.Tags) == 0 {
					untagged = append(untagged, c)
					continue
				}
				for _, t := range c.Tags {
					tagSet[t] = append(tagSet[t], c)
				}
			}

			var tagNames []string
			for t := range tagSet {
				tagNames = append(tagNames, t)
			}
			sort.Strings(tagNames)

			w := tabwriter.NewWriter(out, 0, 2, 3, ' ', 0)
			for _, tag := range tagNames {
				if _, err := fmt.Fprintf(out, "[%s]\n", tag); err != nil {
					return err
				}
				for _, c := range tagSet[tag] {
					issuer := c.Issuer
					if issuer == "" {
						issuer = i18n.T("cmd.list.emptyIssuer")
					}
					_, _ = fmt.Fprintf(w, "  %s\t%s\t%s\n", c.Label, issuer, c.Type)
				}
				_ = w.Flush()
				if _, err := fmt.Fprintln(out); err != nil {
					return err
				}
			}

			if len(untagged) > 0 {
				if _, err := fmt.Fprintf(out, "[%s]\n", i18n.T("cmd.list.untagged")); err != nil {
					return err
				}
				for _, c := range untagged {
					issuer := c.Issuer
					if issuer == "" {
						issuer = i18n.T("cmd.list.emptyIssuer")
					}
					_, _ = fmt.Fprintf(w, "  %s\t%s\t%s\n", c.Label, issuer, c.Type)
				}
				_ = w.Flush()
				if _, err := fmt.Fprintln(out); err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&tagFilter, "tag", "", i18n.T("cmd.list.flag.tag"))

	return cmd
}
