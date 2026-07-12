package main

import (
	"fmt"
	"os"

	"github.com/pharmalytica/janus/cmd/license-server/commands"
)

func main() {
	rootCmd := commands.RootCommand()

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}