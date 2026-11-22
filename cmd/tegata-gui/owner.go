package main

import (
	"fmt"
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
}

// ErrOwnerSaltChangedCode is the error string the frontend matches to show a
// localized message when the organization salt changed.
const ErrOwnerSaltChangedCode = "owner-salt-changed"

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
		if v == "" {
			return nil
		}
		return &v
	}
	next, _, err := audit.UpdateOwner(a.vault, a.builder,
		field(employeeID, clearEmployeeID), field(email, clearEmail),
		a.config.Organization.IdentifierSalt)
	if err != nil {
		if tegerrors.Is(err, audit.ErrOwnerSaltChanged) {
			return OwnerInfo{}, fmt.Errorf("%s", ErrOwnerSaltChangedCode)
		}
		return OwnerInfo{}, err
	}
	audit.ApplyOwner(&a.config.Audit, next)
	return a.ownerInfo(next), nil
}
