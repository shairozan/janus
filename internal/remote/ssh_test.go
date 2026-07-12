package remote

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/config"
	"github.com/pharmalytica/janus/internal/scheduler"
)

func slurmProfile(t *testing.T) scheduler.Profile {
	t.Helper()

	p, err := scheduler.Resolve("SLURM", nil)
	require.NoError(t, err)

	return p
}

func writeTestKey(t *testing.T) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	path := filepath.Join(t.TempDir(), "id_rsa")
	require.NoError(t, os.WriteFile(path, pemBytes, 0o600))

	return path
}

func TestNewSSHClientRequiresHostAndKey(t *testing.T) {
	_, err := NewSSHClient(slurmProfile(t), config.RemoteConfig{KeyPath: "/x"})
	assert.Error(t, err) // missing host

	_, err = NewSSHClient(slurmProfile(t), config.RemoteConfig{Host: "h"})
	assert.Error(t, err) // missing key
}

func TestNewSSHClientRejectsBadKey(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "bad")
	require.NoError(t, os.WriteFile(bad, []byte("not a key"), 0o600))

	_, err := NewSSHClient(slurmProfile(t), config.RemoteConfig{Host: "h", KeyPath: bad})
	assert.Error(t, err)
}

func TestNewSSHClientLoadsValidKey(t *testing.T) {
	c, err := NewSSHClient(slurmProfile(t), config.RemoteConfig{
		Host:    "cluster.example.com",
		User:    "jane",
		KeyPath: writeTestKey(t),
	})
	require.NoError(t, err)
	assert.Equal(t, defaultSSHPort, c.runner.port())
}

func TestNewRunnerLoadsKeyAndPort(t *testing.T) {
	_, err := NewRunner(config.RemoteConfig{KeyPath: writeTestKey(t)})
	assert.Error(t, err) // missing host

	r, err := NewRunner(config.RemoteConfig{Host: "h", KeyPath: writeTestKey(t), Port: 2222})
	require.NoError(t, err)
	assert.Equal(t, 2222, r.port())
}

func TestSSHClientPortOverride(t *testing.T) {
	c, err := NewSSHClient(slurmProfile(t), config.RemoteConfig{
		Host:    "h",
		KeyPath: writeTestKey(t),
		Port:    2222,
	})
	require.NoError(t, err)
	assert.Equal(t, 2222, c.runner.port())
}
