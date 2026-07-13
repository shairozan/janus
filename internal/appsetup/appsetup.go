// Package appsetup holds the GUI-free bootstrap steps shared by the desktop app
// and the `janus mcp server` daemon: validating the license JWT against the
// embedded public keys, and constructing the run-log signer from config. Keeping
// these here means the daemon does not import internal/gui (and therefore no
// fyne) to perform the same startup wiring.
package appsetup

import (
	"embed"
	"fmt"
	"io"
	"os"

	"github.com/pharmalytica/janus/internal/config"
	"github.com/pharmalytica/janus/internal/license/validator"
	"github.com/pharmalytica/janus/internal/runlog"
	"github.com/pharmalytica/janus/internal/signing"
)

// ValidateLicenseReader reads a license JWT from r and validates it against the
// public keys embedded in assets, returning the claims.
func ValidateLicenseReader(r io.Reader, assets embed.FS) (*validator.Claims, error) {
	tokenBytes, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read license: %w", err)
	}

	v, err := validator.NewValidator(assets)
	if err != nil {
		return nil, fmt.Errorf("failed to create validator: %w", err)
	}

	claims, err := v.ValidateToken(string(tokenBytes))
	if err != nil {
		return nil, fmt.Errorf("invalid license token: %w", err)
	}

	return claims, nil
}

// ValidateLicensePath opens the license file at path and validates it.
func ValidateLicensePath(path string, assets embed.FS) (*validator.Claims, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open license file at %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	return ValidateLicenseReader(f, assets)
}

// BuildSigner constructs the run-log signer from cfg, validating the configured
// private key against the license's embedded public key for CFR 21 Part 11
// compliance. It returns (nil, nil) when signing is not configured (disabled),
// (signer, nil) when enabled and valid, and (nil, err) on a configuration or key
// mismatch. Callers decide how to surface the error (the GUI shows a dialog; the
// daemon logs and continues unsigned).
func BuildSigner(cfg *config.Config, claims *validator.Claims) (*signing.Signer, error) {
	var privateKeyPath string
	if cfg != nil && cfg.Signing.PrivateKeyPath != "" {
		expanded, err := config.ExpandSigningPrivateKeyPath(cfg.Signing.PrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("expanding signing private key path: %w", err)
		}

		privateKeyPath = expanded
	}

	// Validate key-pair consistency between config and license even when no key is
	// configured (the validator enforces the license's signing requirements).
	if claims != nil {
		if err := validator.ValidateSigningKeyPair(claims, privateKeyPath); err != nil {
			return nil, fmt.Errorf("signing key pair validation failed: %w", err)
		}
	}

	if privateKeyPath == "" {
		return nil, nil // signing disabled
	}

	signer, err := signing.NewSigner(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("creating run log signer: %w", err)
	}

	return signer, nil
}

// BuildTrustStore constructs the run-log verification trust anchor.
//
// This is the seam that will let Janus go open source: the commercial build
// anchors trust in claims.SigningPublicKey (the license's signing key, already
// validated against the configured private key by BuildSigner/
// ValidateSigningKeyPair and, until now, discarded afterward); an open-source
// build would anchor it in a user-managed keyring via runlog.NewKeyringTrust
// instead. Either way the verification path in internal/runlog is identical.
//
// It returns (nil, nil) when the license carries no signing key — signing is
// optional, and a nil TrustStore makes every signed record report Unverifiable,
// never Valid, which is the correct degradation.
func BuildTrustStore(claims *validator.Claims) (runlog.TrustStore, error) {
	if claims == nil || claims.SigningPublicKey == "" {
		return nil, nil
	}

	trust, err := runlog.NewLicenseTrust(claims.SigningPublicKey, claims.UserEmail)
	if err != nil {
		return nil, fmt.Errorf("building license trust store: %w", err)
	}

	return trust, nil
}
