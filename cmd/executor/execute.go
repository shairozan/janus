package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/shairozan/janus/internal/config"
	"github.com/shairozan/janus/internal/execution"
	"github.com/shairozan/janus/internal/runlog"
)

// executeHermes orchestrates Hermes container execution.
// This is the core execution logic that:
// 1. Builds the Hermes payload
// 2. Creates and configures the executor
// 3. Sets up output streaming
// 4. Executes the model
// 5. Updates run log (if enabled)
// 6. Returns the exit code.
func executeHermes(hermesConfig *config.HermesExecutionConfig, retain []string, modelPath string, args []string, flags ExecutorFlags) int {
	// Create context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle SIGINT/SIGTERM gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Fprintln(os.Stderr, "\nInterrupted, stopping execution...")
		cancel()
	}()

	// Load Janus config for Docker socket and other settings
	janusConfig, err := loadJanusConfig(flags.JanusConfig)

	switch {
	case err != nil:
		fmt.Fprintf(os.Stderr, "Warning: Failed to load Janus config: %v\n", err)
		fmt.Fprintln(os.Stderr, "  Continuing with defaults...")
		janusConfig = nil
	case janusConfig != nil:
		// Debug: Show that config was loaded (only if explicitly enabled)
		debugEnv := strings.ToLower(os.Getenv("EXECUTOR_DEBUG"))
		if debugEnv == "true" || debugEnv == "1" {
			fmt.Fprintf(os.Stderr, "Loaded Janus config - Docker socket: %s\n", janusConfig.Hermes.Container.DockerSocket)
		}
	default:
		// No config found - use defaults
		debugEnv := strings.ToLower(os.Getenv("EXECUTOR_DEBUG"))
		if debugEnv == "true" || debugEnv == "1" {
			fmt.Fprintln(os.Stderr, "No Janus config found - using defaults")
		}
	}

	// Create Hermes executor
	// Pass janusConfig to get Docker socket and other Hermes settings
	executor := execution.NewHermesExecutor(janusConfig, hermesConfig)

	// Wire the model's retain (per-model tier) so collection honors it.
	executor.SetModelRetain(retain)

	// Setup output streaming (default behavior, unless --executor-quiet)
	if !flags.Quiet {
		executor.SetOutputWriters(os.Stdout, os.Stderr)
	}

	// Filter out model/output files from args - they're already provided via modelPath
	// Only pass actual NONMEM options (e.g., -maxeval=9999, -clean=3, etc.)
	var additionalOptions []string
	modelFileName := filepath.Base(modelPath)
	outputFileName := strings.TrimSuffix(modelFileName, filepath.Ext(modelFileName)) + ".lst"

	for _, arg := range args {
		// Skip if it's the model file or output file
		if arg == modelPath || arg == modelFileName || arg == outputFileName {
			continue
		}
		// Keep everything else (NONMEM options, etc.)
		additionalOptions = append(additionalOptions, arg)
	}

	// Apply the pre-run output policy (sequential archiving). No-op by default.
	applyRunPolicy(janusConfig, modelPath, false)

	// Execute the model
	// Only pass actual options (not the model/output files)
	result, err := executor.Execute(ctx, modelPath, false, 0, false, additionalOptions)
	if err != nil {
		// Avoid printing the raw error, which may leak container or configuration
		// details (image names, paths, env). Report only the error type for triage.
		fmt.Fprintf(os.Stderr, "Execution failed (error type: %T)\n", err)

		return 1
	}

	// Apply the post-run output policy (backup, cleanup). No-op by default.
	applyRunPolicy(janusConfig, modelPath, true)

	// If execution was quiet, show buffered output now
	if flags.Quiet {
		if len(result.Stdout) > 0 {
			fmt.Printf("\n📤 Standard Output:\n")      //nolint:forbidigo // CLI user output
			fmt.Printf("%s\n", string(result.Stdout)) //nolint:forbidigo // CLI user output
		}
		if len(result.Stderr) > 0 {
			fmt.Printf("\n⚠️  Standard Error:\n")     //nolint:forbidigo // CLI user output
			fmt.Printf("%s\n", string(result.Stderr)) //nolint:forbidigo // CLI user output
		}
	}

	// Update run log if enabled and file exists
	if !flags.NoRunlog {
		updateRunLogIfExists(modelPath, hermesConfig, result, args)
	}

	return result.ExitCode
}

// applyRunPolicy applies the configured run-output policy (sequential archiving
// before a run; backup + cleanup after), so the headless executor honors the
// same policy as the GUI. Failures are logged but never block execution.
func applyRunPolicy(cfg *config.Config, modelPath string, after bool) {
	if cfg == nil {
		return
	}

	policy := cfg.RunPolicy()

	if after {
		if err := policy.AfterRun(modelPath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: run-output policy (after run) failed: %v\n", err)
		}

		return
	}

	if _, err := policy.BeforeRun(modelPath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: run-output policy (before run) failed: %v\n", err)
	}
}

// updateRunLogIfExists conditionally updates the run log if it exists.
// This is opt-in behavior: only updates if .janus.runlog.json already exists.
func updateRunLogIfExists(modelPath string, hermesConfig *config.HermesExecutionConfig, result *execution.ExecutionResult, args []string) {
	modelDir := filepath.Dir(modelPath)
	runLogPath := filepath.Join(modelDir, ".janus.runlog.json")

	// Check if runlog exists
	if _, err := os.Stat(runLogPath); err != nil {
		// Runlog doesn't exist - skip silently
		return
	}

	// Load existing runlog
	logger, err := runlog.NewRunLogger(true, runLogPath)
	if err != nil {
		// Failed to load runlog - skip silently (non-blocking)
		return
	}
	defer logger.Close()

	// Record Hermes execution
	jobID := logger.GenerateJobID()

	hermesMetadata := &runlog.HermesMetadata{
		ExecutionID: jobID,
		ContainerID: "", // Not tracked by executor
		Image: runlog.ContainerImage{
			Full: hermesConfig.Image,
			// Name/Tag/Digest would require parsing the image string
		},
		Resources: runlog.HermesResources{
			CPUCores: hermesConfig.Resources.CPUCores,
			Memory:   hermesConfig.Resources.Memory,
		},
		ModelConfig: filepath.Join(modelDir, ".janus.config.json"),
		FilesCount:  0, // Not tracked by executor
		RuntimeSec:  0, // Not tracked by executor
	}

	// Record execution (duration not tracked for executor - set to 0)
	err = logger.RecordHermesExecution(
		jobID,
		"executor",
		args,
		string(result.Stdout),
		string(result.Stderr),
		result.ExitCode,
		0, // duration
		modelDir,
		hermesMetadata,
		[]string{}, // output files not tracked by executor
	)

	if err != nil {
		// Failed to record - skip silently (non-blocking)
		return
	}
}
