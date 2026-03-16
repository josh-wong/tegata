package auth

import (
	"fmt"

	"github.com/josh-wong/tegata/internal/errors"
	"github.com/josh-wong/tegata/pkg/model"
)

// GetStaticPassword returns the secret for a static credential. It returns
// ErrInvalidInput if the credential is not of type static.
func GetStaticPassword(cred *model.Credential) (string, error) {
	if cred.Type != model.CredentialStatic {
		return "", fmt.Errorf("credential %q is type %s, not static: %w", cred.Label, cred.Type, errors.ErrInvalidInput)
	}
	return cred.Secret, nil
}
