package gui

import (
	"fmt"
	"log"

	"github.com/pharmalytica/janus/internal/config"
	"github.com/pharmalytica/janus/internal/mcp"
	"github.com/pharmalytica/janus/internal/version"
)

// StartMCPServer starts the embedded MCP server if it is enabled in config. It is
// idempotent (a no-op if already running) and a no-op when MCP is disabled.
//
// When no auth token is configured it generates one with crypto/rand and persists
// it back to the config file, so the token is stable across restarts and the user
// pairs an agent only once. A start failure (e.g. a port conflict) is returned to
// the caller, which logs it non-fatally.
func (a *App) StartMCPServer() error {
	if a.config == nil || !a.config.MCP.Enabled {
		return nil
	}

	a.mcpMu.Lock()
	defer a.mcpMu.Unlock()

	if a.mcpServer != nil && a.mcpServer.Running() {
		return nil
	}

	token, err := a.ensureMCPToken()
	if err != nil {
		return fmt.Errorf("failed to prepare MCP auth token: %w", err)
	}

	bridge, err := a.buildMCPService()
	if err != nil {
		return fmt.Errorf("failed to build MCP service: %w", err)
	}

	server, err := mcp.NewServer(mcp.Config{
		Host:         a.config.MCP.Host,
		Port:         a.config.MCP.Port,
		AuthToken:    token,
		AllowExecute: a.config.MCP.AllowExecute,
		Version:      version.Get(),
	}, bridge)
	if err != nil {
		return fmt.Errorf("failed to create MCP server: %w", err)
	}

	if err := server.Start(a.errorCtx); err != nil {
		return fmt.Errorf("failed to start MCP server: %w", err)
	}

	a.mcpServer = server
	log.Printf("MCP server listening on %s/mcp (allow_execute=%t)", server.Addr(), a.config.MCP.AllowExecute)

	return nil
}

// StopMCPServer gracefully stops the embedded MCP server if it is running.
func (a *App) StopMCPServer() error {
	a.mcpMu.Lock()
	defer a.mcpMu.Unlock()

	if a.mcpServer == nil {
		return nil
	}

	err := a.mcpServer.Stop()
	a.mcpServer = nil

	return err
}

// MCPServerRunning reports whether the embedded MCP server is currently running.
func (a *App) MCPServerRunning() bool {
	a.mcpMu.Lock()
	defer a.mcpMu.Unlock()

	return a.mcpServer != nil && a.mcpServer.Running()
}

// ensureMCPToken returns the configured bearer token, generating and persisting
// one when none is set. The generated token is written back to the config file
// so it remains stable across restarts. The caller must hold a.mcpMu.
func (a *App) ensureMCPToken() (string, error) {
	if a.config.MCP.AuthToken != "" {
		return a.config.MCP.AuthToken, nil
	}

	token, err := config.GenerateMCPToken()
	if err != nil {
		return "", err
	}

	a.config.MCP.AuthToken = token

	if err := config.PersistMCPToken(token); err != nil {
		// Non-fatal: the server can still run with the in-memory token this
		// session; it will simply be regenerated next start.
		log.Printf("Warning: failed to persist MCP auth token (it will not survive a restart): %v", err)
	} else {
		log.Printf("Generated MCP auth token and saved it to the config file")
	}

	return token, nil
}

// RegenerateMCPToken generates a fresh bearer token, persists it, and returns it.
// Any currently-running server keeps its old token until restarted, so callers
// should stop and start the server to apply the new token.
func (a *App) RegenerateMCPToken() (string, error) {
	if a.config == nil {
		return "", fmt.Errorf("no configuration loaded")
	}

	token, err := config.GenerateMCPToken()
	if err != nil {
		return "", err
	}

	a.config.MCP.AuthToken = token

	if err := config.PersistMCPToken(token); err != nil {
		return "", fmt.Errorf("failed to persist regenerated token: %w", err)
	}

	return token, nil
}
