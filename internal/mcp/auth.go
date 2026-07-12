package mcp

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// authMiddleware wraps next with mandatory bearer-token authentication. Every
// request must carry "Authorization: Bearer <token>" matching the configured
// token; the comparison is constant-time. Requests without a valid token receive
// 401 and never reach next.
func authMiddleware(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !validBearer(r.Header.Get("Authorization"), token) {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)

			return
		}

		next.ServeHTTP(w, r)
	})
}

// validBearer reports whether the Authorization header carries the expected
// bearer token, using a constant-time comparison to avoid leaking the token via
// timing.
func validBearer(header, token string) bool {
	const prefix = "Bearer "

	if token == "" {
		return false
	}

	if !strings.HasPrefix(header, prefix) {
		return false
	}

	got := strings.TrimSpace(header[len(prefix):])

	return subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
}
