package vault

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	tegerrors "github.com/josh-wong/tegata/internal/errors"
	"github.com/josh-wong/tegata/pkg/model"
)

func unlockTestVault(t *testing.T, path string) *Manager {
	t.Helper()
	m, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := m.Unlock([]byte("test-passphrase")); err != nil {
		m.Close()
		t.Fatalf("Unlock: %v", err)
	}
	return m
}

func TestOwner_PersistsAcrossReopen(t *testing.T) {
	path, _ := createTestVault(t)
	m := unlockTestVault(t, path)
	want := &model.OwnerIdentity{
		EmployeeIDHash: "idhash",
		EmailHash:      "emailhash",
		HashScheme:     "hmac-sha256",
		SaltID:         "0123456789abcdef",
		UpdatedAt:      time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC),
	}
	if err := m.SetOwner(want); err != nil {
		t.Fatalf("SetOwner: %v", err)
	}
	m.Close()

	m = unlockTestVault(t, path)
	defer m.Close()
	got := m.Owner()
	if got == nil || *got != *want {
		t.Errorf("Owner after reopen = %+v, want %+v", got, want)
	}
}

func TestOwner_ReturnsCopy(t *testing.T) {
	path, _ := createTestVault(t)
	m := unlockTestVault(t, path)
	defer m.Close()
	_ = m.SetOwner(&model.OwnerIdentity{EmployeeIDHash: "idhash"})
	got := m.Owner()
	got.EmployeeIDHash = "mutated"
	if m.Owner().EmployeeIDHash != "idhash" {
		t.Error("Owner must return a copy")
	}
}

func TestOwner_ClearAndEmptyIdentity(t *testing.T) {
	path, _ := createTestVault(t)
	m := unlockTestVault(t, path)
	defer m.Close()
	if m.Owner() != nil {
		t.Fatal("new vault should have no owner")
	}
	_ = m.SetOwner(&model.OwnerIdentity{EmailHash: "emailhash"})
	if err := m.SetOwner(&model.OwnerIdentity{HashScheme: "sha256"}); err != nil {
		t.Fatal(err)
	}
	if m.Owner() != nil {
		t.Error("an identity with no hashes should clear the owner")
	}
}

func TestOwner_LockedVault(t *testing.T) {
	path, _ := createTestVault(t)
	m, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if m.Owner() != nil {
		t.Error("locked vault should report no owner")
	}
	if err := m.SetOwner(&model.OwnerIdentity{EmailHash: "x"}); !errors.Is(err, tegerrors.ErrVaultLocked) {
		t.Errorf("SetOwner on locked vault: err = %v", err)
	}
}

func TestOwner_SaveFailureRestoresPrevious(t *testing.T) {
	path, _ := createTestVault(t)
	m := unlockTestVault(t, path)
	defer m.Close()
	_ = m.SetOwner(&model.OwnerIdentity{EmployeeIDHash: "old"})

	// Remove the vault file so saveLocked fails when reading it back.
	if err := os.Rename(path, path+".moved"); err != nil {
		t.Fatal(err)
	}
	if err := m.SetOwner(&model.OwnerIdentity{EmployeeIDHash: "new"}); err == nil {
		t.Fatal("expected save failure")
	}
	if got := m.Owner(); got == nil || got.EmployeeIDHash != "old" {
		t.Errorf("in-memory owner after failed save = %+v, want old value", got)
	}
}

// TestOwner_PayloadJSONOmittedWhenUnset guards backward compatibility: vaults
// without an owner serialize without an "owner" key.
func TestOwner_PayloadJSONOmittedWhenUnset(t *testing.T) {
	b, err := json.Marshal(model.VaultPayload{Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), `"owner"`) {
		t.Errorf("payload JSON contains owner key: %s", b)
	}
}
