package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/viper"

	"github.com/shairozan/janus/cmd"
)

func main() {
	// Set up environment variable prefix
	viper.SetEnvPrefix("JANUS")
	viper.AutomaticEnv()

	// Create a context that cancels on SIGINT/SIGTERM
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	// Execute the command with a signal-aware context
	rootCmd := cmd.Command()
	err := rootCmd.ExecuteContext(ctx)

	// Always call cancel to clean up resources
	cancel()

	// Handle error after cleanup
	if err != nil {
		log.Printf("Error: %s", err.Error())
		os.Exit(1)
	}
}
