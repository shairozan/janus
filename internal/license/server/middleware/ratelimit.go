package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// maxSignupProbeBody bounds the body read the rate limiter does to peek the
// admin email; the handler applies its own bound independently.
const maxSignupProbeBody = 1 << 20 // 1 MiB

// RateLimiter is a simple in-memory sliding-window limiter keyed by an arbitrary
// string. It is safe for the single-replica license server (no leader election);
// a multi-replica deployment would need a shared store. Memory is bounded by an
// opportunistic sweep of keys idle beyond the window.
type RateLimiter struct {
	mu        sync.Mutex
	limit     int
	window    time.Duration
	hits      map[string][]time.Time
	lastSweep time.Time
}

// NewRateLimiter creates a limiter allowing at most limit events per window per key.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{limit: limit, window: window, hits: map[string][]time.Time{}}
}

// Allow records an event for key and reports whether it is within the limit.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	rl.sweep(now)

	cutoff := now.Add(-rl.window)

	kept := rl.hits[key][:0]
	for _, t := range rl.hits[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}

	if len(kept) >= rl.limit {
		rl.hits[key] = kept

		return false
	}

	rl.hits[key] = append(kept, now)

	return true
}

// sweep prunes keys with no events inside the window, at most once per window, to
// keep the map from growing without bound as new emails/IPs appear.
func (rl *RateLimiter) sweep(now time.Time) {
	if now.Sub(rl.lastSweep) < rl.window {
		return
	}

	rl.lastSweep = now
	cutoff := now.Add(-rl.window)

	for k, ts := range rl.hits {
		alive := ts[:0]
		for _, t := range ts {
			if t.After(cutoff) {
				alive = append(alive, t)
			}
		}

		if len(alive) == 0 {
			delete(rl.hits, k)
		} else {
			rl.hits[k] = alive
		}
	}
}

// SignupRateLimit throttles public signup requests by admin email (primary) and
// client IP (secondary). Email is the effective limiter when the request arrives
// via the portal's server-side BFF (all such requests share the portal's IP); the
// BFF should forward the end-user IP via X-Forwarded-For for the per-IP guard to
// bite on direct browser calls. It peeks the body to read the email, then restores
// it for the handler.
func SignupRateLimit(perEmail, perIP *RateLimiter, logger *log.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			if !perIP.Allow(ip) {
				logger.Printf("signup rate limit: ip %s throttled", ip)
				tooManyRequests(w)

				return
			}

			body, _ := io.ReadAll(io.LimitReader(r.Body, maxSignupProbeBody))
			r.Body = io.NopCloser(bytes.NewReader(body))

			var probe struct {
				AdminEmail string `json:"admin_email"`
			}
			_ = json.Unmarshal(body, &probe)

			email := strings.ToLower(strings.TrimSpace(probe.AdminEmail))
			if email != "" && !perEmail.Allow(email) {
				logger.Printf("signup rate limit: email %s throttled", email)
				tooManyRequests(w)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func tooManyRequests(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	_, _ = w.Write([]byte(`{"error":"too many signup requests, please try again later"}`))
}

// clientIP returns the best-effort originating client IP: the first hop of
// X-Forwarded-For when present (the license server runs behind a load balancer),
// else the connection's RemoteAddr host.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}

		return strings.TrimSpace(xff)
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}
