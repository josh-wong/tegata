package vault

import (
	"github.com/josh-wong/tegata/pkg/model"
)

// Marshal serializes a VaultHeader into exactly 128 bytes using big-endian
// encoding at explicit byte offsets.
func Marshal(_ *model.VaultHeader) ([]byte, error) {
	panic("not implemented")
}

// Unmarshal deserializes 128 bytes into a VaultHeader. Returns
// ErrVaultCorrupt if the magic bytes do not match.
func Unmarshal(_ []byte) (*model.VaultHeader, error) {
	panic("not implemented")
}
