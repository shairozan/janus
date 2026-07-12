package pirana

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocateConfigPrefersPiranaHome(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(home, "settings.db"), []byte("x"), 0o600))

	t.Setenv(piranaHomeEnv, home)

	got, err := locateConfig()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, "settings.db"), got)
}

func TestLocateConfigPrefersSettingsDBOverINI(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(home, "settings.db"), []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(home, "pirana.ini"), []byte("x"), 0o600))

	t.Setenv(piranaHomeEnv, home)

	got, err := locateConfig()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, "settings.db"), got)
}

func TestLocateConfigFallsBackToINI(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(home, "pirana.ini"), []byte("x"), 0o600))

	t.Setenv(piranaHomeEnv, home)

	got, err := locateConfig()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, "pirana.ini"), got)
}

func TestLocateConfigNotFound(t *testing.T) {
	// Point every candidate (including the OS home dir) at empty directories so
	// nothing is found, regardless of the developer's real ~/.pirana.
	empty := t.TempDir()
	t.Setenv(piranaHomeEnv, empty)
	t.Setenv("HOME", empty)
	t.Setenv("USERPROFILE", empty)
	t.Setenv("LOCALAPPDATA", empty)
	t.Setenv("APPDATA", empty)

	_, err := locateConfig()
	assert.True(t, errors.Is(err, ErrNotFound), "expected ErrNotFound, got %v", err)
}
