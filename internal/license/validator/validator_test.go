package validator

import (
	"crypto/rand"
	"crypto/rsa"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// claimsExpiring returns claims that satisfy validateClaims, with the given expiry.
func claimsExpiring(expiresAt time.Time) *Claims {
	c := &Claims{
		AgreementID:    42,
		OrganizationID: 7,
		Tier:           "professional",
		MaxSeats:       10,
	}
	c.ExpiresAt = jwt.NewNumericDate(expiresAt)

	return c
}

// signToken signs claims with the given private key using RS256.
func signToken(t *testing.T, priv *rsa.PrivateKey, claims *Claims) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(priv)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	return signed
}

func genKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	return priv
}

func TestValidateToken_SingleKey(t *testing.T) {
	priv := genKey(t)
	v := &Validator{publicKeys: []*rsa.PublicKey{&priv.PublicKey}}

	token := signToken(t, priv, claimsExpiring(time.Now().Add(time.Hour)))

	claims, err := v.ValidateToken(token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.OrganizationID != 7 {
		t.Errorf("unexpected claims: %+v", claims)
	}
}

func TestValidateToken_MatchesSecondOfThreeKeys(t *testing.T) {
	k1, k2, k3 := genKey(t), genKey(t), genKey(t)
	// Token signed by k2; validator holds all three (order: k1, k2, k3).
	v := &Validator{publicKeys: []*rsa.PublicKey{&k1.PublicKey, &k2.PublicKey, &k3.PublicKey}}

	token := signToken(t, k2, claimsExpiring(time.Now().Add(time.Hour)))

	claims, err := v.ValidateToken(token)
	if err != nil {
		t.Fatalf("expected token signed by an embedded key to validate, got: %v", err)
	}
	if claims.Tier != "professional" {
		t.Errorf("unexpected claims: %+v", claims)
	}
}

func TestValidateToken_UnknownKeyFailsGenerically(t *testing.T) {
	embedded, foreign := genKey(t), genKey(t)
	v := &Validator{publicKeys: []*rsa.PublicKey{&embedded.PublicKey}}

	// Signed by a key that is NOT embedded.
	token := signToken(t, foreign, claimsExpiring(time.Now().Add(time.Hour)))

	_, err := v.ValidateToken(token)
	if err == nil {
		t.Fatal("expected validation to fail for unknown signing key")
	}
	if err.Error() != "token validation failed" {
		t.Errorf("expected generic message, got: %v", err)
	}
}

func TestValidateToken_ExpiredSurfacesExpiry(t *testing.T) {
	k1, k2 := genKey(t), genKey(t)
	v := &Validator{publicKeys: []*rsa.PublicKey{&k1.PublicKey, &k2.PublicKey}}

	// Correctly signed by k2 but expired in the past.
	token := signToken(t, k2, claimsExpiring(time.Now().Add(-time.Hour)))

	_, err := v.ValidateToken(token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("expected expiry to be surfaced, got: %v", err)
	}
}

func TestValidateToken_NoEmbeddedKeysFails(t *testing.T) {
	priv := genKey(t)
	v := &Validator{publicKeys: nil}

	token := signToken(t, priv, claimsExpiring(time.Now().Add(time.Hour)))

	_, err := v.ValidateToken(token)
	if err == nil {
		t.Fatal("expected failure when no keys are embedded")
	}
	if err.Error() != "token validation failed" {
		t.Errorf("expected generic message, got: %v", err)
	}
}
