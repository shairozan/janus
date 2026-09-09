// Package mcp provides the `janus mcp` subcommands for the embedded Model Context
// Protocol server. The leaf commands are thin: load config, call a config-layer
// primitive, print, and set the exit code via the returned error.
package mcp

import (
	"embed"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/pharmalytica/janus/internal/config"
)

// Command returns the `mcp` parent command. assets carries the embedded license
// public keys, needed by the `server` subcommand to validate the license.
func Command(assets embed.FS) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Manage the embedded MCP server",
		Long: `Manage Janus's embedded Model Context Protocol (MCP) server, which exposes the
run log to local agents (e.g. Claude Code) over loopback HTTP.

  mcp token    Print the MCP bearer token, provisioning one if not yet set.
  mcp server   Run the MCP server headlessly (no GUI), e.g. as a systemd unit.`,
	}

	cmd.AddCommand(tokenCommand())
	cmd.AddCommand(serverCommand(assets))

	return cmd
}

func tokenCommand() *cobra.Command {
	var cfg *config.Config

	cmd := &cobra.Command{
		Use:   "token",
		Short: "Print the MCP bearer token (provisioning one if unset)",
		Long: `Print the MCP server's bearer token to stdout.

The bearer token is the credential the MCP server checks on every request — it is
distinct from your Janus license. If no token is configured yet, a cryptographically
random one is generated, saved to the config file (so it is stable across restarts),
and printed.

Because it prints only the token to stdout, it is suitable for command substitution
when registering the server with an agent:

  claude mcp add --transport http janus http://127.0.0.1:8731/mcp \
    --header "Authorization: Bearer $(janus mcp token)"`,
		Args:              cobra.NoArgs,
		SilenceUsage:      true,
		PersistentPreRunE: config.NewInitializer(&cfg, config.InitializerOptions{ConfigFlagName: "config", SuppressOutput: true}),
		RunE: func(c *cobra.Command, _ []string) error {
			return runToken(c.OutOrStdout(), cfg)
		},
	}

	return cmd
}

// runToken prints the MCP bearer token, provisioning and persisting one when none
// is configured. It writes only the token to out so it composes in $(...).
func runToken(out io.Writer, cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("no configuration found; run 'janus gui' to set up Janus first")
	}

	token, _, err := config.ProvisionMCPToken(&cfg.MCP)
	if err != nil {
		return err
	}

	fmt.Fprintln(out, token)

	return nil
}
