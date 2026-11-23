package main

import (
	"path/filepath"
	"testing"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/vault"
)

func newOwnerTestApp(t *testing.T) (*App, *recordingSubmitter) {
	t.Helper()
	vaultPath := setupTestVault(t)
	app := NewApp()
	mgr, err := vault.Open(vaultPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.Unlock([]byte(testPassphrase)); err != nil {
		mgr.Close()
		t.Fatal(err)
	}
	app.vault = mgr
	app.vaultPath = vaultPath

	rec := &recordingSubmitter{}
	key := make([]byte, 32)
	builder, err := audit.NewEventBuilder(rec, filepath.Join(t.TempDir(), "queue.tegata"), key, 100)
	if err != nil {
		t.Fatal(err)
	}
	builder.OwnerSource = mgr.Owner
	app.builder = builder
	t.Cleanup(app.LockVault)
	return app, rec
}

func TestAdapter_SetOwner_SavesAndLogs(t *testing.T) {
	app, rec := newOwnerTestApp(t)

	info, err := app.SetOwner("acme-emp-12345", "", false, false)
	if err != nil {
		t.Fatalf("SetOwner: %v", err)
	}
	full := audit.HashString("acme-emp-12345")
	if info.EmployeeIDHash != audit.ShortOwnerHash(full) || info.EmailHash != "" {
		t.Errorf("OwnerInfo = %+v", info)
	}
	if got := app.config.Audit.OwnerHash; got != full {
		t.Errorf("runtime owner hash for Docker labels = %q, want %q", got, full)
	}

	entries := rec.snapshot()
	if len(entries) != 1 || entries[0].Event.OperationType != audit.OpOwnerIdentifierUpdate {
		t.Fatalf("expected one owner-identifier-update event, got %+v", entries)
	}

	// Later events carry the owner hash.
	if _, err := app.AddCredential("gh", "GitHub", "totp", "JBSWY3DPEHPK3PXP", "SHA1", 6, 30, nil, ""); err != nil {
		t.Fatal(err)
	}
	entries = rec.snapshot()
	if got := entries[len(entries)-1].Event.OwnerEmployeeIDHash; got != full {
		t.Errorf("credential-add owner hash = %q, want %q", got, full)
	}
}

func TestAdapter_SetOwner_KeepAndClear(t *testing.T) {
	app, _ := newOwnerTestApp(t)
	if _, err := app.SetOwner("acme-emp-12345", "alex@example.com", false, false); err != nil {
		t.Fatal(err)
	}
	// Empty email with clear unset keeps it; clearEmployeeID removes the ID.
	info, err := app.SetOwner("", "", true, false)
	if err != nil {
		t.Fatal(err)
	}
	if info.EmployeeIDHash != "" || info.EmailHash != audit.ShortOwnerHash(audit.HashString("alex@example.com")) {
		t.Errorf("OwnerInfo = %+v", info)
	}
}

func TestAdapter_SetOwner_SaltChangedCode(t *testing.T) {
	app, _ := newOwnerTestApp(t)
	if _, err := app.SetOwner("acme-emp-12345", "alex@example.com", false, false); err != nil {
		t.Fatal(err)
	}
	app.config.Organization.IdentifierSalt = "new-salt"
	_, err := app.SetOwner("acme-emp-12345", "", false, false)
	if err == nil || err.Error() != ErrOwnerSaltChangedCode {
		t.Errorf("err = %v, want %q", err, ErrOwnerSaltChangedCode)
	}
}

func TestAdapter_OwnerLocked(t *testing.T) {
	app := NewApp()
	if _, err := app.GetOwner(); err == nil {
		t.Error("GetOwner on locked app should fail")
	}
	if _, err := app.SetOwner("x", "", false, false); err == nil {
		t.Error("SetOwner on locked app should fail")
	}
}

// TestAdapter_SetOwner_EmptySaveIsNoOp verifies that saving with nothing
// entered and nothing cleared reports no change, logs no event, and does not
// report a salt change.
func TestAdapter_SetOwner_EmptySaveIsNoOp(t *testing.T) {
	app, rec := newOwnerTestApp(t)
	info, err := app.SetOwner("acme-emp-12345", "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Changed {
		t.Error("first save should report Changed")
	}
	before := len(rec.snapshot())

	info, err = app.SetOwner("", "", false, false)
	if err != nil || info.Changed {
		t.Fatalf("empty save: info = %+v, err = %v; want unchanged, nil", info, err)
	}
	if info.EmployeeIDHash != audit.ShortOwnerHash(audit.HashString("acme-emp-12345")) {
		t.Errorf("empty save should return the current owner, got %+v", info)
	}

	app.config.Organization.IdentifierSalt = "new-salt"
	if info, err = app.SetOwner("", "", false, false); err != nil || info.Changed {
		t.Errorf("empty save after salt change: info = %+v, err = %v; want unchanged, nil", info, err)
	}
	if got := len(rec.snapshot()); got != before {
		t.Errorf("empty saves logged %d events, want 0", got-before)
	}
}

// TestAdapter_SetOwner_SameValueUnchanged verifies that re-entering the
// current identifier reports no change.
func TestAdapter_SetOwner_SameValueUnchanged(t *testing.T) {
	app, _ := newOwnerTestApp(t)
	if _, err := app.SetOwner("acme-emp-12345", "", false, false); err != nil {
		t.Fatal(err)
	}
	info, err := app.SetOwner("acme-emp-12345", "", false, false)
	if err != nil || info.Changed {
		t.Errorf("info = %+v, err = %v; want unchanged, nil", info, err)
	}
}
