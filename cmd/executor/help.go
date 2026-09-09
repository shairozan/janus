package main

import (
	"fmt"
	"runtime"

	"github.com/shairozan/janus/internal/version"
)

// showExecutorHelp displays help text for the executor binary.
// This is shown when --executor-help is used or when executor is run with no arguments.
//
//nolint:forbidigo // CLI help output - intentional user-facing text
func showExecutorHelp() {
	fmt.Print(`executor - Generic container orchestration for pharmacometric tools

USAGE:
    executor [EXECUTOR-FLAGS] [TOOL-ARGS...]

EXECUTOR FLAGS (never passed to container):
    --executor-help                  Show this help message
    --executor-version               Show executor version
    --executor-janus-config PATH     Janus config file for Docker socket and settings
    --executor-quiet                 Suppress output streaming
    --executor-hermes-config PATH    Explicit Hermes config file location
    --executor-no-runlog             Skip run log update

CONFIGURATION:
    executor requires a .janus.config.json file to specify:
    - Container image (NONMEM, Stan, Torsten, Monolix, etc.)
    - Compute resources (CPU cores, memory)
    - Tool-specific entry points

    Create config with: janus hermes init <model-file>

    Optional Janus config (~/.config/janus/config.yaml) provides:
    - Docker socket location (for Docker Desktop on Linux)
    - Default Hermes settings

CONFIG DISCOVERY:
    Hermes config (.janus.config.json):
    1. Check --executor-hermes-config if specified
    2. Scan TOOL-ARGS for file paths, look for .janus.config.json in same directory
    3. Fall back to current working directory
    4. Error if not found

    Janus config (config.yaml):
    1. Check --executor-janus-config if specified
    2. Check $JANUS_CONFIG environment variable
    3. Check ~/.config/janus/config.yaml
    4. Check ./janus.yaml
    5. Use defaults if not found (not an error)

EXAMPLES:
    # NONMEM execution (config in same directory as model.mod)
    executor model.mod model.lst

    # Stan with options (all passed to Stan)
    executor model.stan --algorithm=hmc --num_samples=2000

    # Explicit config location
    executor --executor-hermes-config /shared/config.json model.mod

    # Quiet mode (no streaming)
    executor --executor-quiet model.mod model.lst

TOOL ARGUMENTS:
    All arguments not starting with --executor- are passed directly to the
    container tool unchanged. executor does not interpret tool-specific flags.

MORE INFO:
    Documentation: https://github.com/shairozan/janus
    Report issues: https://github.com/shairozan/janus/issues
`)
}

// showExecutorVersion displays version information for the executor binary.
// This is shown when --executor-version is used.
//
//nolint:forbidigo // CLI version output - intentional user-facing text
func showExecutorVersion() {
	fmt.Printf("executor %s\n", version.Version)
	fmt.Printf("  commit: %s\n", version.Commit)
	fmt.Printf("  built: %s\n", version.Date)
	fmt.Printf("  by: %s\n", version.BuiltBy)
	fmt.Printf("  go: %s\n", runtime.Version())
}
