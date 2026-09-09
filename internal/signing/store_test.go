package signing

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"os"
	"testing"
)

// storeIdentity returns a process-unique identity so a failed run cannot leave
// debris that a later run mistakes for real state, and so concurrent runs on one
// machine do not fight over the same credential.
func storeIdentity(t *testing.T, label string) string {
	t.Helper()

	id := fmt.Sprintf("janus-test-%s-%d@example.invalid", label, os.Getpid())
	t.Cleanup(func() { _ = DeleteKey(id) })

	return id
}

// skipWithoutCredentialStore skips when this host has no usable credential store.
// Headless Linux (no D-Bus session) and locked-down CI are the normal cases; that
// is exactly why signing.private_key_path exists, so it is a skip rather than a
// failure.
func skipWithoutCredentialStore(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		return
	}

	if errors.Is(err, ErrNoStoredKey) {
		return
	}

	t.Skipf("no usable OS credential store on this host (%v); signing.private_key_path covers these hosts", err)
}

func TestStoreLoadRoundTrip(t *testing.T) {
	id := storeIdentity(t, "roundtrip")

	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	keyPEM := EncodePrivateKeyPEM(key)

	if err := StoreKey(id, keyPEM); err != nil {
		skipWithoutCredentialStore(t, err)
		t.Fatalf("StoreKey: %v", err)
	}

	got, err := LoadKey(id)
	if err != nil {
		t.Fatalf("LoadKey: %v", err)
	}

	if string(got) != string(keyPEM) {
		t.Error("the key read back differs from the key stored")
	}

	signer, err := NewSignerFromCredentialStore(id)
	if err != nil {
		t.Fatalf("NewSignerFromCredentialStore: %v", err)
	}

	// The signer must be the same key, not merely a valid one: compare the
	// fingerprint against the key we generated.
	want, err := NewSignerFromKey(key).PublicKeyFingerprint()
	if err != nil {
		t.Fatal(err)
	}

	got2, err := signer.PublicKeyFingerprint()
	if err != nil {
		t.Fatal(err)
	}

	if got2 != want {
		t.Errorf("fingerprint = %s, want %s", got2, want)
	}
}

func TestLoadMissingKeyIsDistinguishable(t *testing.T) {
	id := storeIdentity(t, "missing")

	_, err := LoadKey(id)
	if err == nil {
		t.Fatal("expected an error for an identity with no stored key")
	}

	// Callers branch on this: "no key yet, generate one" is different advice from
	// "your credential store is unavailable", and conflating them sends users off
	// fixing the wrong thing.
	if !errors.Is(err, ErrNoStoredKey) {
		skipWithoutCredentialStore(t, err)
		t.Errorf("error = %v, want it to wrap ErrNoStoredKey", err)
	}
}

func TestDeleteIsIdempotent(t *testing.T) {
	id := storeIdentity(t, "delete")

	// Deleting a key that was never there already satisfies the caller's goal.
	if err := DeleteKey(id); err != nil {
		skipWithoutCredentialStore(t, err)
		t.Fatalf("DeleteKey on a missing key should succeed: %v", err)
	}

	key, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	if err := StoreKey(id, EncodePrivateKeyPEM(key)); err != nil {
		skipWithoutCredentialStore(t, err)
		t.Fatalf("StoreKey: %v", err)
	}

	if err := DeleteKey(id); err != nil {
		t.Fatalf("DeleteKey: %v", err)
	}

	if _, err := LoadKey(id); !errors.Is(err, ErrNoStoredKey) {
		t.Errorf("after delete, LoadKey error = %v, want ErrNoStoredKey", err)
	}
}

func TestEmptyIdentityIsRejected(t *testing.T) {
	// An empty identity would silently address a single shared credential, so
	// every user of the machine would overwrite the same key.
	if err := StoreKey("", []byte("x")); err == nil {
		t.Error("StoreKey accepted an empty identity")
	}

	if _, err := LoadKey(""); err == nil {
		t.Error("LoadKey accepted an empty identity")
	}

	if err := DeleteKey(""); err == nil {
		t.Error("DeleteKey accepted an empty identity")
	}
}

// TestGeneratedKeyFitsTheCredentialStore pins the reason KeyBits is 2048.
//
// Windows Credential Manager caps a credential blob at 2560 bytes. A 2048-bit
// PKCS#1 private key PEM is around 1.7KB and fits; a 4096-bit one is around
// 3.2KB and is rejected outright. If KeyBits is ever raised, this fails on
// Windows rather than shipping a build that cannot store its own key.
func TestGeneratedKeyFitsTheCredentialStore(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	const windowsCredentialBlobLimit = 2560

	if size := len(EncodePrivateKeyPEM(key)); size >= windowsCredentialBlobLimit {
		t.Errorf("a generated key PEM is %d bytes, which does not fit the %d-byte "+
			"Windows credential limit; lower KeyBits or stop defaulting to the credential store",
			size, windowsCredentialBlobLimit)
	}
}

func TestEncodePublicKeyPEMRoundTrips(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	publicPEM, err := EncodePublicKeyPEM(key)
	if err != nil {
		t.Fatalf("EncodePublicKeyPEM: %v", err)
	}

	parsed, err := LoadPublicKeyFromPEM(publicPEM)
	if err != nil {
		t.Fatalf("LoadPublicKeyFromPEM: %v", err)
	}

	if parsed.N.Cmp(key.N) != 0 || parsed.E != key.E {
		t.Error("the exported public key does not match the generated key")
	}
}

func TestEncodePrivateKeyPEMRoundTrips(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := ParsePrivateKeyFromPEM(EncodePrivateKeyPEM(key))
	if err != nil {
		t.Fatalf("ParsePrivateKeyFromPEM: %v", err)
	}

	if parsed.N.Cmp(key.N) != 0 {
		t.Error("the private key did not survive the PEM round trip")
	}
}
