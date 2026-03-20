package auth

import (
	"testing"

	"github.com/josh-wong/tegata/internal/errors"
	"github.com/josh-wong/tegata/pkg/model"
)

func TestGetStaticPassword_Valid(t *testing.T) {
	cred := &model.Credential{
		Type:   model.CredentialStatic,
		Secret: "my-super-secret-password",
	}
	pw, err := GetStaticPassword(cred)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(pw) != "my-super-secret-password" {
		t.Errorf("got %q, want %q", pw, "my-super-secret-password")
	}
}

func TestGetStaticPassword_WrongType(t *testing.T) {
	cred := &model.Credential{
		Type:   model.CredentialTOTP,
		Secret: "some-secret",
	}
	_, err := GetStaticPassword(cred)
	if err == nil {
		t.Fatal("expected error for non-static credential")
	}
	if !errors.Is(err, errors.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}
