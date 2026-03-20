package auth

import (
	"fmt"

	"github.com/josh-wong/tegata/internal/errors"
	"github.com/josh-wong/tegata/pkg/model"
)

// GetStaticPassword returns a zeroable copy of the secret for a static
// credential. Callers must zero the returned slice after use. It returns
// ErrInvalidInput if the credential is not of type static.
func GetStaticPassword(cred *model.Credential) ([]byte, error) {
	if cred.Type != model.CredentialStatic {
		return nil, fmt.Errorf("credential %q is type %s, not static: %w", cred.Label, cred.Type, errors.ErrInvalidInput)
	}
	return []byte(cred.Secret), nil
}
