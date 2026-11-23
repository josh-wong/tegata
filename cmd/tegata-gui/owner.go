package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/josh-wong/tegata/internal/audit"
	tegerrors "github.com/josh-wong/tegata/internal/errors"
	"github.com/josh-wong/tegata/pkg/model"
)

// OwnerInfo is the frontend view of the vault owner's identifiers. Only short
// hash prefixes are exposed; raw identifiers are never stored, so they cannot
// be returned.
type OwnerInfo struct {
	EmployeeIDHash string `json:"employeeIdHash"`
	EmailHash      string `json:"emailHash"`
	HashScheme     string `json:"hashScheme"`
	UpdatedAt      string `json:"updatedAt"`
	// SaltConfigured reports whether [organization] identifier_salt is set,
	// so the UI can tell the user which hashing scheme a save will use.
	SaltConfigured bool `json:"saltConfigured"`
	// Changed is set by SetOwner and reports whether the save changed any
	// identifier, so the UI does not claim to have saved an unchanged edit.
	// GetOwner always leaves it false.
	Changed bool `json:"changed"`
}

// ErrOwnerSaltChangedCode is the error string the frontend matches to show a
// localized message when the organization salt changed.
const ErrOwnerSaltChangedCode = "owner-salt-changed"

// ErrOwnerConfigUnreadableCode is the error string the frontend matches to
// show a localized message when tegata.toml could not be read.
const ErrOwnerConfigUnreadableCode = "owner-config-unreadable"

// ErrOwnerInvalidCodePrefix starts the error string that SetOwner returns for
// an identifier validation error. The full code is
// "owner-invalid:<field>:<reason>", where field is employeeId or email and
// reason is invalidUtf8, controlChars, tooLong, or missingAt. The frontend
// parses it to show a localized message.
const ErrOwnerInvalidCodePrefix = "owner-invalid:"

// ownerInvalidCode returns the frontend error code for an identifier
// validation error, or "" when err is not one.
func ownerInvalidCode(err error) string {
	var idErr *audit.OwnerIdentifierError
	if !tegerrors.As(err, &idErr) {
		return ""
	}
	field := "employeeId"
	if idErr.Kind == audit.OwnerEmail {
		field = "email"
	}
	var reason string
	switch {
	case tegerrors.Is(err, audit.ErrOwnerInvalidUTF8):
		reason = "invalidUtf8"
	case tegerrors.Is(err, audit.ErrOwnerControlChars):
		reason = "controlChars"
	case tegerrors.Is(err, audit.ErrOwnerTooLong):
		reason = "tooLong"
	case tegerrors.Is(err, audit.ErrOwnerEmailMissingAt):
		reason = "missingAt"
	default:
		return ""
	}
	return ErrOwnerInvalidCodePrefix + field + ":" + reason
}

func (a *App) ownerInfo(o *model.OwnerIdentity) OwnerInfo {
	info := OwnerInfo{SaltConfigured: a.config.Organization.IdentifierSalt != ""}
	if o.IsZero() {
		return info
	}
	info.EmployeeIDHash = audit.ShortOwnerHash(o.EmployeeIDHash)
	info.EmailHash = audit.ShortOwnerHash(o.EmailHash)
	info.HashScheme = o.HashScheme
	info.UpdatedAt = o.UpdatedAt.Format(time.RFC3339)
	return info
}

// GetOwner returns the vault owner's identifiers as short hash prefixes.
func (a *App) GetOwner() (OwnerInfo, error) {
	if a.vault == nil {
		return OwnerInfo{}, fmt.Errorf("vault is locked")
	}
	a.resetIdle()
	return a.ownerInfo(a.vault.Owner()), nil
}

// SetOwner sets, changes, or clears the vault owner's employee ID and email.
// For each identifier, an empty value with its clear flag unset keeps the
// current hash, the clear flag removes it, and any other value replaces it.
// The change is saved to the vault and then recorded as an
// owner-identifier-update audit event.
func (a *App) SetOwner(employeeID, email string, clearEmployeeID, clearEmail bool) (OwnerInfo, error) {
	if a.vault == nil {
		return OwnerInfo{}, fmt.Errorf("vault is locked")
	}
	a.resetIdle()
	field := func(v string, clear bool) *string {
		if clear {
			empty := ""
			return &empty
		}
		// Treat whitespace-only input as "keep" so it is never trimmed to ""
		// and mistaken for a clear request.
		if strings.TrimSpace(v) == "" {
			return nil
		}
		return &v
	}
	employeeIDEdit, emailEdit := field(employeeID, clearEmployeeID), field(email, clearEmail)
	// With an unreadable tegata.toml the organization salt is unknown, so
	// hashing a new value would never match the organization's hashes. Clears
	// need no salt and still go through.
	if a.configLoadFailed && audit.OwnerEditNeedsSalt(employeeIDEdit, emailEdit) {
		return OwnerInfo{}, fmt.Errorf("%s", ErrOwnerConfigUnreadableCode)
	}
	// Nothing entered and nothing cleared is a no-op. Returning early also
	// avoids reporting a salt change for an edit that keeps every hash.
	if employeeIDEdit == nil && emailEdit == nil {
		return a.ownerInfo(a.vault.Owner()), nil
	}
	next, changed, err := audit.UpdateOwner(a.vault, a.builder,
		employeeIDEdit, emailEdit, a.config.Organization.IdentifierSalt)
	if err != nil {
		if tegerrors.Is(err, audit.ErrOwnerSaltChanged) {
			return OwnerInfo{}, fmt.Errorf("%s", ErrOwnerSaltChangedCode)
		}
		if code := ownerInvalidCode(err); code != "" {
			return OwnerInfo{}, fmt.Errorf("%s", code)
		}
		return OwnerInfo{}, err
	}
	audit.ApplyOwner(&a.config.Audit, next)
	info := a.ownerInfo(next)
	info.Changed = changed
	return info, nil
}
