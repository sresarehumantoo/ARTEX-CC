package config

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
)

// IdentityKeyFile is the name of the identity signing key inside the data
// directory. It is separate from the owner label key so rotating it revokes
// identity tokens without changing any claim label.
const IdentityKeyFile = "identity.key"

// IdentityKey returns the key that signs identity tokens, creating it on the
// first call.
func IdentityKey(dataDir string) ([]byte, error) {
	return loadOrCreateKey(dataDir, IdentityKeyFile)
}

// RotateIdentityKey replaces the identity key with a new random one and
// returns the previous key (nil when there was none or it was unreadable)
// and the new one. The new file is renamed into place, so running signers
// see a different inode and reload it.
func RotateIdentityKey(dataDir string) (prev, next []byte, err error) {
	path := filepath.Join(dataDir, IdentityKeyFile)
	prev, _ = readKey(path)
	next = make([]byte, 32)
	_, _ = rand.Read(next)
	tmp, err := os.CreateTemp(dataDir, IdentityKeyFile+".*")
	if err != nil {
		return nil, nil, err
	}
	defer os.Remove(tmp.Name())
	_, err = tmp.WriteString(hex.EncodeToString(next) + "\n")
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return nil, nil, err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return nil, nil, err
	}
	return prev, next, nil
}
