package publickey

import (
	"crypto/rsa"
	"crypto/x509"
	"embed"
	"encoding/pem"
	"fmt"
)

// GetPublicKeys returns all embedded RSA public keys parsed from the PEM file in the
// provided embed.FS. The file may contain multiple concatenated PEM blocks (historical
// keys), and validation tries each one. Returns an error containing "no public key
// embedded" when the file is missing, empty, or contains no usable keys, so callers
// (e.g. the executor) can detect dev-mode builds and skip license validation.
func GetPublicKeys(assets embed.FS) ([]*rsa.PublicKey, error) {
	keyData, err := assets.ReadFile(".license_public_key.pem")
	if err != nil {
		return nil, fmt.Errorf("no public key embedded - build with .license_public_key.pem file: %w", err)
	}

	if len(keyData) == 0 {
		return nil, fmt.Errorf("no public key embedded - .license_public_key.pem is empty")
	}

	keys, err := ParsePublicKeysFromPEM(string(keyData))
	if err != nil {
		return nil, err
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("no public key embedded - .license_public_key.pem contains no RSA public keys")
	}

	return keys, nil
}

// GetPublicKey returns the first embedded RSA public key. It is retained for callers
// and debugging helpers that only need a single key; validation uses GetPublicKeys.
func GetPublicKey(assets embed.FS) (*rsa.PublicKey, error) {
	keys, err := GetPublicKeys(assets)
	if err != nil {
		return nil, err
	}

	return keys[0], nil
}

// ParsePublicKeysFromPEM decodes all RSA public keys from a PEM payload that may hold
// multiple concatenated blocks. Non-RSA-public-key blocks are skipped. It returns an
// error only if a PUBLIC KEY block fails to parse; an empty/blockless payload yields
// an empty slice with no error (callers decide whether that is acceptable).
func ParsePublicKeysFromPEM(publicKeyPEM string) ([]*rsa.PublicKey, error) {
	var keys []*rsa.PublicKey

	rest := []byte(publicKeyPEM)
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}

		// Skip blocks that aren't public keys (e.g. stray certificates/comments).
		if block.Type != "PUBLIC KEY" {
			continue
		}

		publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse public key: %w", err)
		}

		rsaPublicKey, ok := publicKey.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("not an RSA public key")
		}

		keys = append(keys, rsaPublicKey)
	}

	return keys, nil
}

// ParsePublicKeyFromPEM decodes a single RSA public key from PEM format. It returns the
// first key found and errors if none are present.
func ParsePublicKeyFromPEM(publicKeyPEM string) (*rsa.PublicKey, error) {
	keys, err := ParsePublicKeysFromPEM(publicKeyPEM)
	if err != nil {
		return nil, err
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	return keys[0], nil
}

// IsEmbedded returns true if a real public key has been embedded in the assets.
func IsEmbedded(assets embed.FS) bool {
	_, err := assets.ReadFile(".license_public_key.pem")

	return err == nil
}

// GetKeyInfo returns information about the embedded key(s) for debugging.
func GetKeyInfo(assets embed.FS) string {
	if !IsEmbedded(assets) {
		return "No public key embedded (dev mode)"
	}

	keys, err := GetPublicKeys(assets)
	if err != nil {
		return fmt.Sprintf("Invalid embedded key: %v", err)
	}

	if len(keys) == 1 {
		return fmt.Sprintf("RSA-%d public key embedded", keys[0].N.BitLen())
	}

	return fmt.Sprintf("%d public keys embedded (RSA-%d primary)", len(keys), keys[0].N.BitLen())
}
