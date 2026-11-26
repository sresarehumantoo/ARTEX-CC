package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const ownerLabelKeyFile = "owner-label.key"

// OwnerLabelKey returns the key for hashing claim owners into labels, stored
// in dataDir so every process sharing the data directory shows the same
// labels. The first call creates it.
func OwnerLabelKey(dataDir string) ([]byte, error) {
	return loadOrCreateKey(dataDir, ownerLabelKeyFile)
}

func loadOrCreateKey(dataDir, name string) ([]byte, error) {
	path := filepath.Join(dataDir, name)
	key, err := readKey(path)
	if err == nil || !errors.Is(err, os.ErrNotExist) {
		return key, err
	}
	if err := os.MkdirAll(dataDir, 0o711); err != nil {
		return nil, err
	}
	key = make([]byte, 32)
	_, _ = rand.Read(key)
	tmp, err := os.CreateTemp(dataDir, name+".*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	_, err = tmp.WriteString(hex.EncodeToString(key) + "\n")
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return nil, err
	}
	// Linking publishes the complete file atomically, and a concurrent first
	// start that lost the race reads the winner's key instead.
	if err := os.Link(tmp.Name(), path); err != nil {
		if errors.Is(err, os.ErrExist) {
			return readKey(path)
		}
		return nil, err
	}
	return key, nil
}

func readKey(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	key, err := hex.DecodeString(strings.TrimSpace(string(data)))
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("%s: want 64 hex characters", path)
	}
	return key, nil
}
