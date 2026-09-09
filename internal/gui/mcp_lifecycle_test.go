package gui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shairozan/janus/internal/config"
)

func TestEnsureMCPTokenReturnsExisting(t *testing.T) {
	app := &App{config: &config.Config{Input: config.Input{MCP: config.MCPConfig{AuthToken: "preset-token"}}}}

	token, err := app.ensureMCPToken()
	require.NoError(t, err)
	assert.Equal(t, "preset-token", token, "an existing token must be reused, not regenerated")
}

func TestStartMCPServerDisabledIsNoOp(t *testing.T) {
	app := &App{config: &config.Config{Input: config.Input{MCP: config.MCPConfig{Enabled: false}}}}

	require.NoError(t, app.StartMCPServer())
	assert.False(t, app.MCPServerRunning())
}

func TestStopMCPServerWhenNotRunning(t *testing.T) {
	app := &App{}
	require.NoError(t, app.StopMCPServer())
}
