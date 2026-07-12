package commands

import (
	"fmt"

	"github.com/pharmalytica/janus/internal/license/version"
	"github.com/spf13/cobra"
)

// VersionCommand creates the version command.
func VersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Display version information",
		Long:  `Display the license server version, commit, build date, and Go version.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(version.GetFull())
		},
	}
}
