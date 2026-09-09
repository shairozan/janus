package signing

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

// credentialService is the service name Janus registers under in the OS
// credential store. Keys are addressed by (credentialService, identity), so one
// install can hold a key per identity and `janus keys` can find it again.
const credentialService = "janus-signing"

// KeyBits is the RSA key size `janus keys generate` produces.
//
// 2048 rather than 4096 deliberately: Windows Credential Manager caps a
// credential blob at 2560 bytes, and a 4096-bit PKCS#1 private key PEM is around
// 3.2KB — it cannot be stored there at all. A 2048-bit key PEM is about 1.7KB and
// fits with room to spare. RSA-2048 is also what every previously issued Janus
// signing key uses, so existing records stay verifiable alongside new ones.
const KeyBits = 2048

// ErrNoStoredKey reports that the credential store holds no key for an identity.
// Callers distinguish this from a store that is unavailable or locked, because
// the two need different advice: generate a key, versus fix the environment.
var ErrNoStoredKey = errors.New("no signing key in the credential store for this identity")

// GenerateKey creates a new RSA signing key. See KeyBits for the size rationale.
func GenerateKey() (*rsa.PrivateKey, error) {
	key, err := rsa.GenerateKey(rand.Reader, KeyBits)
	if err != nil {
		return nil, fmt.Errorf("generating RSA key: %w", err)
	}

	return key, nil
}

// EncodePrivateKeyPEM renders a private key as PKCS#1 PEM, the form
// ParsePrivateKeyFromPEM reads back.
func EncodePrivateKeyPEM(key *rsa.PrivateKey) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
}

// EncodePublicKeyPEM renders the public half as PKIX PEM — the form the trust
// keyring stores and `janus keys export-public` prints.
func EncodePublicKeyPEM(key *rsa.PrivateKey) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return "", fmt.Errorf("marshalling public key: %w", err)
	}

	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})), nil
}

// StoreKey writes a private key into the OS credential store under identity.
//
// It overwrites any key already held for that identity, so callers that must not
// clobber an existing key check with LoadKey first — silently replacing a signing
// key would orphan every record signed with the old one.
func StoreKey(identity string, privateKeyPEM []byte) error {
	if identity == "" {
		return errors.New("identity is required to store a signing key")
	}

	if err := keyring.Set(credentialService, identity, string(privateKeyPEM)); err != nil {
		return fmt.Errorf("writing the signing key to the credential store: %w", err)
	}

	return nil
}

// LoadKey reads the private key PEM held for identity. It returns ErrNoStoredKey
// when the store is reachable but holds nothing for that identity.
func LoadKey(identity string) ([]byte, error) {
	if identity == "" {
		return nil, errors.New("identity is required to load a signing key")
	}

	secret, err := keyring.Get(credentialService, identity)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil, fmt.Errorf("%w: %s", ErrNoStoredKey, identity)
		}

		return nil, fmt.Errorf("reading the signing key from the credential store: %w", err)
	}

	return []byte(secret), nil
}

// DeleteKey removes the key held for identity. Deleting a key that is not there
// is not an error — the caller's goal is "no key under this identity", which is
// already true.
func DeleteKey(identity string) error {
	if identity == "" {
		return errors.New("identity is required to delete a signing key")
	}

	if err := keyring.Delete(credentialService, identity); err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil
		}

		return fmt.Errorf("deleting the signing key from the credential store: %w", err)
	}

	return nil
}

// NewSignerFromCredentialStore builds a Signer from the key held for identity.
func NewSignerFromCredentialStore(identity string) (*Signer, error) {
	keyPEM, err := LoadKey(identity)
	if err != nil {
		return nil, err
	}

	privateKey, err := ParsePrivateKeyFromPEM(keyPEM)
	if err != nil {
		return nil, fmt.Errorf("parsing the stored signing key: %w", err)
	}

	return &Signer{privateKey: privateKey}, nil
}

// NewSignerFromKey wraps an already-parsed key, for callers that have just
// generated or imported one and should not round-trip it through the store.
func NewSignerFromKey(key *rsa.PrivateKey) *Signer {
	return &Signer{privateKey: key}
}
