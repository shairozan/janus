package publickey

import (
	"crypto/rsa"
	"crypto/x509"
	"embed"
	"encoding/pem"
	"fmt"
)

// GetPublicKey returns the embedded RSA public key parsed from PEM format from the provided embed.FS.
// Returns an error if the key file doesn't exist or is invalid.
func GetPublicKey(assets embed.FS) (*rsa.PublicKey, error) {
	// Read the embedded public key file
	keyData, err := assets.ReadFile(".license_public_key.pem")
	if err != nil {
		return nil, fmt.Errorf("no public key embedded - build with .license_public_key.pem file: %w", err)
	}

	publicKeyPEM := string(keyData)
	if publicKeyPEM == "" {
		return nil, fmt.Errorf("embedded public key is empty")
	}

	return ParsePublicKeyFromPEM(publicKeyPEM)
}

// ParsePublicKeyFromPEM decodes an RSA public key from PEM format.
func ParsePublicKeyFromPEM(publicKeyPEM string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	rsaPublicKey, ok := publicKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}

	return rsaPublicKey, nil
}

// IsEmbedded returns true if a real public key has been embedded in the assets.
func IsEmbedded(assets embed.FS) bool {
	_, err := assets.ReadFile(".license_public_key.pem")

	return err == nil
}

// GetKeyInfo returns information about the embedded key for debugging.
func GetKeyInfo(assets embed.FS) string {
	if !IsEmbedded(assets) {
		return "No public key embedded (dev mode)"
	}

	publicKey, err := GetPublicKey(assets)
	if err != nil {
		return fmt.Sprintf("Invalid embedded key: %v", err)
	}

	return fmt.Sprintf("RSA-%d public key embedded", publicKey.N.BitLen())
}
