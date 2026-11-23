package audit

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/josh-wong/tegata/internal/config"
	"github.com/josh-wong/tegata/pkg/model"
)

// Known-answer vectors computed with standard tools, matching the commands in
// the managed deployment guide:
//
//	printf %s "acme-emp-12345"   | shasum -a 256
//	printf %s "alex@example.com" | shasum -a 256
//	printf %s "acme-emp-12345"   | openssl dgst -sha256 -hmac "org-salt-1"
const (
	vecEmployeeSHA   = "337b17df6e2a61e9c6d29c94acfdc01172dab6ad997e88023d05a06c2ffd008a"
	vecEmailSHA      = "6db61e6dcbcf2390e4a46af426f26a133a3bee45021422fc7ae86e9136f14110"
	vecEmployeeHMAC  = "ecb503194f22440d11efc4aeff348844ff27113289b611eea4ad7f282a81951a"
	vecSalt          = "org-salt-1"
	vecSaltIDPrefix  = "8be49129ff74b005"
	vecEmployeeInput = "acme-emp-12345"
)

func strPtr(s string) *string { return &s }

func TestNormalizeOwnerIdentifier(t *testing.T) {
	tests := []struct {
		name    string
		kind    OwnerIdentifierKind
		in      string
		want    string
		wantErr bool
	}{
		{"trims employee ID", OwnerEmployeeID, "  acme-emp-1 \t", "acme-emp-1", false},
		{"keeps employee ID case", OwnerEmployeeID, "ACME-Emp-1", "ACME-Emp-1", false},
		{"lowercases email", OwnerEmail, " Alex@Example.COM ", "alex@example.com", false},
		{"blank means unset", OwnerEmail, "   ", "", false},
		{"email requires at sign", OwnerEmail, "alex.example.com", "", true},
		{"rejects control characters", OwnerEmployeeID, "acme\x00emp", "", true},
		{"rejects embedded newline", OwnerEmployeeID, "acme\nemp", "", true},
		{"rejects invalid UTF-8", OwnerEmployeeID, "acme\xffemp", "", true},
		{"accepts max length", OwnerEmployeeID, strings.Repeat("a", MaxOwnerIdentifierLen), strings.Repeat("a", MaxOwnerIdentifierLen), false},
		{"rejects over max length", OwnerEmployeeID, strings.Repeat("a", MaxOwnerIdentifierLen+1), "", true},
		{"counts characters not bytes", OwnerEmployeeID, strings.Repeat("社", MaxOwnerIdentifierLen), strings.Repeat("社", MaxOwnerIdentifierLen), false},
		// "e" + U+0301 COMBINING ACUTE ACCENT (NFD) becomes U+00E9 (NFC).
		{"composes decomposed accents", OwnerEmployeeID, "rené-1", "rené-1", false},
		// "カ" + U+3099 COMBINING VOICED SOUND MARK becomes "ガ" (U+30AC).
		{"composes decomposed kana", OwnerEmployeeID, "ガ-1", "ガ-1", false},
		{"composes before lowercasing email", OwnerEmail, "RÉNE@example.com", "réne@example.com", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeOwnerIdentifier(tt.kind, tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHashOwnerIdentifier_KnownVectors(t *testing.T) {
	if got := HashOwnerIdentifier(vecEmployeeInput, ""); got != vecEmployeeSHA {
		t.Errorf("SHA-256: got %s, want %s", got, vecEmployeeSHA)
	}
	if got := HashOwnerIdentifier(vecEmployeeInput, vecSalt); got != vecEmployeeHMAC {
		t.Errorf("HMAC-SHA256: got %s, want %s", got, vecEmployeeHMAC)
	}
	scheme, saltID := ownerScheme(vecSalt)
	if scheme != OwnerHashSchemeHMACSHA256 || saltID != vecSaltIDPrefix {
		t.Errorf("ownerScheme(salt) = %q, %q; want %q, %q", scheme, saltID, OwnerHashSchemeHMACSHA256, vecSaltIDPrefix)
	}
	if scheme, saltID := ownerScheme(""); scheme != OwnerHashSchemeSHA256 || saltID != "" {
		t.Errorf("ownerScheme(\"\") = %q, %q", scheme, saltID)
	}
}

func TestMergeOwnerIdentity_SetBoth(t *testing.T) {
	now := time.Date(2026, 11, 22, 1, 2, 3, 0, time.FixedZone("JST", 9*3600))
	o, err := MergeOwnerIdentity(nil, strPtr(" "+vecEmployeeInput+" "), strPtr("Alex@Example.com"), "", now)
	if err != nil {
		t.Fatal(err)
	}
	if o.EmployeeIDHash != vecEmployeeSHA || o.EmailHash != vecEmailSHA {
		t.Errorf("hashes = %s, %s", o.EmployeeIDHash, o.EmailHash)
	}
	if o.HashScheme != OwnerHashSchemeSHA256 || o.SaltID != "" {
		t.Errorf("scheme = %q saltID = %q", o.HashScheme, o.SaltID)
	}
	if !o.UpdatedAt.Equal(now) || o.UpdatedAt.Location() != time.UTC {
		t.Errorf("UpdatedAt = %v, want %v in UTC", o.UpdatedAt, now)
	}
	if o.PrimaryHash() != vecEmployeeSHA {
		t.Errorf("PrimaryHash should prefer the employee ID")
	}
}

func TestMergeOwnerIdentity_KeepClearReplace(t *testing.T) {
	now := time.Now()
	cur, _ := MergeOwnerIdentity(nil, strPtr(vecEmployeeInput), strPtr("alex@example.com"), "", now)

	// nil keeps, "" clears.
	o, err := MergeOwnerIdentity(cur, nil, strPtr(""), "", now)
	if err != nil {
		t.Fatal(err)
	}
	if o.EmployeeIDHash != vecEmployeeSHA || o.EmailHash != "" {
		t.Errorf("keep/clear: got %+v", o)
	}

	// Clearing the employee ID makes the email the primary identifier.
	o, err = MergeOwnerIdentity(cur, strPtr(""), nil, "", now)
	if err != nil {
		t.Fatal(err)
	}
	if o.PrimaryHash() != vecEmailSHA {
		t.Errorf("PrimaryHash after clearing employee ID = %s, want email hash", o.PrimaryHash())
	}

	// Clearing both returns nil.
	o, err = MergeOwnerIdentity(cur, strPtr(""), strPtr(" "), "", now)
	if err != nil || o != nil {
		t.Errorf("clear both: got %+v, %v; want nil, nil", o, err)
	}
}

func TestMergeOwnerIdentity_ValidationError(t *testing.T) {
	_, err := MergeOwnerIdentity(nil, nil, strPtr("not-an-email"), "", time.Now())
	if err == nil || !strings.HasPrefix(err.Error(), "email:") {
		t.Errorf("err = %v, want email validation error", err)
	}
	var idErr *OwnerIdentifierError
	if !errors.As(err, &idErr) || idErr.Kind != OwnerEmail || !errors.Is(err, ErrOwnerEmailMissingAt) {
		t.Errorf("err = %#v, want OwnerIdentifierError{Kind: OwnerEmail} wrapping ErrOwnerEmailMissingAt", err)
	}
}

func TestNormalizeOwnerIdentifier_SentinelErrors(t *testing.T) {
	tests := []struct {
		kind OwnerIdentifierKind
		in   string
		want error
	}{
		{OwnerEmployeeID, "acme\xffemp", ErrOwnerInvalidUTF8},
		{OwnerEmployeeID, "acme\x00emp", ErrOwnerControlChars},
		{OwnerEmployeeID, strings.Repeat("a", MaxOwnerIdentifierLen+1), ErrOwnerTooLong},
		{OwnerEmail, "alex.example.com", ErrOwnerEmailMissingAt},
	}
	for _, tt := range tests {
		if _, err := NormalizeOwnerIdentifier(tt.kind, tt.in); !errors.Is(err, tt.want) {
			t.Errorf("NormalizeOwnerIdentifier(%q) err = %v, want %v", tt.in, err, tt.want)
		}
	}
}

func TestMergeOwnerIdentity_SaltChange(t *testing.T) {
	now := time.Now()
	cur, _ := MergeOwnerIdentity(nil, strPtr(vecEmployeeInput), strPtr("alex@example.com"), "", now)

	// Keeping a hash computed without a salt while a salt is now configured
	// would mix schemes.
	if _, err := MergeOwnerIdentity(cur, nil, strPtr("alex@example.com"), vecSalt, now); !errors.Is(err, ErrOwnerSaltChanged) {
		t.Errorf("unsalted to salted: err = %v, want ErrOwnerSaltChanged", err)
	}

	// Re-entering both under the new salt succeeds.
	o, err := MergeOwnerIdentity(cur, strPtr(vecEmployeeInput), strPtr("alex@example.com"), vecSalt, now)
	if err != nil {
		t.Fatal(err)
	}
	if o.EmployeeIDHash != vecEmployeeHMAC || o.HashScheme != OwnerHashSchemeHMACSHA256 || o.SaltID != vecSaltIDPrefix {
		t.Errorf("re-entered: got %+v", o)
	}

	// Switching from one salt to another is also detected.
	if _, err := MergeOwnerIdentity(o, nil, strPtr(""), "other-salt", now); !errors.Is(err, ErrOwnerSaltChanged) {
		t.Errorf("salt A to salt B: err = %v, want ErrOwnerSaltChanged", err)
	}

	// Clearing the kept field instead of keeping it is allowed.
	if _, err := MergeOwnerIdentity(o, strPtr(""), strPtr("alex@example.com"), "other-salt", now); err != nil {
		t.Errorf("clear + re-enter under new salt: %v", err)
	}
}

// memOwnerStore is an in-memory OwnerStore for UpdateOwner tests.
type memOwnerStore struct {
	owner   *model.OwnerIdentity
	saveErr error
	saves   int
}

func (s *memOwnerStore) Owner() *model.OwnerIdentity {
	if s.owner == nil {
		return nil
	}
	cp := *s.owner
	return &cp
}

func (s *memOwnerStore) SetOwner(o *model.OwnerIdentity) error {
	s.saves++
	if s.saveErr != nil {
		return s.saveErr
	}
	s.owner = o
	return nil
}

func lastEvent(t *testing.T, sub *mockSubmitter) AuthEvent {
	t.Helper()
	sub.mu.Lock()
	defer sub.mu.Unlock()
	if len(sub.calls) == 0 {
		t.Fatal("no events submitted")
	}
	return sub.calls[len(sub.calls)-1].Event
}

func TestUpdateOwner_LogsEventWithOldAndNewHashes(t *testing.T) {
	sub := &mockSubmitter{}
	b, _ := newTestBuilder(t, sub)
	store := &memOwnerStore{}

	// First set: no old hashes.
	if _, changed, err := UpdateOwner(store, b, strPtr(vecEmployeeInput), nil, ""); err != nil || !changed {
		t.Fatalf("first set: changed=%v err=%v", changed, err)
	}
	evt := lastEvent(t, sub)
	if evt.OperationType != OpOwnerIdentifierUpdate {
		t.Errorf("operation = %q", evt.OperationType)
	}
	if evt.OwnerEmployeeIDHash != vecEmployeeSHA || evt.OldOwnerEmployeeIDHash != "" {
		t.Errorf("first set event = %+v", evt)
	}
	if evt.LabelHash != emptyLabelHash {
		t.Errorf("owner update must not reference a credential; label_hash = %s", evt.LabelHash)
	}

	// Change: old hash recorded.
	if _, _, err := UpdateOwner(store, b, strPtr("acme-emp-99999"), strPtr("alex@example.com"), ""); err != nil {
		t.Fatal(err)
	}
	evt = lastEvent(t, sub)
	if evt.OldOwnerEmployeeIDHash != vecEmployeeSHA || evt.OwnerEmailHash != vecEmailSHA {
		t.Errorf("change event = %+v", evt)
	}

	// Clear: new fields empty, old fields and old scheme set.
	if _, _, err := UpdateOwner(store, b, strPtr(""), strPtr(""), ""); err != nil {
		t.Fatal(err)
	}
	evt = lastEvent(t, sub)
	if evt.OwnerHash() != "" || evt.OwnerHashScheme != "" || evt.OldOwnerEmailHash != vecEmailSHA || evt.OldOwnerHashScheme != OwnerHashSchemeSHA256 {
		t.Errorf("clear event = %+v", evt)
	}
	if store.owner != nil {
		t.Errorf("store should be cleared, got %+v", store.owner)
	}
}

func TestUpdateOwner_NoChangeNoEvent(t *testing.T) {
	sub := &mockSubmitter{}
	b, _ := newTestBuilder(t, sub)
	store := &memOwnerStore{}
	_, _, _ = UpdateOwner(store, b, strPtr(vecEmployeeInput), nil, "")
	before := sub.CallCount()
	saves := store.saves

	_, changed, err := UpdateOwner(store, b, strPtr("  "+vecEmployeeInput), nil, "")
	if err != nil || changed {
		t.Fatalf("same value: changed=%v err=%v", changed, err)
	}
	if sub.CallCount() != before || store.saves != saves {
		t.Error("an unchanged identity must not be saved or logged")
	}
}

func TestUpdateOwner_SaveFailureLogsNothing(t *testing.T) {
	sub := &mockSubmitter{}
	b, _ := newTestBuilder(t, sub)
	store := &memOwnerStore{saveErr: errors.New("disk full")}

	_, _, err := UpdateOwner(store, b, strPtr(vecEmployeeInput), nil, "")
	if !errors.Is(err, ErrOwnerSave) || !errors.Is(err, store.saveErr) {
		t.Fatalf("err = %v, want ErrOwnerSave wrapping the store error", err)
	}
	if !strings.HasPrefix(err.Error(), "saving owner identifiers: ") {
		t.Errorf("err message = %q", err.Error())
	}
	if sub.CallCount() != 0 {
		t.Error("an event must not be logged for a change that was not saved")
	}
}

func TestUpdateOwner_NilBuilder(t *testing.T) {
	store := &memOwnerStore{}
	if _, changed, err := UpdateOwner(store, nil, strPtr(vecEmployeeInput), nil, ""); err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	if store.owner.EmployeeIDHash != vecEmployeeSHA {
		t.Error("identity should be saved without a builder")
	}
}

func TestEventBuilder_StampsOwnerFromSource(t *testing.T) {
	sub := &mockSubmitter{}
	b, _ := newTestBuilder(t, sub)
	owner := &model.OwnerIdentity{EmployeeIDHash: vecEmployeeSHA, EmailHash: vecEmailSHA, HashScheme: OwnerHashSchemeSHA256}
	b.OwnerSource = func() *model.OwnerIdentity { return owner }

	_ = b.LogEvent("totp", "GitHub", "github.com", "host", true)
	evt := lastEvent(t, sub)
	if evt.OwnerEmployeeIDHash != vecEmployeeSHA || evt.OwnerEmailHash != vecEmailSHA || evt.OwnerHashScheme != OwnerHashSchemeSHA256 {
		t.Errorf("owner not stamped: %+v", evt)
	}

	// Events logged after an edit pick up the new identity immediately.
	owner = nil
	_ = b.LogEvent("totp", "GitHub", "github.com", "host", true)
	if evt := lastEvent(t, sub); evt.OwnerHash() != "" {
		t.Errorf("owner should be empty after clearing, got %+v", evt)
	}
}

func TestEventBuilder_OwnerSourcePanicIsContained(t *testing.T) {
	sub := &mockSubmitter{}
	b, _ := newTestBuilder(t, sub)
	b.OwnerSource = func() *model.OwnerIdentity { panic("boom") }
	if err := b.LogEvent("totp", "GitHub", "", "", true); err != nil {
		t.Fatal(err)
	}
	if sub.CallCount() != 1 {
		t.Error("event should still be submitted when OwnerSource panics")
	}
}

// TestAuthEvent_JSONUnchangedWithoutOwner guards backward compatibility: an
// event without owner fields must serialize exactly as before the fields were
// added, so content hashes of existing and owner-less events are unaffected.
func TestAuthEvent_JSONUnchangedWithoutOwner(t *testing.T) {
	evt := AuthEvent{
		EventID:       "tegata-test",
		Timestamp:     time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		OperationType: "hotp",
		LabelHash:     "lhash",
		ServiceHash:   "shash",
		HostHash:      "hhash",
		Success:       true,
	}
	got, err := json.Marshal(evt)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"event_id":"tegata-test","timestamp":"2024-01-01T00:00:00Z","operation_type":"hotp","label_hash":"lhash","service_hash":"shash","host_hash":"hhash","success":true,"prev_hash":""}`
	if string(got) != want {
		t.Errorf("JSON changed:\n got %s\nwant %s", got, want)
	}
}

func TestApplyOwner(t *testing.T) {
	var cfg config.AuditConfig
	ApplyOwner(&cfg, &model.OwnerIdentity{EmailHash: vecEmailSHA, HashScheme: OwnerHashSchemeSHA256})
	if cfg.OwnerHash != vecEmailSHA || cfg.OwnerHashScheme != OwnerHashSchemeSHA256 {
		t.Errorf("ApplyOwner: %+v", cfg)
	}
	ApplyOwner(&cfg, nil)
	if cfg.OwnerHash != "" || cfg.OwnerHashScheme != "" {
		t.Errorf("ApplyOwner(nil) should clear: %+v", cfg)
	}
}

func TestShortOwnerHash(t *testing.T) {
	if got := ShortOwnerHash(vecEmployeeSHA); got != vecEmployeeSHA[:12] {
		t.Errorf("got %s", got)
	}
	if got := ShortOwnerHash(""); got != "" {
		t.Errorf("got %q", got)
	}
}
