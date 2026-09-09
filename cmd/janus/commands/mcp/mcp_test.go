package mcp

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/config"
)

func withTempViperConfig(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yml")
	require.NoError(t, os.WriteFile(path, []byte("mcp:\n  enabled: true\n"), 0600))

	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.SetConfigFile(path)
	require.NoError(t, viper.ReadInConfig())

	return path
}

func TestRunTokenNilConfig(t *testing.T) {
	var out bytes.Buffer
	err := runToken(&out, nil)
	require.Error(t, err)
	assert.Empty(t, out.String())
}

func TestRunTokenProvisionsAndPrints(t *testing.T) {
	path := withTempViperConfig(t)

	var out bytes.Buffer
	cfg := &config.Config{Input: config.Input{MCP: config.MCPConfig{Enabled: true}}}

	require.NoError(t, runToken(&out, cfg))

	token := strings.TrimSpace(out.String())
	assert.NotEmpty(t, token)
	// Only the token is printed (no extra chatter) so it composes in $(...).
	assert.Equal(t, 1, len(strings.Split(token, "\n")))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), token)
}

func TestRunTokenPrintsExisting(t *testing.T) {
	withTempViperConfig(t)

	var out bytes.Buffer
	cfg := &config.Config{Input: config.Input{MCP: config.MCPConfig{Enabled: true, AuthToken: "preset-token"}}}

	require.NoError(t, runToken(&out, cfg))
	assert.Equal(t, "preset-token", strings.TrimSpace(out.String()))
}
