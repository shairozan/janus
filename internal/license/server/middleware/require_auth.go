package middleware

import (
	"net/http"
	"strings"
)

// RequireAuth is a middleware that requires authentication (either OIDC or JWT).
// It should be applied AFTER the Authentication middleware.
// allowedDomains is a list of email domains allowed for OIDC authentication.
// If empty, no domain validation is performed.
func RequireAuth(allowedDomains []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if either OIDC user or JWT claims are present
			oidcUser, hasOIDC := GetOIDCUser(r)
			_, hasJWT := GetClaims(r)

			if !hasOIDC && !hasJWT {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error": "authentication required"}`))
				return
			}

			// If OIDC authentication, validate email domain (if domains specified)
			if hasOIDC && len(allowedDomains) > 0 {
				if !isAllowedEmailDomain(oidcUser.Email, allowedDomains) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusForbidden)
					w.Write([]byte(`{"error": "access denied: email domain not authorized"}`))
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireOIDC is a middleware that specifically requires OIDC authentication.
// Use this when you want to ensure user identity (not just API tokens).
// allowedDomains is a list of email domains allowed for OIDC authentication.
// If empty, no domain validation is performed.
func RequireOIDC(allowedDomains []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			oidcUser, hasOIDC := GetOIDCUser(r)

			if !hasOIDC {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error": "OIDC authentication required"}`))
				return
			}

			// Validate email domain (if domains specified)
			if len(allowedDomains) > 0 {
				if !isAllowedEmailDomain(oidcUser.Email, allowedDomains) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusForbidden)
					w.Write([]byte(`{"error": "access denied: email domain not authorized"}`))
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isAllowedEmailDomain checks if the email is from an allowed domain.
func isAllowedEmailDomain(email string, allowedDomains []string) bool {
	// Extract domain from email
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}

	domain := strings.ToLower(parts[1])

	// Check if domain is in allowed list
	for _, allowed := range allowedDomains {
		if strings.ToLower(allowed) == domain {
			return true
		}
	}

	return false
}
