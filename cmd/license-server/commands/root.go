package commands

import (
	"github.com/spf13/cobra"
)

// RootCommand creates the root command for license-server.
func RootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "license-server",
		Short: "Janus license server for managing organizations, licenses, and tokens",
		Long: `License server for Janus - manages customer organizations, license agreements,
and JWT token generation/validation.

The license server operates in two modes:
  1. CLI mode: Manage organizations, licenses, and generate tokens
  2. Server mode: HTTP API for token validation and usage tracking

Examples:
  # Start the license server
  license-server serve --database-url="postgres://..."

  # Create an organization
  license-server org create --name="Acme Corp"

  # Generate a token
  license-server token generate --org-id=1 --user="jane@acme.com"
`,
	}

	// Add subcommands
	cmd.AddCommand(ServeCommand())
	cmd.AddCommand(MigrateCommand())
	cmd.AddCommand(VersionCommand())

	return cmd
}