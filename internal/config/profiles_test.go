package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeConfig(t *testing.T, dir, contents string) string {
	t.Helper()

	path := filepath.Join(dir, "config.yml")
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))

	return path
}

func TestProfilesLifecycle(t *testing.T) {
	dir := t.TempDir()
	cfg := writeConfig(t, dir, "organization: Acme\n")

	// No profiles yet.
	names, err := ListProfiles(cfg)
	require.NoError(t, err)
	assert.Empty(t, names)

	// Save the current config as two profiles.
	require.NoError(t, SaveProfile(cfg, "prod"))

	writeConfig(t, dir, "organization: Dev\n") // change active config
	require.NoError(t, SaveProfile(cfg, "dev"))

	names, err = ListProfiles(cfg)
	require.NoError(t, err)
	assert.Equal(t, []string{"dev", "prod"}, names)

	// Switch back to prod → active config has prod's contents.
	require.NoError(t, SwitchProfile(cfg, "prod"))
	data, err := os.ReadFile(cfg)
	require.NoError(t, err)
	assert.Equal(t, "organization: Acme\n", string(data))

	// Delete a profile.
	require.NoError(t, DeleteProfile(cfg, "dev"))
	names, _ = ListProfiles(cfg)
	assert.Equal(t, []string{"prod"}, names)
}

func TestSaveProfileRequiresName(t *testing.T) {
	cfg := writeConfig(t, t.TempDir(), "x: 1\n")
	assert.Error(t, SaveProfile(cfg, "   "))
}

func TestSwitchUnknownProfile(t *testing.T) {
	cfg := writeConfig(t, t.TempDir(), "x: 1\n")
	assert.Error(t, SwitchProfile(cfg, "ghost"))
}
