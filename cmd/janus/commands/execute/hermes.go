//go:build !test
// +build !test

package execute

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/shairozan/janus/internal/config"
	"github.com/shairozan/janus/internal/execution"
)

func hermesCommand() *cobra.Command {
	var cfg *config.Config
	var quiet bool
	var detach bool

	cmd := &cobra.Command{
		Use:   "hermes [model-path]",
		Short: "Execute a model using Hermes container orchestration",
		Long: `Execute a NONMEM model using Hermes container orchestration.

Hermes provides isolated, reproducible execution environments using containers.
The model must have a .janus.config.json file (created with 'janus hermes init').

The execution will:
1. Load configuration from .janus.config.json
2. Parse model dependencies ($DATA files, etc.)
3. Create a workspace with all required files
4. Execute the model in a container
5. Collect results and update the run log

By default, execution output is streamed in real-time (like PSN/BBI).
Use --quiet to suppress output streaming.

Example:
  janus execute hermes /path/to/model.mod           # Stream output (default)
  janus execute hermes model.mod --quiet            # Suppress output, show at end
  janus execute hermes model.mod --detach           # Run in background`,
		Args: cobra.ExactArgs(1),
		PersistentPreRunE: config.NewInitializer(&cfg, config.InitializerOptions{
			ConfigFlagName: "config",
		}),
		RunE: func(c *cobra.Command, args []string) error {
			modelPath := args[0]

			return runHermes(cfg, modelPath, quiet, detach)
		},
	}

	attributes(cmd)

	return cmd
}

func attributes(c *cobra.Command) {
	c.Flags().BoolP("quiet", "q", false, "suppress output streaming, show results at end")
	c.Flags().BoolP("detach", "d", false, "run execution in background")

	_ = viper.BindPFlags(c.Flags())
}

//nolint:forbidigo // CLI command with user-facing output
func runHermes(cfg *config.Config, modelPath string, quiet bool, detach bool) error {
	// Validate flags
	if quiet && detach {
		return fmt.Errorf("cannot use --quiet and --detach together (detached runs are already quiet)")
	}

	// Resolve absolute path to model
	absModelPath, err := filepath.Abs(modelPath)
	if err != nil {
		return fmt.Errorf("failed to resolve model path: %w", err)
	}

	// Check if model exists
	if _, err := os.Stat(absModelPath); os.IsNotExist(err) {
		return fmt.Errorf("model file not found: %s", absModelPath)
	}

	// Load Hermes configuration from .janus.config.json
	modelDir := filepath.Dir(absModelPath)
	configPath := filepath.Join(modelDir, ".janus.config.json")

	modelCfg, err := config.LoadModelConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load Hermes configuration: %w\nHint: run 'janus hermes init %s' to create configuration", err, modelPath)
	}

	if modelCfg.Hermes == nil {
		return fmt.Errorf("hermes execution requires a hermes section in %s\nHint: run 'janus hermes init %s' to create configuration", configPath, modelPath)
	}

	if err := modelCfg.Hermes.Validate(); err != nil {
		return fmt.Errorf("invalid Hermes configuration: %w", err)
	}

	hermesConfig := modelCfg.Hermes

	fmt.Printf("🚀 Executing model with Hermes container orchestration\n")
	fmt.Printf("📦 Container: %s\n", hermesConfig.Image)
	fmt.Printf("💻 Resources: %d cores, %s memory\n", hermesConfig.Resources.CPUCores, hermesConfig.Resources.Memory)
	fmt.Printf("📄 Model: %s\n\n", filepath.Base(absModelPath))

	// Build Hermes payload using the functional core
	payload, err := execution.BuildHermesPayload(absModelPath, hermesConfig)
	if err != nil {
		return fmt.Errorf("failed to build Hermes payload: %w", err)
	}

	fmt.Printf("📂 Workspace contains %d files\n", len(payload.WorkspaceStructure))
	fmt.Printf("🎯 Entry point: %s\n\n", payload.EntryPoint)

	// Create executor
	executor := execution.NewHermesExecutor(cfg, hermesConfig)
	executor.SetModelRetain(modelCfg.Retain)

	// Configure output streaming (default behavior unless quiet or detached)
	streamOutput := !quiet && !detach
	if streamOutput {
		fmt.Println("📡 Streaming execution output...")
		fmt.Println("---")
		executor.SetOutputWriters(os.Stdout, os.Stderr)
	}

	// Set up context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n⚠️  Received interrupt signal, cancelling execution...")
		cancel()
	}()

	// Execute the model
	var result *execution.ExecutionResult
	if detach {
		fmt.Println("🔄 Running in background...")
		result, err = executor.Execute(ctx, absModelPath, false, hermesConfig.Resources.CPUCores, false, nil)
	} else {
		// Execute and wait for completion
		result, err = executor.Execute(ctx, absModelPath, false, hermesConfig.Resources.CPUCores, false, nil)
	}

	// If streaming was enabled, add separator after output
	if streamOutput {
		fmt.Println("---")
	}

	if err != nil {
		fmt.Printf("❌ Execution failed: %v\n", err)

		return err
	}

	// Display results
	fmt.Printf("✅ Execution completed successfully\n")
	fmt.Printf("📊 Exit code: %d\n", result.ExitCode)

	// Only display buffered output if streaming wasn't enabled
	// (otherwise it was already streamed in real-time)
	if !streamOutput {
		// Display stdout if present
		if len(result.Stdout) > 0 {
			fmt.Printf("\n📤 Standard Output:\n")
			fmt.Printf("%s\n", string(result.Stdout))
		}

		// Display stderr if present
		if len(result.Stderr) > 0 {
			fmt.Printf("\n⚠️  Standard Error:\n")
			fmt.Printf("%s\n", string(result.Stderr))
		}
	}

	// Check for run log in model directory
	runLogPath := filepath.Join(filepath.Dir(absModelPath), ".janus.runlog.json")
	if _, err = os.Stat(runLogPath); err == nil {
		fmt.Printf("\n📝 Run log created: %s\n", runLogPath)
	}

	return nil
}
