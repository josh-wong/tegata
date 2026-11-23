package audit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/josh-wong/tegata/internal/config"
	"github.com/josh-wong/tegata/pkg/model"
	"golang.org/x/text/unicode/norm"
)

// Owner hash schemes recorded in model.OwnerIdentity.HashScheme and in audit
// events. Administrators use the scheme to reproduce a hash from a known
// identifier when mapping vaults and containers to people.
const (
	// OwnerHashSchemeSHA256 is SHA-256(normalized identifier), used when no
	// organization salt is configured.
	OwnerHashSchemeSHA256 = "sha256"
	// OwnerHashSchemeHMACSHA256 is HMAC-SHA256(key=salt, msg=normalized
	// identifier), used when identifier_salt is set in the [organization]
	// section of tegata.toml.
	OwnerHashSchemeHMACSHA256 = "hmac-sha256"
)

// MaxOwnerIdentifierLen is the maximum length, in characters, of an employee
// ID or email address after normalization.
const MaxOwnerIdentifierLen = 256

// saltIDMessage is the fixed message HMACed with the salt to derive SaltID.
const saltIDMessage = "tegata-owner-salt-id-v1"

// ErrOwnerSaltChanged is returned by MergeOwnerIdentity when an edit would
// keep an identifier hash computed under a different salt than the one now in
// effect. The user must re-enter every identifier that is still set.
var ErrOwnerSaltChanged = errors.New("the identifier salt has changed since the identifiers were set; re-enter both the employee ID and the email address")

// ErrOwnerSave wraps the error returned by UpdateOwner when the new identity
// could not be saved to the vault. It lets callers tell a storage failure
// apart from invalid identifier input.
var ErrOwnerSave = errors.New("saving owner identifiers")

// OwnerIdentifierKind distinguishes the two supported identifier inputs. The
// kind controls normalization only; both kinds share the same hash scheme.
type OwnerIdentifierKind int

const (
	// OwnerEmployeeID is an organization-issued employee ID or stable UUID.
	// It is the recommended primary identifier.
	OwnerEmployeeID OwnerIdentifierKind = iota
	// OwnerEmail is an email address, used when no employee ID is available
	// or alongside one so administrators can cross-check.
	OwnerEmail
)

// NormalizeOwnerIdentifier returns the canonical form of an identifier that is
// hashed. Surrounding whitespace is trimmed for both kinds, and the value is
// converted to Unicode Normalization Form C (NFC) so that composed and
// decomposed spellings of the same characters, which differ by input method
// and operating system, hash identically. Email addresses are lowercased;
// employee IDs keep their case because some organizations issue
// case-sensitive IDs. No organization-specific format is enforced. An empty
// result means "not set". Values that contain control characters or exceed
// MaxOwnerIdentifierLen are rejected so that hashes are reproducible from what
// an administrator can type.
func NormalizeOwnerIdentifier(kind OwnerIdentifierKind, raw string) (string, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "", nil
	}
	if !utf8.ValidString(v) {
		return "", errors.New("identifier is not valid UTF-8")
	}
	v = norm.NFC.String(v)
	for _, r := range v {
		if unicode.IsControl(r) {
			return "", errors.New("identifier must not contain control characters")
		}
	}
	if utf8.RuneCountInString(v) > MaxOwnerIdentifierLen {
		return "", fmt.Errorf("identifier must be at most %d characters", MaxOwnerIdentifierLen)
	}
	if kind == OwnerEmail {
		v = strings.ToLower(v)
		if !strings.Contains(v, "@") {
			return "", errors.New("email address must contain @")
		}
	}
	return v, nil
}

// HashOwnerIdentifier hashes an already-normalized identifier. When salt is
// empty the result is SHA-256(value); otherwise it is HMAC-SHA256 keyed with
// the organization salt, which prevents matching a leaked ledger against a
// staff directory without also knowing the salt. Both forms are hex-encoded
// and can be reproduced with standard tools, for example
// `printf %s "$ID" | sha256sum` or
// `printf %s "$ID" | openssl dgst -sha256 -hmac "$SALT"`.
func HashOwnerIdentifier(normalized, salt string) string {
	if salt == "" {
		return HashString(normalized)
	}
	mac := hmac.New(sha256.New, []byte(salt))
	mac.Write([]byte(normalized))
	return hex.EncodeToString(mac.Sum(nil))
}

// ownerScheme returns the hash scheme and salt fingerprint for salt. The
// fingerprint is the first 16 hex characters of HMAC-SHA256(salt, fixed
// message), which identifies the salt without revealing it.
func ownerScheme(salt string) (scheme, saltID string) {
	if salt == "" {
		return OwnerHashSchemeSHA256, ""
	}
	mac := hmac.New(sha256.New, []byte(salt))
	mac.Write([]byte(saltIDMessage))
	return OwnerHashSchemeHMACSHA256, hex.EncodeToString(mac.Sum(nil))[:16]
}

// hashField normalizes and hashes one raw identifier. Empty input yields "".
func hashField(kind OwnerIdentifierKind, raw, salt string) (string, error) {
	normalized, err := NormalizeOwnerIdentifier(kind, raw)
	if err != nil || normalized == "" {
		return "", err
	}
	return HashOwnerIdentifier(normalized, salt), nil
}

// MergeOwnerIdentity applies an edit to the current identity and returns the
// new one. For each identifier, a nil pointer keeps the current hash, a
// pointer to an empty string clears it, and any other value replaces it with
// a fresh hash. Raw inputs are never retained. A nil result means no
// identifier is set.
//
// Because hashes cannot be reversed, a kept hash cannot be recomputed under a
// new salt. When the salt has changed and the edit would keep an existing
// hash, ErrOwnerSaltChanged is returned.
func MergeOwnerIdentity(current *model.OwnerIdentity, employeeID, email *string, salt string, now time.Time) (*model.OwnerIdentity, error) {
	scheme, saltID := ownerScheme(salt)
	next := &model.OwnerIdentity{HashScheme: scheme, SaltID: saltID, UpdatedAt: now.UTC()}

	keptOld := false
	if employeeID != nil {
		h, err := hashField(OwnerEmployeeID, *employeeID, salt)
		if err != nil {
			return nil, fmt.Errorf("employee ID: %w", err)
		}
		next.EmployeeIDHash = h
	} else if current != nil && current.EmployeeIDHash != "" {
		next.EmployeeIDHash = current.EmployeeIDHash
		keptOld = true
	}
	if email != nil {
		h, err := hashField(OwnerEmail, *email, salt)
		if err != nil {
			return nil, fmt.Errorf("email: %w", err)
		}
		next.EmailHash = h
	} else if current != nil && current.EmailHash != "" {
		next.EmailHash = current.EmailHash
		keptOld = true
	}

	if next.IsZero() {
		return nil, nil
	}
	if keptOld && (current.HashScheme != scheme || current.SaltID != saltID) {
		return nil, ErrOwnerSaltChanged
	}
	return next, nil
}

// OwnerEqual reports whether two identities hold the same hashes under the
// same scheme. UpdatedAt is ignored.
func OwnerEqual(a, b *model.OwnerIdentity) bool {
	if a.IsZero() || b.IsZero() {
		return a.IsZero() && b.IsZero()
	}
	return a.EmployeeIDHash == b.EmployeeIDHash &&
		a.EmailHash == b.EmailHash &&
		a.HashScheme == b.HashScheme &&
		a.SaltID == b.SaltID
}

// ShortOwnerHash returns the first 12 hex characters of a hash for display in
// settings and history views. The short form lets a user or administrator
// confirm which identifier is set without showing the full digest.
func ShortOwnerHash(hash string) string {
	if len(hash) <= 12 {
		return hash
	}
	return hash[:12]
}

// OwnerStore is the subset of the vault manager used to read and persist the
// owner identity. It is an interface because the vault package imports audit.
type OwnerStore interface {
	Owner() *model.OwnerIdentity
	SetOwner(*model.OwnerIdentity) error
}

// UpdateOwner applies an identifier edit (see MergeOwnerIdentity for the
// meaning of nil and empty inputs), saves the result to store, and then logs
// an owner-identifier-update event through b. The event is logged only after
// the vault save succeeds, and only when the hashes actually changed. b may
// be nil or disabled, in which case the identity is saved without an event.
// It returns the new identity (nil when cleared) and whether it changed.
func UpdateOwner(store OwnerStore, b *EventBuilder, employeeID, email *string, salt string) (*model.OwnerIdentity, bool, error) {
	old := store.Owner()
	next, err := MergeOwnerIdentity(old, employeeID, email, salt, time.Now())
	if err != nil {
		return nil, false, err
	}
	if OwnerEqual(old, next) {
		return old, false, nil
	}
	if err := store.SetOwner(next); err != nil {
		return nil, false, fmt.Errorf("%w: %w", ErrOwnerSave, err)
	}
	if b != nil {
		// LogOwnerUpdate never fails the caller; ledger errors queue the event.
		_ = b.LogOwnerUpdate(old, next)
	}
	return next, true, nil
}

// ApplyOwner copies the owner's primary hash and scheme into the runtime-only
// fields of cfg so that Docker auto-start writes current ownership labels. A
// nil owner clears the fields.
func ApplyOwner(cfg *config.AuditConfig, o *model.OwnerIdentity) {
	if o.IsZero() {
		cfg.OwnerHash, cfg.OwnerHashScheme = "", ""
		return
	}
	cfg.OwnerHash, cfg.OwnerHashScheme = o.PrimaryHash(), o.HashScheme
}
