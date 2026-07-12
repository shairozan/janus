package middleware

import (
	"log"
	"net/http"
	"time"
)

// Logging logs HTTP requests and responses.
func Logging(logger *log.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap the response writer to capture status code
			wrapped := newResponseWriter(w)

			// Log the incoming request
			logger.Printf("→ %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)

			// Call the next handler
			next.ServeHTTP(wrapped, r)

			// Log the response
			duration := time.Since(start)
			logger.Printf("← %s %s %d (%d bytes) in %v",
				r.Method,
				r.URL.Path,
				wrapped.statusCode,
				wrapped.written,
				duration,
			)
		})
	}
}