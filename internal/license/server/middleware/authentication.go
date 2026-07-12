package middleware

import (
	"context"
	"log"
	"net/http"

	"github.com/pharmalytica/janus/internal/license/auth"
)

// OIDCValidator is the subset of auth.OIDCValidator used by Authentication.
type OIDCValidator interface {
	ValidateToken(ctx context.Context, tokenString string) (*auth.OIDCUser, error)
}

// Authentication extracts and validates a Cognito access token from the
// Authorization: Bearer header. On a valid token it stores the OIDCUser
// (Sub and Expiry populated) in the request context.
//
// This middleware is permissive — requests with missing or invalid tokens pass
// through without an OIDCUser in context. Use RequireUserInfo on individual
// routes to enforce access.
func Authentication(oidcValidator OIDCValidator, logger *log.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if oidcValidator == nil {
				next.ServeHTTP(w, r)

				return
			}

			token := bearerToken(r)
			if token == "" {
				next.ServeHTTP(w, r)

				return
			}

			user, err := oidcValidator.ValidateToken(r.Context(), token)
			if err != nil {
				logger.Printf("token validation failed: %v", err)
				next.ServeHTTP(w, r)

				return
			}

			ctx := context.WithValue(r.Context(), contextKeyOIDCUser, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// bearerToken extracts the token string from an "Authorization: Bearer <token>"
// header. Returns an empty string if the header is absent or malformed.
func bearerToken(r *http.Request) string {
	const prefix = "Bearer "

	h := r.Header.Get("Authorization")
	if len(h) <= len(prefix) || h[:len(prefix)] != prefix {
		return ""
	}

	return h[len(prefix):]
}
