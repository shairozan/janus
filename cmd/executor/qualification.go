package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pharmalytica/janus/internal/qa"
	"github.com/pharmalytica/janus/internal/qarun"
	"github.com/pharmalytica/janus/internal/summary"
)

// runQualification executes the headless IQ or OQ path and returns a process
// exit code: 0 when the qualification passes, non-zero otherwise.
func runQualification(flags ExecutorFlags) int {
	cfg, cfgErr := loadJanusConfig(flags.JanusConfig)

	store := qa.NewStore(qarun.DefaultQABaseDir())

	_, runDir, err := store.NewRunDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		return 1
	}

	opts := qa.Options{ExecutorProbe: qarun.NewExecutorProbe()}

	if flags.RunIQ {
		res := qa.RunIQ(cfg, runDir, opts)
		if saveErr := store.Save(runDir, qa.KindIQ, res); saveErr != nil {
			fmt.Fprintf(os.Stderr, "Error saving IQ report: %v\n", saveErr)

			return 1
		}

		printResult("IQ", string(res.Status), res.Checks, runDir, flags.Quiet)

		return statusExitCode(res.Status)
	}

	// OQ requires a valid configuration to build the executor.
	if cfgErr != nil || cfg == nil {
		fmt.Fprintf(os.Stderr, "OQ requires a valid configuration: %v\n", cfgErr)

		return 1
	}

	provider := qarun.BuildExecutorProvider(cfg)

	res, runErr := qa.RunOQ(context.Background(), cfg, provider, summary.NewNONMEMSummarizer(), runDir, opts)
	if res == nil {
		fmt.Fprintf(os.Stderr, "OQ error: %v\n", runErr)

		return 1
	}

	if saveErr := store.Save(runDir, qa.KindOQ, res); saveErr != nil {
		fmt.Fprintf(os.Stderr, "Error saving OQ report: %v\n", saveErr)

		return 1
	}

	if runErr != nil {
		fmt.Fprintf(os.Stderr, "OQ run error: %v\n", runErr)
	}

	printResult("OQ", string(res.Status), res.Checks, runDir, flags.Quiet)

	return statusExitCode(res.Status)
}

// statusExitCode maps a qualification status to a process exit code.
func statusExitCode(status qa.Status) int {
	if status == qa.StatusPass {
		return 0
	}

	return 1
}

// printResult writes a concise human-readable summary. In quiet mode only the
// header and any non-pass checks are printed.
//
//nolint:forbidigo // CLI qualification output - intentional user-facing text
func printResult(kind, status string, checks []qa.Check, runDir string, quiet bool) {
	fmt.Printf("%s: %s (report: %s)\n", kind, status, filepath.Join(runDir, kindFile(kind)))

	for _, c := range checks {
		if quiet && c.Status == qa.StatusPass {
			continue
		}

		if c.Reason != "" {
			fmt.Printf("  [%s] %s — %s\n", c.Status, c.Name, c.Reason)
		} else {
			fmt.Printf("  [%s] %s\n", c.Status, c.Name)
		}
	}
}

// kindFile maps the printed kind label to its report filename.
func kindFile(kind string) string {
	if kind == "OQ" {
		return "oq.json"
	}

	return "iq.json"
}
