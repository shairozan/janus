package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// TokenValidator validates access tokens and fetches user info from the OIDC
// provider. Both *OIDCValidator (single issuer) and *ValidatorSet (multi issuer)
// satisfy it, so either can be wired into the server.
type TokenValidator interface {
	ValidateToken(ctx context.Context, tokenString string) (*OIDCUser, error)
	FetchUserInfo(ctx context.Context, accessToken string) (*OIDCUser, error)
}

// IssuerFromToken extracts the unverified `iss` claim from a JWT. It is used
// only to route a token to the right per-issuer validator; that validator then
// performs full signature/issuer/audience verification, so reading the claim
// unverified here is safe.
func IssuerFromToken(raw string) (string, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("malformed JWT: expected 3 parts, got %d", len(parts))
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	var claims struct {
		Iss string `json:"iss"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("failed to parse JWT payload: %w", err)
	}

	if claims.Iss == "" {
		return "", fmt.Errorf("token has no iss claim")
	}

	return claims.Iss, nil
}

// ValidatorSet routes tokens to a per-issuer validator by their `iss` claim, so
// one license-server can validate tokens from multiple Cognito user pools (the
// >300-IdP-per-pool scaling path). Today a single issuer is registered; adding
// more pools is a Register call, not a rewrite.
type ValidatorSet struct {
	mu       sync.RWMutex
	byIssuer map[string]TokenValidator
}

// NewValidatorSet creates an empty ValidatorSet.
func NewValidatorSet() *ValidatorSet {
	return &ValidatorSet{byIssuer: make(map[string]TokenValidator)}
}

// Register builds an OIDCValidator for the given config and indexes it by its
// issuer URL.
func (s *ValidatorSet) Register(ctx context.Context, config *OIDCConfig) error {
	v, err := NewOIDCValidator(ctx, config)
	if err != nil {
		return err
	}

	s.registerValidator(v.GetIssuer(), v)

	return nil
}

// registerValidator indexes a validator by issuer (also the test seam).
func (s *ValidatorSet) registerValidator(issuer string, v TokenValidator) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byIssuer[issuer] = v
}

func (s *ValidatorSet) validatorFor(rawToken string) (TokenValidator, error) {
	issuer, err := IssuerFromToken(rawToken)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	v, ok := s.byIssuer[issuer]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no validator registered for issuer %q", issuer)
	}

	return v, nil
}

// ValidateToken selects the validator matching the token's issuer and delegates.
func (s *ValidatorSet) ValidateToken(ctx context.Context, tokenString string) (*OIDCUser, error) {
	v, err := s.validatorFor(tokenString)
	if err != nil {
		return nil, err
	}

	return v.ValidateToken(ctx, tokenString)
}

// FetchUserInfo selects the validator matching the access token's issuer and
// delegates the userinfo call.
func (s *ValidatorSet) FetchUserInfo(ctx context.Context, accessToken string) (*OIDCUser, error) {
	v, err := s.validatorFor(accessToken)
	if err != nil {
		return nil, err
	}

	return v.FetchUserInfo(ctx, accessToken)
}
