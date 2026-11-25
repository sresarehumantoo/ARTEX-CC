package config

// IdentityKeyFile is the name of the identity signing key inside the data
// directory. It is separate from the owner label key so rotating it revokes
// identity tokens without changing any claim label.
const IdentityKeyFile = "identity.key"

// IdentityKey returns the key that signs identity tokens, creating it on the
// first call.
func IdentityKey(dataDir string) ([]byte, error) {
	return loadOrCreateKey(dataDir, IdentityKeyFile)
}
