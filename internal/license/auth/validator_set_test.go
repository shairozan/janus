package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
)

// craftToken builds an unsigned JWT (header.payload.sig) whose payload carries
// the given issuer + subject. The signature is irrelevant — ValidatorSet routes
// on the unverified iss and the fake validator below does not verify.
func craftToken(t *testing.T, iss, sub string) string {
	t.Helper()

	enc := func(v interface{}) string {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}

		return base64.RawURLEncoding.EncodeToString(b)
	}

	header := enc(map[string]string{"alg": "RS256", "typ": "JWT"})
	payload := enc(map[string]string{"iss": iss, "sub": sub})

	return header + "." + payload + ".sig"
}

type fakeValidator struct {
	user        *OIDCUser
	validateErr error
}

func (f *fakeValidator) ValidateToken(_ context.Context, _ string) (*OIDCUser, error) {
	return f.user, f.validateErr
}

func (f *fakeValidator) FetchUserInfo(_ context.Context, _ string) (*OIDCUser, error) {
	return f.user, f.validateErr
}

func TestIssuerFromToken(t *testing.T) {
	t.Run("valid token", func(t *testing.T) {
		iss, err := IssuerFromToken(craftToken(t, "https://pool-a", "sub-1"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if iss != "https://pool-a" {
			t.Fatalf("got iss %q, want https://pool-a", iss)
		}
	})

	t.Run("malformed token", func(t *testing.T) {
		if _, err := IssuerFromToken("not-a-jwt"); err == nil {
			t.Fatal("expected error for malformed token")
		}
	})

	t.Run("missing iss", func(t *testing.T) {
		payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"x"}`))
		if _, err := IssuerFromToken("h." + payload + ".s"); err == nil {
			t.Fatal("expected error for token without iss")
		}
	})
}

func TestValidatorSet_Routing(t *testing.T) {
	set := NewValidatorSet()
	want := &OIDCUser{Sub: "sub-1", Groups: []string{"customer_admin"}}
	set.registerValidator("https://pool-a", &fakeValidator{user: want})

	t.Run("routes to the matching issuer", func(t *testing.T) {
		got, err := set.ValidateToken(context.Background(), craftToken(t, "https://pool-a", "sub-1"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got.Sub != "sub-1" || len(got.Groups) != 1 || got.Groups[0] != "customer_admin" {
			t.Fatalf("unexpected user: %+v", got)
		}
	})

	t.Run("unknown issuer is rejected", func(t *testing.T) {
		_, err := set.ValidateToken(context.Background(), craftToken(t, "https://pool-unknown", "sub-1"))
		if err == nil {
			t.Fatal("expected error for unregistered issuer")
		}
	})

	t.Run("FetchUserInfo routes by issuer too", func(t *testing.T) {
		_, err := set.FetchUserInfo(context.Background(), craftToken(t, "https://pool-unknown", "sub-1"))
		if err == nil {
			t.Fatal("expected error for unregistered issuer")
		}
	})

	t.Run("propagates validator errors", func(t *testing.T) {
		set.registerValidator("https://pool-b", &fakeValidator{validateErr: errors.New("bad token")})
		if _, err := set.ValidateToken(context.Background(), craftToken(t, "https://pool-b", "s")); err == nil {
			t.Fatal("expected the validator error to propagate")
		}
	})
}
