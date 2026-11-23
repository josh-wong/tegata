package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/config"
	"github.com/josh-wong/tegata/internal/errors"
	"github.com/josh-wong/tegata/internal/i18n"
	"github.com/josh-wong/tegata/internal/vault"
	pkgmodel "github.com/josh-wong/tegata/pkg/model"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// newOwnerCmd returns the 'tegata owner' command group, which manages the
// hashed employee ID and email that attribute audit events to the vault owner
// in managed multi-user deployments.
func newOwnerCmd() *cobra.Command {
	ownerCmd := &cobra.Command{
		Use:   "owner",
		Short: i18n.T("cmd.owner.short"),
		Long:  i18n.T("cmd.owner.long"),
	}
	ownerCmd.AddCommand(newOwnerShowCmd(), newOwnerSetCmd(), newOwnerClearCmd())
	return ownerCmd
}

func newOwnerShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "show",
		Short:   i18n.T("cmd.owner.show.short"),
		Example: i18n.T("cmd.owner.show.example"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			vaultPath, err := resolveVaultPath(cmd)
			if err != nil {
				return err
			}
			passphrase, err := promptPassphrase(i18n.T("cmd.prompt.passphrase"))
			if err != nil {
				return err
			}
			mgr, err := openAndUnlock(vaultPath, passphrase)
			zeroBytes(passphrase)
			if err != nil {
				return err
			}
			defer mgr.Close()

			printOwner(cmd.OutOrStdout(), mgr.Owner())
			return nil
		},
	}
}

func newOwnerSetCmd() *cobra.Command {
	var employeeID, email string

	cmd := &cobra.Command{
		Use:     "set",
		Short:   i18n.T("cmd.owner.set.short"),
		Long:    i18n.T("cmd.owner.set.long"),
		Example: i18n.T("cmd.owner.set.example"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var idPtr, emailPtr *string
			if cmd.Flags().Changed("employee-id") {
				idPtr = &employeeID
			}
			if cmd.Flags().Changed("email") {
				emailPtr = &email
			}

			// Without flags, prompt interactively so raw identifiers do not end
			// up in shell history. Prompts run before the passphrase prompt
			// because a piped passphrase consumes all of stdin.
			if idPtr == nil && emailPtr == nil {
				if !term.IsTerminal(int(os.Stdin.Fd())) {
					return fmt.Errorf("%s: %w", i18n.T("cmd.owner.set.error.noInput"), errors.ErrInvalidInput)
				}
				errOut := cmd.ErrOrStderr()
				_, _ = fmt.Fprintln(errOut, i18n.T("cmd.owner.set.info.prompt"))
				reader := bufio.NewReader(os.Stdin)
				idPtr = promptOptionalLine(errOut, reader, i18n.T("cmd.owner.set.prompt.employeeID"))
				emailPtr = promptOptionalLine(errOut, reader, i18n.T("cmd.owner.set.prompt.email"))
				if idPtr == nil && emailPtr == nil {
					_, _ = fmt.Fprintln(errOut, i18n.T("cmd.owner.info.unchanged"))
					return nil
				}
			}
			if (idPtr != nil && strings.TrimSpace(*idPtr) == "") || (emailPtr != nil && strings.TrimSpace(*emailPtr) == "") {
				return fmt.Errorf("%s: %w", i18n.T("cmd.owner.set.error.empty"), errors.ErrInvalidInput)
			}
			return runOwnerUpdate(cmd, idPtr, emailPtr)
		},
	}

	cmd.Flags().StringVar(&employeeID, "employee-id", "", i18n.T("cmd.owner.set.flag.employeeID"))
	cmd.Flags().StringVar(&email, "email", "", i18n.T("cmd.owner.set.flag.email"))
	return cmd
}

func newOwnerClearCmd() *cobra.Command {
	var clearID, clearEmail bool

	cmd := &cobra.Command{
		Use:     "clear",
		Short:   i18n.T("cmd.owner.clear.short"),
		Example: i18n.T("cmd.owner.clear.example"),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// With no flags, clear both identifiers.
			if !clearID && !clearEmail {
				clearID, clearEmail = true, true
			}
			empty := ""
			var idPtr, emailPtr *string
			if clearID {
				idPtr = &empty
			}
			if clearEmail {
				emailPtr = &empty
			}
			return runOwnerUpdate(cmd, idPtr, emailPtr)
		},
	}

	cmd.Flags().BoolVar(&clearID, "employee-id", false, i18n.T("cmd.owner.clear.flag.employeeID"))
	cmd.Flags().BoolVar(&clearEmail, "email", false, i18n.T("cmd.owner.clear.flag.email"))
	return cmd
}

// runOwnerUpdate unlocks the vault, applies the identifier edit, and logs an
// owner-identifier-update audit event when the identifiers changed.
func runOwnerUpdate(cmd *cobra.Command, employeeID, email *string) error {
	vaultPath, err := resolveVaultPath(cmd)
	if err != nil {
		return err
	}
	dir := vaultDir(vaultPath)
	cfg, err := config.Load(dir)
	if err != nil && audit.OwnerEditNeedsSalt(employeeID, email) {
		return fmt.Errorf("%s: %w", i18n.T("cmd.owner.error.loadConfig"), err)
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

	builder := setupAuditBuilder(cmd.ErrOrStderr(), dir, passphrase, mgr)
	if builder != nil {
		defer func() { _ = builder.Close() }()
	}

	next, changed, err := updateOwner(mgr, builder, employeeID, email, cfg.Organization.IdentifierSalt)
	if err != nil {
		return err
	}
	if !changed {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), i18n.T("cmd.owner.info.unchanged"))
		return nil
	}
	_, _ = fmt.Fprintln(cmd.ErrOrStderr(), i18n.T("cmd.owner.success.updated"))
	printOwner(cmd.OutOrStdout(), next)
	return nil
}

// updateOwner wraps audit.UpdateOwner and maps validation errors to
// ErrInvalidInput so the CLI exits with the usage error code. Vault save
// failures are returned unchanged so they keep their own exit code.
func updateOwner(mgr *vault.Manager, builder *audit.EventBuilder, employeeID, email *string, salt string) (*pkgmodel.OwnerIdentity, bool, error) {
	next, changed, err := audit.UpdateOwner(mgr, builder, employeeID, email, salt)
	if err != nil {
		if errors.Is(err, audit.ErrOwnerSaltChanged) {
			return nil, false, fmt.Errorf("%s: %w", i18n.T("cmd.owner.error.saltChanged"), errors.ErrInvalidInput)
		}
		if errors.Is(err, audit.ErrOwnerSave) || errors.Is(err, errors.ErrVaultLocked) {
			return nil, false, err
		}
		return nil, false, fmt.Errorf("%s: %w", ownerErrorMessage(err), errors.ErrInvalidInput)
	}
	return next, changed, nil
}

// ownerErrorMessage returns a localized message for an identifier validation
// error from audit.UpdateOwner. Errors it does not recognize are returned as
// their English text.
func ownerErrorMessage(err error) string {
	var idErr *audit.OwnerIdentifierError
	if !errors.As(err, &idErr) {
		return err.Error()
	}
	field := i18n.T("cmd.owner.error.field.employeeID")
	if idErr.Kind == audit.OwnerEmail {
		field = i18n.T("cmd.owner.error.field.email")
	}
	data := map[string]any{"Field": field, "Max": audit.MaxOwnerIdentifierLen}
	switch {
	case errors.Is(err, audit.ErrOwnerInvalidUTF8):
		return i18n.Tf("cmd.owner.error.invalidUTF8", data)
	case errors.Is(err, audit.ErrOwnerControlChars):
		return i18n.Tf("cmd.owner.error.controlChars", data)
	case errors.Is(err, audit.ErrOwnerTooLong):
		return i18n.Tf("cmd.owner.error.tooLong", data)
	case errors.Is(err, audit.ErrOwnerEmailMissingAt):
		return i18n.T("cmd.owner.error.emailMissingAt")
	}
	return err.Error()
}

// promptOptionalLine prints prompt and reads one line from r. It returns nil
// when the line is blank, meaning "keep the current value". The prompt is
// written to w.
func promptOptionalLine(w io.Writer, r *bufio.Reader, prompt string) *string {
	_, _ = fmt.Fprint(w, prompt)
	line, err := r.ReadString('\n')
	if err != nil && line == "" {
		return nil
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}
	return &line
}

// printOwner writes the owner identity to w. Only short hash prefixes are
// shown; raw identifiers are never stored, so they cannot be displayed.
func printOwner(w io.Writer, o *pkgmodel.OwnerIdentity) {
	if o.IsZero() {
		_, _ = fmt.Fprintln(w, i18n.T("cmd.owner.show.none"))
		return
	}
	notSet := i18n.T("cmd.owner.show.notSet")
	field := func(h string) string {
		if h == "" {
			return notSet
		}
		return audit.ShortOwnerHash(h)
	}
	primary := i18n.T("cmd.owner.show.primaryEmployeeID")
	if o.EmployeeIDHash == "" {
		primary = i18n.T("cmd.owner.show.primaryEmail")
	}
	_, _ = fmt.Fprint(w, i18n.Tf("cmd.owner.show.output", map[string]any{
		"EmployeeID": field(o.EmployeeIDHash),
		"Email":      field(o.EmailHash),
		"Primary":    primary,
		"Scheme":     o.HashScheme,
		"Updated":    o.UpdatedAt.Local().Format("2006-01-02 15:04:05"),
	}))
}
