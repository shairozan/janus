package middleware

import (
	"context"
	"log"
	"net/http"

	"github.com/pharmalytica/janus/internal/license/auth"
	"github.com/pharmalytica/janus/internal/license/jwt"
)

// JWTValidator is an interface for validating JWT tokens.
type JWTValidator interface {
	ValidateToken(tokenString string) (*jwt.Claims, error)
}

// OIDCValidator is an interface for validating OIDC ID tokens.
type OIDCValidator interface {
	ValidateIDToken(ctx context.Context, tokenString string) (*auth.OIDCUser, error)
	GetIssuer() string
}

// Authentication extracts and validates Bearer tokens from the Authorization header.
// Supports both Janus JWT tokens and OIDC ID tokens.
// If a valid token is found, the claims/user are added to the request context.
// This middleware is permissive - it doesn't fail if no token is present, allowing
// endpoints to decide if authentication is required.
func Authentication(jwtGen JWTValidator, oidcValidator OIDCValidator, logger *log.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract Bearer token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				// No token provided - pass through without claims
				next.ServeHTTP(w, r)
				return
			}

			// Check for "Bearer " prefix
			const bearerPrefix = "Bearer "
			if len(authHeader) < len(bearerPrefix) || authHeader[:len(bearerPrefix)] != bearerPrefix {
				// Invalid format - pass through without claims
				next.ServeHTTP(w, r)
				return
			}

			// Extract token
			tokenString := authHeader[len(bearerPrefix):]
			if tokenString == "" {
				next.ServeHTTP(w, r)
				return
			}

			ctx := r.Context()

			// Try OIDC validation first (if configured)
			if oidcValidator != nil {
				oidcUser, err := oidcValidator.ValidateIDToken(ctx, tokenString)
				if err == nil {
					// Valid OIDC token - add user to context
					ctx = context.WithValue(ctx, contextKeyOIDCUser, oidcUser)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				// OIDC validation failed, try JWT validation below
				logger.Printf("OIDC token validation failed (will try JWT): %v", err)
			}

			// Try Janus JWT validation
			claims, err := jwtGen.ValidateToken(tokenString)
			if err != nil {
				// Invalid token - log but don't fail the request
				logger.Printf("JWT token validation failed: %v", err)
				next.ServeHTTP(w, r)
				return
			}

			// Valid JWT token - add claims to context
			ctx = context.WithValue(ctx, contextKeyClaims, claims)

			// Pass the enriched request to the next handler
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}