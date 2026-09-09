package main

import (
	"strings"
)

// ExecutorFlags contains flags specific to executor operation.
// These flags are never passed to the container - they control executor's behavior.
type ExecutorFlags struct {
	HermesConfig string // Explicit config path via --executor-hermes-config
	JanusConfig  string // Janus config path via --executor-janus-config
	Quiet        bool   // Suppress streaming via --executor-quiet
	Help         bool   // Show executor help via --executor-help
	Version      bool   // Show executor version via --executor-version
	NoRunlog     bool   // Skip runlog update via --executor-no-runlog
	RunIQ        bool   // Run Installation Qualification via --executor-run-iq
	RunOQ        bool   // Run Operational Qualification via --executor-run-oq

	// RetiredLicense records that --executor-license was passed. Janus no longer
	// uses a license; the flag is accepted and ignored so that existing wrapper
	// scripts keep working, and main warns once rather than failing.
	RetiredLicense bool
}

// parseExecutorFlags separates executor-specific flags from container arguments.
// All arguments starting with --executor- are parsed and removed from the container args.
// Everything else is passed through unchanged.
//
// Returns: (executorFlags, containerArgs).
func parseExecutorFlags(args []string) (ExecutorFlags, []string) {
	flags := ExecutorFlags{}
	containerArgs := []string{}

	for i := 0; i < len(args); i++ {
		arg := args[i] //nolint:gosec // G602: loop bounds guarantee valid index

		switch {
		// Boolean flags (no arguments)
		case arg == "--executor-help":
			flags.Help = true
		case arg == "--executor-version":
			flags.Version = true
		case arg == "--executor-quiet":
			flags.Quiet = true
		case arg == "--executor-no-runlog":
			flags.NoRunlog = true
		case arg == "--executor-run-iq":
			flags.RunIQ = true
		case arg == "--executor-run-oq":
			flags.RunOQ = true

		// Retired: Janus no longer requires a license. Still consumed here, with
		// its value, so it cannot reach the container. Letting it fall through to
		// the default branch would forward "--executor-license" to nmfe as a
		// control-stream filename and displace the real model path, turning a
		// removed flag into an opaque NONMEM failure for anyone whose wrapper
		// script still passes it.
		case strings.HasPrefix(arg, "--executor-license="):
			flags.RetiredLicense = true
		case arg == "--executor-license":
			flags.RetiredLicense = true

			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++ // consume its value too
			}

		// --executor-hermes-config with = syntax
		case strings.HasPrefix(arg, "--executor-hermes-config="):
			flags.HermesConfig = strings.TrimPrefix(arg, "--executor-hermes-config=")

		// --executor-hermes-config with separate argument
		case arg == "--executor-hermes-config":
			if i+1 < len(args) {
				flags.HermesConfig = args[i+1]
				i++ // Skip next arg (it's the value)
			}

		// --executor-janus-config with = syntax
		case strings.HasPrefix(arg, "--executor-janus-config="):
			flags.JanusConfig = strings.TrimPrefix(arg, "--executor-janus-config=")

		// --executor-janus-config with separate argument
		case arg == "--executor-janus-config":
			if i+1 < len(args) {
				flags.JanusConfig = args[i+1]
				i++ // Skip next arg (it's the value)
			}

		default:
			// Not an executor flag - pass to container
			containerArgs = append(containerArgs, arg)
		}
	}

	return flags, containerArgs
}
