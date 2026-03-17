package main

import (
	"fmt"

	"github.com/josh-wong/tegata/internal/auth"
	"github.com/josh-wong/tegata/internal/clipboard"
	"github.com/josh-wong/tegata/internal/config"
	"github.com/spf13/cobra"
)

func newGetCmd() *cobra.Command {
	var show bool

	cmd := &cobra.Command{
		Use:   "get <label>",
		Short: "Retrieve a static password",
		Args:  cobra.ExactArgs(1),
		Example: `  tegata get backup-key
  tegata get --show backup-key`,
		RunE: func(cmd *cobra.Command, args []string) error {
			label := args[0]

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

			cred, err := mgr.GetCredential(label)
			if err != nil {
				return err
			}

			password, err := auth.GetStaticPassword(cred)
			if err != nil {
				return err
			}

			// Load config for clipboard timeout.
			cfg, _ := config.Load(vaultDir(vaultPath))

			cm := clipboard.NewManager()
			defer cm.Close()
			if err := cm.CopyWithAutoClear(password, cfg.ClipboardTimeout); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: clipboard copy failed: %v\n", err)
			} else {
				fmt.Printf("Copied to clipboard (auto-clear in %ds)\n",
					int(cfg.ClipboardTimeout.Seconds()))
			}

			if show {
				fmt.Printf("Password: %s\n", password)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&show, "show", false, "display password in terminal")

	return cmd
}
