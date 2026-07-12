package version

import (
	"log"

	"github.com/spf13/cobra"

	"github.com/pharmalytica/janus/internal/version"
)

func Command() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version number of Janus",
		Long:  `Print the version number of Janus`,
		Run: func(cmd *cobra.Command, args []string) {
			log.Printf("Janus %s", version.Get())
		},
	}
}
