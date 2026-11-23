package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/josh-wong/tegata/internal/audit"
	"github.com/josh-wong/tegata/internal/errors"
	"github.com/josh-wong/tegata/internal/i18n"
	"github.com/josh-wong/tegata/internal/vault"
)

func runOwnerCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newOwnerCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func vaultOwner(t *testing.T, path string) (employeeIDHash, emailHash string) {
	t.Helper()
	m, err := vault.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if err := m.Unlock([]byte("integration-test-passphrase")); err != nil {
		t.Fatal(err)
	}
	o := m.Owner()
	if o == nil {
		return "", ""
	}
	return o.EmployeeIDHash, o.EmailHash
}

func TestOwnerCmd_SetShowClear(t *testing.T) {
	path, _ := createIntegrationVault(t)
	t.Setenv("TEGATA_VAULT", path)
	t.Setenv("TEGATA_PASSPHRASE", "integration-test-passphrase")

	if _, err := runOwnerCmd(t, "set", "--employee-id", "acme-emp-12345", "--email", "Alex@Example.com"); err != nil {
		t.Fatalf("set: %v", err)
	}
	id, email := vaultOwner(t, path)
	if id != audit.HashString("acme-emp-12345") || email != audit.HashString("alex@example.com") {
		t.Errorf("stored hashes = %s, %s", id, email)
	}

	out, err := runOwnerCmd(t, "show")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if !bytes.Contains([]byte(out), []byte(audit.ShortOwnerHash(id))) {
		t.Errorf("show output missing short hash:\n%s", out)
	}
	if bytes.Contains([]byte(out), []byte("acme-emp-12345")) || bytes.Contains([]byte(out), []byte(id)) {
		t.Errorf("show must print only the short hash, never the raw value or full hash:\n%s", out)
	}

	if _, err := runOwnerCmd(t, "clear", "--email"); err != nil {
		t.Fatalf("clear --email: %v", err)
	}
	id, email = vaultOwner(t, path)
	if id == "" || email != "" {
		t.Errorf("after clear --email: id=%q email=%q", id, email)
	}

	if _, err := runOwnerCmd(t, "clear"); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if id, email = vaultOwner(t, path); id != "" || email != "" {
		t.Errorf("after clear: id=%q email=%q", id, email)
	}
}

func TestOwnerCmd_UsesOrganizationSalt(t *testing.T) {
	path, _ := createIntegrationVault(t)
	t.Setenv("TEGATA_VAULT", path)
	t.Setenv("TEGATA_PASSPHRASE", "integration-test-passphrase")
	toml := "[organization]\nidentifier_salt = \"org-salt-1\"\n"
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), "tegata.toml"), []byte(toml), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := runOwnerCmd(t, "set", "--employee-id", "acme-emp-12345"); err != nil {
		t.Fatalf("set: %v", err)
	}
	id, _ := vaultOwner(t, path)
	if id != audit.HashOwnerIdentifier("acme-emp-12345", "org-salt-1") {
		t.Errorf("expected HMAC hash, got %s", id)
	}
}

func TestOwnerCmd_InvalidInput(t *testing.T) {
	path, _ := createIntegrationVault(t)
	t.Setenv("TEGATA_VAULT", path)
	t.Setenv("TEGATA_PASSPHRASE", "integration-test-passphrase")

	for _, args := range [][]string{
		{"set", "--email", "not-an-email"},
		{"set", "--employee-id", "  "},
	} {
		if _, err := runOwnerCmd(t, args...); !errors.Is(err, errors.ErrInvalidInput) {
			t.Errorf("%v: err = %v, want ErrInvalidInput", args, err)
		}
	}
	if id, email := vaultOwner(t, path); id != "" || email != "" {
		t.Error("invalid input must not change the stored identity")
	}
}

func TestOwnerErrorMessage(t *testing.T) {
	_, err := audit.MergeOwnerIdentity(nil, nil, strPtr("not-an-email"), "", time.Now())
	if got := ownerErrorMessage(err); got != i18n.T("cmd.owner.error.emailMissingAt") {
		t.Errorf("missing @: got %q", got)
	}
	_, err = audit.MergeOwnerIdentity(nil, strPtr("acme\x00emp"), nil, "", time.Now())
	want := i18n.Tf("cmd.owner.error.controlChars", map[string]any{"Field": i18n.T("cmd.owner.error.field.employeeID")})
	if got := ownerErrorMessage(err); got != want {
		t.Errorf("control characters: got %q, want %q", got, want)
	}
	if got := ownerErrorMessage(errors.New("other")); got != "other" {
		t.Errorf("unrecognized error: got %q, want it passed through", got)
	}
}

func TestOwnerFieldInput(t *testing.T) {
	if ownerFieldInput("  ") != nil {
		t.Error("blank should keep (nil)")
	}
	if p := ownerFieldInput(" - "); p == nil || *p != "" {
		t.Error("- should clear")
	}
	if p := ownerFieldInput(" acme "); p == nil || *p != "acme" {
		t.Error("value should be trimmed and replaced")
	}
}

func strPtr(s string) *string { return &s }
