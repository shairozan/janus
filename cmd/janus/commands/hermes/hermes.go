package hermes

import (
	"github.com/spf13/cobra"
)

// Command returns the parent hermes command.
func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hermes",
		Short: "Manage Hermes container execution configuration",
		Long: `Manage Hermes container execution configuration.

Hermes is a container-based execution system that allows running NONMEM
models in isolated, reproducible environments.

Use 'janus hermes init' to create a configuration file for a model,
then 'janus execute hermes' to run the model.`,
	}

	// Add subcommands
	cmd.AddCommand(initCommand())

	return cmd
}
