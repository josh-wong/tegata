package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/config"
	"github.com/josh-wong/tegata/internal/crypto"
	"github.com/josh-wong/tegata/internal/vault"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize a new encrypted vault",
		Long: `Create a new encrypted vault file. If a path is given it is used as the
vault directory; otherwise the current directory is used.`,
		Args: cobra.MaximumNArgs(1),
		Example: `  tegata init /mnt/usb
  tegata init`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Determine vault directory.
			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting working directory: %w", err)
			}
			if len(args) > 0 {
				dir = args[0]
			}

			vaultPath := filepath.Join(dir, vaultFilename)

			// Check that vault doesn't already exist.
			if _, err := os.Stat(vaultPath); err == nil {
				return fmt.Errorf("vault already exists at %s", vaultPath)
			}

			// Ensure the directory exists.
			if err := os.MkdirAll(dir, 0700); err != nil {
				return fmt.Errorf("creating directory: %w", err)
			}

			passphrase, err := promptNewPassphrase()
			if err != nil {
				return err
			}
			defer zeroBytes(passphrase)

			recoveryKey, err := vault.Create(vaultPath, passphrase, crypto.DefaultParams)
			if err != nil {
				return fmt.Errorf("creating vault: %w", err)
			}

			// Write default config alongside the vault.
			if err := config.WriteDefaults(dir); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not write default config: %v\n", err)
			}

			fmt.Printf("Vault created: %s\n\n", vaultPath)
			fmt.Println("Recovery key (store this somewhere safe -- you cannot see it again):")
			fmt.Printf("\n    %s\n\n", recoveryKey)
			fmt.Println("If you forget your passphrase, this key is the only way to recover your vault.")

			// Audit opt-in: run full SetupStack immediately on yes.
			fmt.Fprintf(os.Stderr, "\nEnable audit logging? (requires Docker) [y/N]: ")
			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() {
				answer := strings.TrimSpace(scanner.Text())
				if strings.EqualFold(answer, "y") {
					runInitAudit(vaultPath, dir, passphrase)
				}
			}

			return nil
		},
	}
}

// runInitAudit runs the full Docker audit setup immediately after vault
// creation. It opens and unlocks the vault to derive the stable entity ID,
// then calls audit.SetupStack. Errors are printed to stderr and the user is
// directed to run 'tegata ledger start' to retry.
func runInitAudit(vaultPath, dir string, passphrase []byte) {
	fmt.Fprintln(os.Stderr, "Setting up audit ledger (this may take several minutes)...")

	mgr, err := vault.Open(vaultPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Audit setup failed: %v\nRun 'tegata ledger start' to retry.\n", err)
		return
	}
	if err := mgr.Unlock(passphrase); err != nil {
		mgr.Close()
		fmt.Fprintf(os.Stderr, "Audit setup failed: %v\nRun 'tegata ledger start' to retry.\n", err)
		return
	}
	vaultID := mgr.VaultID()
	mgr.Close()

	u, err := user.Current()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Audit setup failed: %v\nRun 'tegata ledger start' to retry.\n", err)
		return
	}
	composeDir := filepath.Join(u.HomeDir, ".tegata", "docker")

	bundleFS, err := fs.Sub(dockerBundle, "docker-bundle")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Audit setup failed: %v\n", err)
		return
	}

	progressFn := func(msg string) { fmt.Fprintln(os.Stderr, msg) }

	auditCfg, err := audit.SetupStack(bundleFS, composeDir, vaultID, progressFn, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Audit setup failed: %v\nRun 'tegata ledger start' to retry.\n", err)
		return
	}

	if writeErr := config.WriteAuditSection(dir, auditCfg); writeErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not save audit config: %v\n", writeErr)
		return
	}

	fmt.Fprintln(os.Stderr, "Audit logging enabled and active.")
}
