package appsetup_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/shairozan/janus/internal/appsetup"
	"github.com/shairozan/janus/internal/config"
	"github.com/shairozan/janus/internal/runlog"
)

// writeKeyPair generates an RSA key, writes the private half to a PEM file, and
// returns the path plus the public half in PEM form.
func writeKeyPair(t *testing.T, dir, name string) (privatePath, publicPEM string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}

	privatePath = filepath.Join(dir, name+".pem")
	privateBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	if err := os.WriteFile(privatePath, privateBytes, 0o600); err != nil {
		t.Fatalf("writing private key: %v", err)
	}

	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("marshalling public key: %v", err)
	}

	publicPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}))

	return privatePath, publicPEM
}

func writeKeyring(t *testing.T, dir, email, publicPEM string) string {
	t.Helper()

	// The keyring is YAML with a block scalar per key; indent the PEM to match.
	var indented string
	for _, line := range splitLines(publicPEM) {
		indented += "      " + line + "\n"
	}

	path := filepath.Join(dir, "keyring.yml")
	body := "keys:\n  - email: " + email + "\n    public_key: |\n" + indented

	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("writing keyring: %v", err)
	}

	return path
}

func splitLines(s string) []string {
	var out []string

	start := 0

	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			if i > start {
				out = append(out, s[start:i])
			}

			start = i + 1
		}
	}

	if start < len(s) {
		out = append(out, s[start:])
	}

	return out
}

// TestSignAndVerifyWithoutLicense is the Sprint 1 exit criterion: a record signs
// and verifies with no license anywhere in the picture — the key comes from
// config, and the trust anchor from a user-maintained keyring.
func TestSignAndVerifyWithoutLicense(t *testing.T) {
	dir := t.TempDir()
	privatePath, publicPEM := writeKeyPair(t, dir, "signing")
	keyringPath := writeKeyring(t, dir, "modeler@example.com", publicPEM)

	cfg := &config.Config{}
	cfg.Signing.PrivateKeyPath = privatePath
	cfg.Signing.Identity = "modeler@example.com"
	cfg.Signing.KeyringPath = keyringPath

	signer, err := appsetup.BuildSigner(cfg)
	if err != nil {
		t.Fatalf("BuildSigner: %v", err)
	}

	if signer == nil {
		t.Fatal("BuildSigner returned no signer despite a configured key")
	}

	trust, err := appsetup.BuildTrustStore(cfg)
	if err != nil {
		t.Fatalf("BuildTrustStore: %v", err)
	}

	if trust == nil {
		t.Fatal("BuildTrustStore returned no trust anchor despite a configured keyring")
	}

	record := &runlog.RunRecord{ID: "run-1", ModelFile: "model.mod"}
	if err := runlog.SignRecordWithInfo(record, signer, appsetup.SignerIdentity(cfg)); err != nil {
		t.Fatalf("signing record: %v", err)
	}

	if record.SignerEmail != "modeler@example.com" {
		t.Errorf("SignerEmail = %q, want the configured identity", record.SignerEmail)
	}

	result := runlog.VerifyRecordStatus(record, trust)
	if result.Status != runlog.VerificationValid {
		t.Errorf("VerifyRecordStatus = %v (%s), want Valid", result.Status, result.Message)
	}
}

// TestUntrustedKeyIsNotValid guards the property the keyring exists to provide:
// a well-formed signature from a key the keyring does not list must not verify.
// Without this, swapping the license anchor for a keyring would silently accept
// anyone.
func TestUntrustedKeyIsNotValid(t *testing.T) {
	dir := t.TempDir()
	privatePath, _ := writeKeyPair(t, dir, "signing")
	_, strangerPEM := writeKeyPair(t, dir, "stranger")
	keyringPath := writeKeyring(t, dir, "stranger@example.com", strangerPEM)

	cfg := &config.Config{}
	cfg.Signing.PrivateKeyPath = privatePath
	cfg.Signing.Identity = "modeler@example.com"
	cfg.Signing.KeyringPath = keyringPath

	signer, err := appsetup.BuildSigner(cfg)
	if err != nil {
		t.Fatalf("BuildSigner: %v", err)
	}

	trust, err := appsetup.BuildTrustStore(cfg)
	if err != nil {
		t.Fatalf("BuildTrustStore: %v", err)
	}

	record := &runlog.RunRecord{ID: "run-1", ModelFile: "model.mod"}
	if err := runlog.SignRecordWithInfo(record, signer, appsetup.SignerIdentity(cfg)); err != nil {
		t.Fatalf("signing record: %v", err)
	}

	result := runlog.VerifyRecordStatus(record, trust)
	if result.Status == runlog.VerificationValid {
		t.Error("a key absent from the keyring verified as Valid")
	}
}

// TestNoSigningConfigured covers the default open-source install: nothing
// configured, no error, no signer, no trust anchor.
func TestNoSigningConfigured(t *testing.T) {
	cfg := &config.Config{}

	signer, err := appsetup.BuildSigner(cfg)
	if err != nil {
		t.Fatalf("BuildSigner: %v", err)
	}

	if signer != nil {
		t.Error("expected no signer when signing is unconfigured")
	}

	trust, err := appsetup.BuildTrustStore(cfg)
	if err != nil {
		t.Fatalf("BuildTrustStore: %v", err)
	}

	if trust != nil {
		t.Error("expected no trust anchor when no keyring is configured")
	}

	if got := appsetup.SignerIdentity(cfg); got != "" {
		t.Errorf("SignerIdentity = %q, want empty", got)
	}
}

// TestMissingKeyringIsNotAnError covers a configured-but-absent keyring: that
// means "trust nobody yet", not a misconfiguration, so startup must not fail.
func TestMissingKeyringIsNotAnError(t *testing.T) {
	cfg := &config.Config{}
	cfg.Signing.KeyringPath = filepath.Join(t.TempDir(), "does-not-exist.yml")

	trust, err := appsetup.BuildTrustStore(cfg)
	if err != nil {
		t.Fatalf("BuildTrustStore on a missing keyring should not error: %v", err)
	}

	if trust != nil {
		t.Error("expected no trust anchor from a missing keyring")
	}
}
