package main

import (
	"fmt"
	"os"

	"github.com/shairozan/janus/internal/config"
)

func main() {
	// 1. Parse arguments - separate executor flags from container args
	execFlags, containerArgs := parseExecutorFlags(os.Args[1:])

	if execFlags.RetiredLicense {
		fmt.Fprintln(os.Stderr,
			"Warning: --executor-license is no longer used and was ignored. "+
				"Janus does not require a license; you can drop the flag.")
	}

	// 2. Handle executor-specific commands
	if execFlags.Help {
		showExecutorHelp()
		os.Exit(0)
	}

	if execFlags.Version {
		showExecutorVersion()
		os.Exit(0)
	}

	// 2a. Headless qualification (IQ/OQ) — self-contained, does not touch the
	// Hermes container flow below.
	if execFlags.RunIQ || execFlags.RunOQ {
		os.Exit(runQualification(execFlags))
	}

	// 3. If no arguments, show help
	if len(containerArgs) == 0 && execFlags.HermesConfig == "" {
		showExecutorHelp()
		os.Exit(0)
	}

	// 4. Find Hermes config (explicit or heuristic discovery)
	configPath, modelPath, err := findHermesConfig(containerArgs, execFlags.HermesConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// 5. Load the full model configuration (retain is a top-level, model-wide
	// property, so we need the whole ModelConfig, not just the hermes section).
	modelCfg, err := config.LoadModelConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load model config: %v\n", err)
		os.Exit(1)
	}

	if modelCfg.Hermes == nil {
		fmt.Fprintf(os.Stderr, "Hermes execution requires a hermes section in %s\n", configPath)
		os.Exit(1)
	}

	if err := modelCfg.Hermes.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid Hermes config: %v\n", err)
		os.Exit(1)
	}

	// 6. Execute via Hermes
	exitCode := executeHermes(modelCfg.Hermes, modelCfg.Retain, modelPath, containerArgs, execFlags)
	os.Exit(exitCode)
}
