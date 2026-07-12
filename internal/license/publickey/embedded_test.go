package publickey

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"
)

// pemForKey encodes an RSA public key as a PKIX PEM block.
func pemForKey(t *testing.T, key *rsa.PublicKey) string {
	t.Helper()

	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}

	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

// newKey generates a small RSA key for testing.
func newKey(t *testing.T) *rsa.PublicKey {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	return &priv.PublicKey
}

func TestParsePublicKeysFromPEM_SingleBlock(t *testing.T) {
	key := newKey(t)

	keys, err := ParsePublicKeysFromPEM(pemForKey(t, key))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}
	if keys[0].N.Cmp(key.N) != 0 {
		t.Error("parsed key does not match input")
	}
}

func TestParsePublicKeysFromPEM_MultipleBlocks(t *testing.T) {
	k1, k2, k3 := newKey(t), newKey(t), newKey(t)
	concatenated := pemForKey(t, k1) + pemForKey(t, k2) + pemForKey(t, k3)

	keys, err := ParsePublicKeysFromPEM(concatenated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}
}

func TestParsePublicKeysFromPEM_EmptyAndGarbage(t *testing.T) {
	for name, input := range map[string]string{
		"empty":      "",
		"whitespace": "   \n  ",
		"garbage":    "this is not a PEM file at all",
	} {
		t.Run(name, func(t *testing.T) {
			keys, err := ParsePublicKeysFromPEM(input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(keys) != 0 {
				t.Errorf("expected 0 keys, got %d", len(keys))
			}
		})
	}
}

func TestParsePublicKeysFromPEM_SkipsNonPublicKeyBlocks(t *testing.T) {
	key := newKey(t)
	// A stray block of a different type should be skipped, not fail the parse.
	stray := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("ignored")}))

	keys, err := ParsePublicKeysFromPEM(stray + pemForKey(t, key))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected 1 key (stray skipped), got %d", len(keys))
	}
}

func TestParsePublicKeysFromPEM_CorruptPublicKeyBlockErrors(t *testing.T) {
	corrupt := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: []byte("not-valid-der")}))

	_, err := ParsePublicKeysFromPEM(corrupt)
	if err == nil {
		t.Fatal("expected error for corrupt PUBLIC KEY block")
	}
	if !strings.Contains(err.Error(), "failed to parse public key") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestParsePublicKeyFromPEM_FirstKey(t *testing.T) {
	k1, k2 := newKey(t), newKey(t)

	got, err := ParsePublicKeyFromPEM(pemForKey(t, k1) + pemForKey(t, k2))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.N.Cmp(k1.N) != 0 {
		t.Error("expected first key to be returned")
	}
}

func TestParsePublicKeyFromPEM_NoBlock(t *testing.T) {
	_, err := ParsePublicKeyFromPEM("")
	if err == nil {
		t.Fatal("expected error when no PEM block present")
	}
}
