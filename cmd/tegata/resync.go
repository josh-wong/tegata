package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/auth"
	"github.com/josh-wong/tegata/internal/errors"
	"github.com/spf13/cobra"
)

func newResyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resync <label>",
		Short: "Resynchronize an HOTP counter",
		Args:  cobra.ExactArgs(1),
		Example: `  tegata resync my-hotp-service`,
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

			builder := setupAuditBuilder(cmd.ErrOrStderr(), vaultDir(vaultPath), passphrase, mgr)
			if builder != nil {
				defer func() { _ = builder.Close() }()
			}

			cred, err := mgr.GetCredential(label)
			if err != nil {
				return err
			}

			if cred.Type != "hotp" {
				return fmt.Errorf("credential %q is type %s, expected hotp: %w",
					label, cred.Type, errors.ErrInvalidInput)
			}

			scanner := bufio.NewScanner(os.Stdin)

			fmt.Fprint(os.Stderr, "Enter first code from the server/app: ")
			if !scanner.Scan() {
				return fmt.Errorf("reading first code: %w", errors.ErrInvalidInput)
			}
			code1 := strings.TrimSpace(scanner.Text())

			fmt.Fprint(os.Stderr, "Enter second consecutive code: ")
			if !scanner.Scan() {
				return fmt.Errorf("reading second code: %w", errors.ErrInvalidInput)
			}
			code2 := strings.TrimSpace(scanner.Text())

			// Decode the base32 secret.
			secret, err := decodeBase32Secret(cred.Secret)
			if err != nil {
				return fmt.Errorf("decoding secret for %q: %w", label, err)
			}
			defer zeroBytes(secret)

			newCounter, err := auth.ResyncHOTP(secret, code1, code2, cred.Counter, cred.Digits, cred.Algorithm)
			if err != nil {
				fmt.Println("Could not find matching counter within look-ahead window. Verify the codes are from the correct service.")
				return err
			}

			cred.Counter = newCounter
			if err := mgr.UpdateCredential(cred); err != nil {
				return fmt.Errorf("saving counter: %w", err)
			}

			if builder != nil {
				if logErr := builder.LogEvent("credential-update", cred.Label, cred.Issuer, audit.Hostname(), true); logErr != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Audit log failed: %v\n", logErr)
				}
			}

			fmt.Printf("Counter resynchronized. Next code will use counter %d.\n", newCounter)
			return nil
		},
	}

	return cmd
}
