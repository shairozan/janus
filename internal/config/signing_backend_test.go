package config_test

import (
	"errors"
	"testing"

	"github.com/shairozan/janus/internal/config"
)

// TestResolveSigningBackend pins the inference rules, including the one that
// matters for upgrades: a config written before the credential store existed —
// private_key_path and nothing else — must keep resolving to the file backend.
func TestResolveSigningBackend(t *testing.T) {
	tests := []struct {
		name        string
		cfg         config.SigningConfig
		wantBackend string
		wantEnabled bool
	}{
		{
			name:        "nothing configured means signing is off",
			cfg:         config.SigningConfig{},
			wantEnabled: false,
		},
		{
			name:        "a pre-existing key path still resolves to the file backend",
			cfg:         config.SigningConfig{PrivateKeyPath: "~/.config/janus/signing.pem"},
			wantBackend: config.SigningBackendFile,
			wantEnabled: true,
		},
		{
			name:        "an identity alone implies the credential store",
			cfg:         config.SigningConfig{Identity: "modeler@example.com"},
			wantBackend: config.SigningBackendKeychain,
			wantEnabled: true,
		},
		{
			name: "an explicit file backend wins over an identity",
			cfg: config.SigningConfig{
				Backend:        config.SigningBackendFile,
				PrivateKeyPath: "/keys/signing.pem",
				Identity:       "modeler@example.com",
			},
			wantBackend: config.SigningBackendFile,
			wantEnabled: true,
		},
		{
			name: "an explicit keychain backend wins over a key path",
			cfg: config.SigningConfig{
				Backend:        config.SigningBackendKeychain,
				PrivateKeyPath: "/keys/signing.pem",
				Identity:       "modeler@example.com",
			},
			wantBackend: config.SigningBackendKeychain,
			wantEnabled: true,
		},
		{
			name:        "an explicit keychain backend with no identity has nothing to address",
			cfg:         config.SigningConfig{Backend: config.SigningBackendKeychain},
			wantBackend: config.SigningBackendKeychain,
			wantEnabled: false,
		},
		{
			name:        "an explicit file backend with no path has nothing to read",
			cfg:         config.SigningConfig{Backend: config.SigningBackendFile},
			wantBackend: config.SigningBackendFile,
			wantEnabled: false,
		},
		{
			name: "the keyring alone does not enable signing",
			cfg:  config.SigningConfig{KeyringPath: "~/.config/janus/keyring.yml"},
			// Verification and signing are independent: trusting other people's
			// keys must not imply that this install holds one of its own.
			wantEnabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend, enabled, err := config.ResolveSigningBackend(tt.cfg)

			if err != nil {
				t.Fatalf("ResolveSigningBackend: %v", err)
			}

			if enabled != tt.wantEnabled {
				t.Errorf("enabled = %v, want %v", enabled, tt.wantEnabled)
			}

			if tt.wantBackend != "" && backend != tt.wantBackend {
				t.Errorf("backend = %q, want %q", backend, tt.wantBackend)
			}
		})
	}
}

// TestUnknownSigningBackendIsRejected pins the fix for a silent misconfiguration:
// a typo'd backend used to fall through to inference and could select the other
// backend entirely, signing with a stale key file under a different fingerprint.
func TestUnknownSigningBackendIsRejected(t *testing.T) {
	cfg := config.SigningConfig{
		Backend:        "keyring", // a plausible typo for "keychain"
		Identity:       "modeler@example.com",
		PrivateKeyPath: "/keys/old-signing.pem",
	}

	backend, enabled, err := config.ResolveSigningBackend(cfg)
	if err == nil {
		t.Fatalf("an unknown backend must be an error; got backend=%q enabled=%v", backend, enabled)
	}

	if !errors.Is(err, config.ErrUnknownSigningBackend) {
		t.Errorf("error = %v, want it to wrap ErrUnknownSigningBackend", err)
	}

	if enabled {
		t.Error("an unresolvable backend must not report signing as enabled")
	}
}
