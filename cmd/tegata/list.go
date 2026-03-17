package main

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List all credentials in the vault",
		Args:    cobra.NoArgs,
		Example: `  tegata list`,
		RunE: func(cmd *cobra.Command, args []string) error {
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

			creds := mgr.ListCredentials()
			if len(creds) == 0 {
				fmt.Println("No credentials stored. Run tegata add to add one.")
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 3, ' ', 0)
			_, _ = fmt.Fprintln(w, "LABEL\tISSUER\tTYPE")
			for _, c := range creds {
				issuer := c.Issuer
				if issuer == "" {
					issuer = "--"
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", c.Label, issuer, c.Type)
			}
			return w.Flush()
		},
	}
}
