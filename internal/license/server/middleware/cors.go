package middleware

import (
	"net/http"
	"slices"
)

// CORS restricts cross-origin browser access to an allowlist of origins. It backs
// the public signup endpoint, which a separate marketing-site origin may POST to
// directly from the browser. When the allowlist is empty no CORS headers are set
// (portal-only mode — the portal calls the endpoint server-side and needs no
// CORS). OPTIONS preflights are always answered with 204 so they never fall
// through to a handler that enforces POST.
func CORS(allowedOrigins []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && slices.Contains(allowedOrigins, origin) {
				h := w.Header()
				h.Set("Access-Control-Allow-Origin", origin)
				h.Add("Vary", "Origin")
				h.Set("Access-Control-Allow-Methods", "POST, OPTIONS")
				h.Set("Access-Control-Allow-Headers", "Content-Type")
				h.Set("Access-Control-Max-Age", "600")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
