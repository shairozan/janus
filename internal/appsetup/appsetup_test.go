package appsetup_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shairozan/janus/internal/appsetup"
	"github.com/shairozan/janus/internal/config"
	"github.com/shairozan/janus/internal/runlog"
	"github.com/shairozan/janus/internal/signing"
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

// TestKeychainBackendSignsAndVerifies is the Sprint 3 exit criterion: a key held
// in the OS credential store signs a record, and that record verifies against a
// keyring built from the exported public half — with no key file on disk at all.
func TestKeychainBackendSignsAndVerifies(t *testing.T) {
	identity := fmt.Sprintf("janus-appsetup-%d@example.invalid", os.Getpid())

	key, err := signing.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	if err := signing.StoreKey(identity, signing.EncodePrivateKeyPEM(key)); err != nil {
		t.Skipf("no usable OS credential store on this host (%v); the file backend covers these hosts", err)
	}

	t.Cleanup(func() { _ = signing.DeleteKey(identity) })

	publicPEM, err := signing.EncodePublicKeyPEM(key)
	if err != nil {
		t.Fatalf("EncodePublicKeyPEM: %v", err)
	}

	cfg := &config.Config{}
	cfg.Signing.Backend = config.SigningBackendKeychain
	cfg.Signing.Identity = identity
	cfg.Signing.KeyringPath = writeKeyring(t, t.TempDir(), identity, publicPEM)

	signer, err := appsetup.BuildSigner(cfg)
	if err != nil {
		t.Fatalf("BuildSigner: %v", err)
	}

	if signer == nil {
		t.Fatal("BuildSigner returned no signer for a key held in the credential store")
	}

	trust, err := appsetup.BuildTrustStore(cfg)
	if err != nil {
		t.Fatalf("BuildTrustStore: %v", err)
	}

	record := &runlog.RunRecord{ID: "run-keychain", ModelFile: "model.mod"}
	if err := runlog.SignRecordWithInfo(record, signer, appsetup.SignerIdentity(cfg)); err != nil {
		t.Fatalf("signing record: %v", err)
	}

	if record.SignerEmail != identity {
		t.Errorf("SignerEmail = %q, want the identity bound to the key", record.SignerEmail)
	}

	if result := runlog.VerifyRecordStatus(record, trust); result.Status != runlog.VerificationValid {
		t.Errorf("VerifyRecordStatus = %v (%s), want Valid", result.Status, result.Message)
	}
}

// TestMissingStoredKeyExplainsItself covers the first-run mistake: the config
// names an identity the credential store has no key for. The error has to point
// at `janus keys generate`, because "no signing key" and "credential store
// unavailable" need completely different fixes.
func TestMissingStoredKeyExplainsItself(t *testing.T) {
	cfg := &config.Config{}
	cfg.Signing.Backend = config.SigningBackendKeychain
	cfg.Signing.Identity = fmt.Sprintf("janus-absent-%d@example.invalid", os.Getpid())

	_, err := appsetup.BuildSigner(cfg)
	if err == nil {
		t.Fatal("expected an error when the credential store holds no key for the identity")
	}

	// A host with no usable credential store fails differently — "dbus-launch not
	// found" on headless Linux, for instance — and "run janus keys generate" is
	// the wrong advice there, since generating a key would fail for the same
	// reason. That distinction is the whole point of ErrNoStoredKey, so assert on
	// it rather than on the message, and skip where the store itself is absent.
	if !errors.Is(err, signing.ErrNoStoredKey) {
		t.Skipf("no usable OS credential store on this host (%v); "+
			"signing.private_key_path covers these hosts", err)
	}

	if !strings.Contains(err.Error(), "janus keys generate") {
		t.Errorf("error does not say how to fix it: %v", err)
	}
}

// TestFileBackendWithoutIdentityRefusesToSign pins the fix for a silent
// attribution loss.
//
// A config carried over from before signing.identity existed holds only
// private_key_path, and that still resolves to a usable file-backed signer. It
// used to sign happily with SignerIdentity() == "", so every new record went to
// disk with an empty signer_email while older records had a real one — no error,
// no warning, and only for upgrading users. Failing loudly is the right trade:
// an unsigned run is recoverable, an unattributed signed record is not.
func TestFileBackendWithoutIdentityRefusesToSign(t *testing.T) {
	dir := t.TempDir()
	privatePath, _ := writeKeyPair(t, dir, "legacy")

	cfg := &config.Config{}
	cfg.Signing.PrivateKeyPath = privatePath // exactly the pre-upgrade shape

	signer, err := appsetup.BuildSigner(cfg)
	if err == nil {
		t.Fatal("a file-backed key with no identity must not produce a signer that records empty attribution")
	}

	if signer != nil {
		t.Error("no signer may be returned alongside the error")
	}

	if !strings.Contains(err.Error(), "signing.identity") {
		t.Errorf("the error must name the setting to add; got: %v", err)
	}
}

// TestFileBackendWithIdentitySigns is the other half: the same config with an
// identity added works, so the fix above is a prompt to configure rather than a
// removal of the file backend.
func TestFileBackendWithIdentitySigns(t *testing.T) {
	dir := t.TempDir()
	privatePath, publicPEM := writeKeyPair(t, dir, "legacy")

	cfg := &config.Config{}
	cfg.Signing.PrivateKeyPath = privatePath
	cfg.Signing.Identity = "modeler@example.com"
	cfg.Signing.KeyringPath = writeKeyring(t, dir, "modeler@example.com", publicPEM)

	signer, err := appsetup.BuildSigner(cfg)
	if err != nil {
		t.Fatalf("BuildSigner: %v", err)
	}

	record := &runlog.RunRecord{ID: "run-legacy", ModelFile: "model.mod"}
	if err := runlog.SignRecordWithInfo(record, signer, appsetup.SignerIdentity(cfg)); err != nil {
		t.Fatalf("signing record: %v", err)
	}

	if record.SignerEmail != "modeler@example.com" {
		t.Errorf("SignerEmail = %q, want the configured identity", record.SignerEmail)
	}

	trust, err := appsetup.BuildTrustStore(cfg)
	if err != nil {
		t.Fatalf("BuildTrustStore: %v", err)
	}

	if result := runlog.VerifyRecordStatus(record, trust); result.Status != runlog.VerificationValid {
		t.Errorf("VerifyRecordStatus = %v (%s), want Valid", result.Status, result.Message)
	}
}
