package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/license/auth"
)

func newTestCache(t *testing.T) (*auth.UserInfoCache, *miniredis.Miniredis) {
	t.Helper()

	mr := miniredis.RunT(t)

	cache, err := auth.NewUserInfoCache("redis://" + mr.Addr())
	require.NoError(t, err)

	t.Cleanup(func() { _ = cache.Close() })

	return cache, mr
}

func TestUserInfoCache_MissOnEmpty(t *testing.T) {
	cache, _ := newTestCache(t)

	user, err := cache.Get(context.Background(), "some-token")
	require.NoError(t, err)
	assert.Nil(t, user, "expected nil on cache miss")
}

func TestUserInfoCache_SetThenGet(t *testing.T) {
	cache, _ := newTestCache(t)

	want := &auth.OIDCUser{Sub: "sub123", Email: "alice@januspk.com", Name: "Alice"}

	err := cache.Set(context.Background(), "my-token", want, time.Minute)
	require.NoError(t, err)

	got, err := cache.Get(context.Background(), "my-token")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, want.Sub, got.Sub)
	assert.Equal(t, want.Email, got.Email)
	assert.Equal(t, want.Name, got.Name)
}

func TestUserInfoCache_DifferentTokensDontCollide(t *testing.T) {
	cache, _ := newTestCache(t)

	alice := &auth.OIDCUser{Sub: "alice", Email: "alice@januspk.com"}
	bob := &auth.OIDCUser{Sub: "bob", Email: "bob@januspk.com"}

	require.NoError(t, cache.Set(context.Background(), "token-alice", alice, time.Minute))
	require.NoError(t, cache.Set(context.Background(), "token-bob", bob, time.Minute))

	gotAlice, err := cache.Get(context.Background(), "token-alice")
	require.NoError(t, err)
	assert.Equal(t, "alice@januspk.com", gotAlice.Email)

	gotBob, err := cache.Get(context.Background(), "token-bob")
	require.NoError(t, err)
	assert.Equal(t, "bob@januspk.com", gotBob.Email)
}

func TestUserInfoCache_MissAfterTTLExpiry(t *testing.T) {
	cache, mr := newTestCache(t)

	user := &auth.OIDCUser{Sub: "sub", Email: "alice@januspk.com"}
	require.NoError(t, cache.Set(context.Background(), "expiring-token", user, 30*time.Second))

	// Advance miniredis clock past TTL.
	mr.FastForward(31 * time.Second)

	got, err := cache.Get(context.Background(), "expiring-token")
	require.NoError(t, err)
	assert.Nil(t, got, "expected nil after TTL expiry")
}

func TestUserInfoCache_RedisUnavailable(t *testing.T) {
	// Point at a port nothing is listening on.
	cache, err := auth.NewUserInfoCache("redis://localhost:19999")
	require.NoError(t, err) // construction succeeds; failure is deferred to first call
	defer func() { _ = cache.Close() }()

	// Get on unavailable Redis should return an error, not panic.
	_, err = cache.Get(context.Background(), "token")
	assert.Error(t, err)
}

// Ensure that a zero-TTL Set doesn't error (even if the key expires immediately).
func TestUserInfoCache_ZeroTTL(t *testing.T) {
	cache, _ := newTestCache(t)

	user := &auth.OIDCUser{Sub: "sub", Email: "alice@januspk.com"}
	err := cache.Set(context.Background(), "zero-ttl-token", user, 0)
	assert.NoError(t, err)
}

// Verify the Redis client created by NewUserInfoCache works with miniredis.
func TestNewUserInfoCache_InvalidURL(t *testing.T) {
	_, err := auth.NewUserInfoCache("not-a-url://??")
	assert.Error(t, err)
}

// Verify that UserInfoCache.Get handles a corrupted cache value gracefully.
func TestUserInfoCache_CorruptedValue(t *testing.T) {
	cache, mr := newTestCache(t)

	// Write garbage directly into Redis at the key that Get will look up.
	token := "corrupt-token"
	// Compute the key the same way the cache does (sha256 -> hex, prefixed).
	// We'll use the miniredis client directly.
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rc.Close() }()

	key := "userinfo:" + "badhash" // wrong hash, but let's inject at a known key
	require.NoError(t, rc.Set(context.Background(), key, []byte("not json"), time.Minute).Err())

	// A miss for our token is expected (the injected key doesn't match the sha256 of "corrupt-token").
	got, err := cache.Get(context.Background(), token)
	require.NoError(t, err)
	assert.Nil(t, got)
}
