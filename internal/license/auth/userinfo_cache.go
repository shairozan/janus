package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const cacheKeyPrefix = "userinfo:"

// UserInfoCache is a Redis-backed write-through cache for OIDC UserInfo
// responses, keyed by a SHA-256 hash of the raw access token.
type UserInfoCache struct {
	client *redis.Client
}

// NewUserInfoCache creates a UserInfoCache connected to the given Redis URL.
// The URL must be in the form redis://[user:password@]host:port/db.
func NewUserInfoCache(redisURL string) (*UserInfoCache, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}

	return &UserInfoCache{client: redis.NewClient(opts)}, nil
}

// Close releases the underlying Redis connection.
func (c *UserInfoCache) Close() error {
	return c.client.Close()
}

// Get retrieves a cached OIDCUser for the given raw access token.
// Returns (nil, nil) on a cache miss.
func (c *UserInfoCache) Get(ctx context.Context, rawToken string) (*OIDCUser, error) {
	key := tokenCacheKey(rawToken)

	data, err := c.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("redis get: %w", err)
	}

	var user OIDCUser
	if err := json.Unmarshal(data, &user); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached user: %w", err)
	}

	return &user, nil
}

// Set stores an OIDCUser in the cache keyed by the raw access token.
func (c *UserInfoCache) Set(ctx context.Context, rawToken string, user *OIDCUser, ttl time.Duration) error {
	key := tokenCacheKey(rawToken)

	data, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("failed to marshal user for cache: %w", err)
	}

	if err := c.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("redis set: %w", err)
	}

	return nil
}

func tokenCacheKey(rawToken string) string {
	h := sha256.Sum256([]byte(rawToken))

	return cacheKeyPrefix + hex.EncodeToString(h[:])
}
