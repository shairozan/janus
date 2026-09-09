package mcp

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/shairozan/janus/internal/appsetup"
	"github.com/shairozan/janus/internal/config"
	"github.com/shairozan/janus/internal/mcp"
	"github.com/shairozan/janus/internal/mcpservice"
	"github.com/shairozan/janus/internal/version"
)

// serverCommand returns the `mcp server` command, which runs the MCP server
// headlessly (no GUI) until it receives SIGINT/SIGTERM. It coexists with a
// running GUI: the run-log store serializes writes across processes with a file
// lock, so neither the daemon nor the GUI is the privileged writer.
func serverCommand() *cobra.Command {
	var cfg *config.Config

	cmd := &cobra.Command{
		Use:   "server",
		Short: "Run the MCP server headlessly (no GUI)",
		Long: `Run Janus's Model Context Protocol server as a foreground process with no GUI,
suitable for a systemd --user unit on a workbench (e.g. Metworx) or a laptop.

It binds loopback only and requires the same bearer token as the GUI server
(printed by 'janus mcp token'). Execution is gated: pass --allow-execute (or set
mcp.allow_execute) to let agents launch runs. The server runs until it receives
SIGINT or SIGTERM, then shuts down gracefully.

Example systemd --user unit (~/.config/systemd/user/janus-mcp.service):

  [Unit]
  Description=Janus MCP server
  After=network.target

  [Service]
  ExecStart=%h/.local/bin/janus mcp server --allow-execute
  Restart=on-failure

  [Install]
  WantedBy=default.target

Then: systemctl --user enable --now janus-mcp.service`,
		Args:              cobra.NoArgs,
		SilenceUsage:      true,
		PersistentPreRunE: config.NewInitializer(&cfg, config.InitializerOptions{ConfigFlagName: "config", SuppressOutput: true}),
		RunE: func(c *cobra.Command, _ []string) error {
			return runServer(c, cfg)
		},
	}

	cmd.Flags().Bool("allow-execute", false, "allow agents to launch runs (execute_run); overrides config when set")
	cmd.Flags().String("host", "", "loopback bind host (overrides config; default 127.0.0.1)")
	cmd.Flags().Int("port", 0, "bind port (overrides config; default 8731)")

	return cmd
}

func runServer(c *cobra.Command, cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("no configuration found; run 'janus gui' to set up Janus first")
	}

	// Running this command is the explicit intent to serve, regardless of the
	// mcp.enabled flag (which only gates GUI auto-start). Apply flag overrides,
	// then normalize/validate host and port.
	cfg.MCP.Enabled = true

	if host, _ := c.Flags().GetString("host"); host != "" {
		cfg.MCP.Host = host
	}

	if port, _ := c.Flags().GetInt("port"); port != 0 {
		cfg.MCP.Port = port
	}

	if allow, _ := c.Flags().GetBool("allow-execute"); allow {
		cfg.MCP.AllowExecute = true
	}

	if err := config.ValidateMCPConfig(&cfg.MCP); err != nil {
		return fmt.Errorf("invalid MCP configuration: %w", err)
	}

	// Build the run-log signer (non-fatal: continue unsigned on failure).
	signer, err := appsetup.BuildSigner(cfg)
	if err != nil {
		log.Printf("Warning: run-log signing disabled: %v", err)
	}

	signerEmail := ""
	if signer != nil {
		signerEmail = appsetup.SignerIdentity(cfg)
	}

	// Provision the bearer token (generate + persist if unset).
	token, generated, err := config.ProvisionMCPToken(&cfg.MCP)
	if err != nil {
		return fmt.Errorf("preparing MCP auth token: %w", err)
	}

	if generated {
		log.Printf("Generated and saved a new MCP auth token (retrieve it with 'janus mcp token')")
	}

	// Run until SIGINT/SIGTERM; the context drives graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	resolver := mcpservice.NewEphemeralResolver(signer, signerEmail)

	service, err := mcpservice.New(mcpservice.Options{
		Config:  cfg,
		Resolve: resolver.Resolve,
		AppCtx:  ctx,
		ErrSink: func(e error) { log.Printf("mcp: %v", e) },
	})
	if err != nil {
		return fmt.Errorf("building MCP service: %w", err)
	}

	server, err := mcp.NewServer(mcp.Config{
		Host:         cfg.MCP.Host,
		Port:         cfg.MCP.Port,
		AuthToken:    token,
		AllowExecute: cfg.MCP.AllowExecute,
		Version:      version.Get(),
	}, service)
	if err != nil {
		return fmt.Errorf("creating MCP server: %w", err)
	}

	if err := server.Start(ctx); err != nil {
		return fmt.Errorf("starting MCP server: %w", err)
	}

	log.Printf("MCP server listening on http://%s/mcp (allow_execute=%t); press Ctrl-C to stop",
		server.Addr(), cfg.MCP.AllowExecute)

	<-ctx.Done()
	log.Printf("Shutting down MCP server...")

	if err := server.Stop(); err != nil {
		return fmt.Errorf("stopping MCP server: %w", err)
	}

	return nil
}

