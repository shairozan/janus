package runlog

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// SignerIdentity is an authorized signer: who they are, and the key that proves it.
type SignerIdentity struct {
	Fingerprint  string
	Email        string
	PublicKeyPEM string
}

// TrustStore answers the only question that matters at verification time: is this
// signing key authorized?
//
// Before this existed, verification used the public key the record supplied about
// ITSELF, which means a record vouched for its own authenticity — an attacker
// signs a fabricated record with a keypair they generated and it renders as valid.
// A signature is only meaningful relative to a key you already trust.
type TrustStore interface {
	Trusted(fingerprint string) bool
	Describe(fingerprint string) (SignerIdentity, bool)
}

// KeySet is a TrustStore over a fixed set of authorized signers. Where the keys
// come from is the caller's business — that is precisely the seam that lets the
// commercial build anchor trust in a license and an open-source build anchor it in
// a user-managed keyring, with one verification path serving both.
type KeySet struct {
	byFingerprint map[string]SignerIdentity
}

// NewKeySet builds a trust store over the given identities. A KeySet with no
// identities trusts nothing — an empty trust store is not a permissive one.
func NewKeySet(identities ...SignerIdentity) *KeySet {
	byFingerprint := make(map[string]SignerIdentity, len(identities))
	for _, identity := range identities {
		byFingerprint[identity.Fingerprint] = identity
	}

	return &KeySet{byFingerprint: byFingerprint}
}

// Trusted reports whether the fingerprint belongs to an authorized signer.
func (k *KeySet) Trusted(fingerprint string) bool {
	if fingerprint == "" {
		return false
	}

	_, ok := k.byFingerprint[fingerprint]

	return ok
}

// Describe returns the identity behind a fingerprint.
func (k *KeySet) Describe(fingerprint string) (SignerIdentity, bool) {
	identity, ok := k.byFingerprint[fingerprint]

	return identity, ok
}

// fingerprintOfPEM computes the SHA-256 fingerprint of a PEM-encoded public key.
// It must agree with signing.Signer.PublicKeyFingerprint, which fingerprints the
// DER bytes of the PKIX encoding.
func fingerprintOfPEM(publicKeyPEM string) (string, error) {
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return "", fmt.Errorf("no PEM block found in public key")
	}

	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parsing public key: %w", err)
	}

	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", fmt.Errorf("re-encoding public key: %w", err)
	}

	sum := sha256.Sum256(der)

	return hex.EncodeToString(sum[:]), nil
}

// NewLicenseTrust builds the commercial trust anchor from the license's
// SigningPublicKey claim — the value ValidateSigningKeyPair already verifies at
// startup and, until now, threw away.
func NewLicenseTrust(publicKeyPEM, email string) (*KeySet, error) {
	fingerprint, err := fingerprintOfPEM(publicKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("license signing key: %w", err)
	}

	return NewKeySet(SignerIdentity{
		Fingerprint:  fingerprint,
		Email:        email,
		PublicKeyPEM: publicKeyPEM,
	}), nil
}

// keyringFile is the on-disk shape of the open-source trust anchor.
type keyringFile struct {
	Keys []struct {
		Email     string `yaml:"email"`
		PublicKey string `yaml:"public_key"`
	} `yaml:"keys"`
}

// NewKeyringTrust builds the open-source trust anchor from a user- or
// org-maintained keyring. This is the seam that lets Janus drop its license server
// without losing the meaning of a signature.
func NewKeyringTrust(path string) (*KeySet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading keyring %s: %w", path, err)
	}

	var parsed keyringFile
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("parsing keyring %s: %w", path, err)
	}

	identities := make([]SignerIdentity, 0, len(parsed.Keys))

	for i, key := range parsed.Keys {
		fingerprint, err := fingerprintOfPEM(key.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("keyring entry %d (%s): %w", i, key.Email, err)
		}

		identities = append(identities, SignerIdentity{
			Fingerprint:  fingerprint,
			Email:        key.Email,
			PublicKeyPEM: key.PublicKey,
		})
	}

	return NewKeySet(identities...), nil
}
