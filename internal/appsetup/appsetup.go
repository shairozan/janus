// Package appsetup holds the GUI-free bootstrap steps shared by the desktop app
// and the `janus mcp server` daemon: constructing the run-log signer and the
// verification trust anchor from config. Keeping these here means the daemon does
// not import internal/gui (and therefore no fyne) to perform the same startup
// wiring.
package appsetup

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/pharmalytica/janus/internal/config"
	"github.com/pharmalytica/janus/internal/runlog"
	"github.com/pharmalytica/janus/internal/signing"
)

// BuildSigner constructs the run-log signer from cfg. It returns (nil, nil) when
// signing is not configured (disabled), (signer, nil) when enabled, and
// (nil, err) on a configuration or key error. Callers decide how to surface the
// error (the GUI shows a dialog; the daemon logs and continues unsigned).
func BuildSigner(cfg *config.Config) (*signing.Signer, error) {
	if cfg == nil || cfg.Signing.PrivateKeyPath == "" {
		return nil, nil // signing disabled
	}

	privateKeyPath, err := config.ExpandSigningPrivateKeyPath(cfg.Signing.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("expanding signing private key path: %w", err)
	}

	signer, err := signing.NewSigner(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("creating run log signer: %w", err)
	}

	return signer, nil
}

// SignerIdentity returns the identity recorded on records this install signs.
//
// The identity belongs to the signing key, not to the process: it is what the
// trust keyring matches a public key to, and verification anchors on the key
// fingerprint rather than on this string. Resolving it per-run (from the OS user,
// say) would let the identity on a record drift from the key that produced the
// signature, which is a provenance defect in a tamper-evident log.
//
// It returns "" when signing.identity is unset, which is not an error — an
// unsigned install has no identity to record.
func SignerIdentity(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}

	return cfg.Signing.Identity
}

// BuildTrustStore constructs the run-log verification trust anchor from the
// user- or org-maintained keyring: a list of public keys whose signatures this
// install accepts. See runlog.NewKeyringTrust for the file shape.
//
// It returns (nil, nil) when no keyring is configured or the configured keyring
// does not exist yet. Signing and verification are independent — you can sign
// without trusting anyone — and a nil TrustStore makes every signed record report
// Unverifiable rather than Valid, which is the correct degradation.
func BuildTrustStore(cfg *config.Config) (runlog.TrustStore, error) {
	if cfg == nil || cfg.Signing.KeyringPath == "" {
		return nil, nil
	}

	keyringPath, err := config.ExpandSigningKeyringPath(cfg.Signing.KeyringPath)
	if err != nil {
		return nil, fmt.Errorf("expanding signing keyring path: %w", err)
	}

	trust, err := runlog.NewKeyringTrust(keyringPath)
	if err != nil {
		// A keyring that has not been created yet means "trust nobody", not a
		// misconfiguration. Anything else (malformed YAML, unreadable key) is a
		// real error the caller should see.
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}

		return nil, fmt.Errorf("building keyring trust store: %w", err)
	}

	return trust, nil
}
