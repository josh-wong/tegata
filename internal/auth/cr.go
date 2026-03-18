package auth

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/josh-wong/tegata/internal/errors"
	"github.com/josh-wong/tegata/pkg/model"
)

// SignChallenge computes an HMAC over the raw challenge bytes using the secret
// bytes decoded from cred's stored secret. The algorithm is taken from
// cred.Algorithm: "SHA256" produces a 64-character lowercase hex string using
// HMAC-SHA256; anything else (including empty) defaults to HMAC-SHA1 producing
// a 40-character lowercase hex string. This default matches the SHA1 fallback
// in hashFuncFromAlgorithm used by the OTP engines.
//
// The challenge is treated as raw bytes — no hex decoding is applied. The
// caller defines the encoding of the challenge string (typically raw ASCII or
// UTF-8 as provided by the user on the command line).
//
// Neither the challenge nor the resulting HMAC response is logged at any slog
// level, as both are sensitive authentication values.
//
// Returns errors.ErrInvalidInput if cred.Type is not CredentialChallengeResponse.
func SignChallenge(cred *model.Credential, secret, challenge []byte) (string, error) {
	if cred.Type != model.CredentialChallengeResponse {
		return "", fmt.Errorf("credential %q has type %q, not challenge-response: %w",
			cred.Label, cred.Type, errors.ErrInvalidInput)
	}

	var mac []byte
	switch cred.Algorithm {
	case "SHA256":
		h := hmac.New(sha256.New, secret)
		h.Write(challenge)
		mac = h.Sum(nil)
	default:
		// Default to SHA1, matching the hashFuncFromAlgorithm convention in otp.go
		// where an empty or unrecognized algorithm falls back to SHA-1.
		h := hmac.New(sha1.New, secret)
		h.Write(challenge)
		mac = h.Sum(nil)
	}

	return hex.EncodeToString(mac), nil
}
