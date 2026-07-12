package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pharmalytica/janus/internal/license/version"
)

// VersionCommand creates the version command.
func VersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Display version information",
		Long:  `Display the license server version, commit, build date, and Go version.`,
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Println(version.GetFull()) //nolint:forbidigo // CLI version output
		},
	}
}
