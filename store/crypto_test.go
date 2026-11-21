package store

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"testing"
)

func TestEncryptDecryptRoundtrip(t *testing.T) {
	t.Parallel()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}

	plaintext := "super-secret-password-123"
	encrypted, err := aesGCMEncrypt(key, plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if encrypted == plaintext {
		t.Error("encrypted should differ from plaintext")
	}

	decrypted, err := aesGCMDecrypt(key, encrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if decrypted != plaintext {
		t.Errorf("decrypted = %q, want %q", decrypted, plaintext)
	}
}

func TestDecryptPlaintext(t *testing.T) {
	t.Parallel()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}

	_, err := aesGCMDecrypt(key, "not-hex")
	if err == nil {
		t.Error("expected error decrypting non-hex string")
	}
}

func TestWrongKey(t *testing.T) {
	t.Parallel()
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	if _, err := rand.Read(key1); err != nil {
		t.Fatal(err)
	}
	if _, err := rand.Read(key2); err != nil {
		t.Fatal(err)
	}

	encrypted, err := aesGCMEncrypt(key1, "secret")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	_, err = aesGCMDecrypt(key2, encrypted)
	if err == nil {
		t.Error("expected error decrypting with wrong key")
	}
}

func TestEncPrefix(t *testing.T) {
	t.Parallel()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}

	encrypted, err := aesGCMEncrypt(key, "test-value")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	prefixed := "enc:" + encrypted
	if !strings.HasPrefix(prefixed, "enc:") {
		t.Error("prefixed value should start with enc:")
	}

	_, err = hex.DecodeString(encrypted)
	if err != nil {
		t.Errorf("encrypted value should be valid hex: %v", err)
	}
}
