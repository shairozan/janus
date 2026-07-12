package middleware

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/pharmalytica/janus/internal/license/auth"
)

const maxUserInfoCacheTTL = 5 * time.Minute

// UserInfoFunc fetches user info from the OIDC provider using an access token.
// The returned OIDCUser must have Email populated.
type UserInfoFunc func(ctx context.Context, accessToken string) (*auth.OIDCUser, error)

// RequireUserInfo is a per-route middleware that enforces Cognito authentication
// and email-domain authorization. It must run after the Authentication middleware
// has had a chance to set the OIDCUser in context.
//
// On each request it:
//  1. Rejects (401) if no OIDCUser is in context (token was absent or invalid).
//  2. Checks the Redis cache for a previously fetched UserInfo for this token.
//  3. On a cache miss calls fetchUserInfo and writes the result to cache.
//     The TTL is the lesser of the token's remaining lifetime and maxUserInfoCacheTTL.
//  4. Rejects (403) if the email domain is not in allowedDomains.
//  5. Enriches the OIDCUser in context with Email and Name from UserInfo.
func RequireUserInfo(
	fetchUserInfo UserInfoFunc,
	cache *auth.UserInfoCache,
	allowedDomains []string,
	logger *log.Logger,
) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := GetOIDCUser(r)
			if !ok || user == nil {
				writeJSONError(w, http.StatusUnauthorized, "authentication required")

				return
			}

			rawToken := bearerToken(r)

			userInfo, err := resolveUserInfo(r.Context(), rawToken, user, fetchUserInfo, cache, logger)
			if err != nil {
				logger.Printf("userinfo lookup failed for sub %s: %v", user.Sub, err)
				writeJSONError(w, http.StatusUnauthorized, "authentication required")

				return
			}

			if len(allowedDomains) > 0 && !IsAllowedDomain(userInfo.Email, allowedDomains) {
				writeJSONError(w, http.StatusForbidden, "access denied: email domain not authorized")

				return
			}

			enriched := *user
			enriched.Email = userInfo.Email
			enriched.Name = userInfo.Name

			ctx := context.WithValue(r.Context(), contextKeyOIDCUser, &enriched)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// resolveUserInfo returns cached UserInfo or fetches it from the provider,
// writing the result into the cache on a miss.
func resolveUserInfo(
	ctx context.Context,
	rawToken string,
	user *auth.OIDCUser,
	fetch UserInfoFunc,
	cache *auth.UserInfoCache,
	logger *log.Logger,
) (*auth.OIDCUser, error) {
	if cache != nil && rawToken != "" {
		cached, err := cache.Get(ctx, rawToken)
		if err != nil {
			logger.Printf("userinfo cache get error: %v", err)
		} else if cached != nil {
			return cached, nil
		}
	}

	fetched, err := fetch(ctx, rawToken)
	if err != nil {
		return nil, err
	}

	if cache != nil && rawToken != "" {
		ttl := cacheTTL(user)
		if err := cache.Set(ctx, rawToken, fetched, ttl); err != nil {
			logger.Printf("userinfo cache set error: %v", err)
		}
	}

	return fetched, nil
}

// cacheTTL returns the TTL to use for a cached UserInfo entry: the lesser of
// the token's remaining lifetime and maxUserInfoCacheTTL.
func cacheTTL(user *auth.OIDCUser) time.Duration {
	if user.Expiry.IsZero() {
		return maxUserInfoCacheTTL
	}

	remaining := time.Until(user.Expiry)
	if remaining <= 0 {
		return maxUserInfoCacheTTL
	}

	if remaining < maxUserInfoCacheTTL {
		return remaining
	}

	return maxUserInfoCacheTTL
}

// IsAllowedDomain reports whether email's domain is in allowedDomains
// (case-insensitive). It is the single source of truth for the Janus-staff
// email-domain gate — reused to compute the /me is_staff flag so the UI and the
// route enforcement agree.
func IsAllowedDomain(email string, allowedDomains []string) bool {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 || parts[1] == "" {
		return false
	}

	domain := strings.ToLower(parts[1])

	for _, allowed := range allowedDomains {
		if strings.ToLower(allowed) == domain {
			return true
		}
	}

	return false
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":"` + msg + `"}`))
}
