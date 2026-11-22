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

func TestDecryptCiphertextTooShort(t *testing.T) {
	t.Parallel()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	_, err := aesGCMDecrypt(key, hex.EncodeToString([]byte("short")))
	if err != errCiphertextTooShort {
		t.Errorf("got %v, want errCiphertextTooShort", err)
	}
}

func TestEncryptBadKeySize(t *testing.T) {
	t.Parallel()
	_, err := aesGCMEncrypt(make([]byte, 15), "data")
	if err == nil {
		t.Error("expected error for 15-byte key")
	}
}

func TestDecryptBadKeySize(t *testing.T) {
	t.Parallel()
	_, err := aesGCMDecrypt(make([]byte, 15), hex.EncodeToString(make([]byte, 32)))
	if err == nil {
		t.Error("expected error for 15-byte key")
	}
}

func TestEncryptEmptyString(t *testing.T) {
	t.Parallel()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	encrypted, err := aesGCMEncrypt(key, "")
	if err != nil {
		t.Fatalf("encrypt empty: %v", err)
	}
	decrypted, err := aesGCMDecrypt(key, encrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if decrypted != "" {
		t.Errorf("decrypted = %q, want empty string", decrypted)
	}
}

func TestParseCredentialKey(t *testing.T) {
	t.Parallel()

	validKey := hex.EncodeToString(make([]byte, 32))
	key, err := ParseCredentialKey(validKey)
	if err != nil {
		t.Fatalf("valid key: %v", err)
	}
	if len(key) != 32 {
		t.Errorf("key length = %d, want 32", len(key))
	}

	_, err = ParseCredentialKey("not-hex")
	if err == nil {
		t.Error("expected error for invalid hex")
	}

	_, err = ParseCredentialKey(hex.EncodeToString(make([]byte, 16)))
	if err == nil {
		t.Error("expected error for 16-byte key")
	}

	_, err = ParseCredentialKey("")
	if err == nil {
		t.Error("expected error for empty key")
	}
}
