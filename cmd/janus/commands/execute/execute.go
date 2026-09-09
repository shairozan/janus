package execute

import (
	"github.com/spf13/cobra"
)

// Command returns the execute command following the Cobra factory pattern.
func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "execute",
		Short: "Execute NONMEM models",
		Long: `Execute NONMEM models using various execution methods.

Supported execution methods:
- hermes: Container-based execution with resource isolation
- local:   Direct local execution (future)
- remote:  Remote grid execution (future)

Use 'janus execute [method] --help' for more information about each method.`,
	}

	// Add subcommands
	cmd.AddCommand(hermesCommand())

	return cmd
}
