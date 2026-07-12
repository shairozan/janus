package main

import (
	"context"
	"embed"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/viper"

	"github.com/pharmalytica/janus/cmd"
)

// EmbeddedAssets contains files embedded at build time
//
//go:embed .license_public_key.pem
var EmbeddedAssets embed.FS

func main() {
	// Set up environment variable prefix
	viper.SetEnvPrefix("JANUS")
	viper.AutomaticEnv()

	// Create a context that cancels on SIGINT/SIGTERM
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	// Execute the command with signal-aware context and embedded assets
	rootCmd := cmd.Command(EmbeddedAssets)
	err := rootCmd.ExecuteContext(ctx)

	// Always call cancel to clean up resources
	cancel()

	// Handle error after cleanup
	if err != nil {
		log.Printf("Error: %s", err.Error())
		os.Exit(1)
	}
}
