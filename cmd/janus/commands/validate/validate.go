// Package validate provides the `janus validate iq|oq` subcommands, a
// discoverable CLI surface over the same internal/qa core used by the GUI and
// the executor. Each leaf command is thin: load config, build deps, call the qa
// core, print, and set the exit code via the returned error.
package validate

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/shairozan/janus/internal/config"
	"github.com/shairozan/janus/internal/qa"
	"github.com/shairozan/janus/internal/qarun"
	"github.com/shairozan/janus/internal/summary"
)

// Command returns the `validate` parent command.
func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Run installation/operational qualification (IQ/OQ)",
		Long: `Run runtime qualification of this Janus install.

  validate iq   Installation Qualification — is Janus itself installed and operable?
  validate oq   Operational Qualification  — is the configuration valid and functional?

Each run writes a JSON report under ~/.config/janus/qa/<uuid>/ and exits non-zero
if the qualification fails.`,
	}

	cmd.AddCommand(iqCommand(), oqCommand())

	return cmd
}

func iqCommand() *cobra.Command {
	var cfg *config.Config

	cmd := &cobra.Command{
		Use:               "iq",
		Short:             "Run Installation Qualification (Janus tool self-check)",
		Args:              cobra.NoArgs,
		SilenceUsage:      true,
		PersistentPreRunE: config.NewInitializer(&cfg, config.InitializerOptions{ConfigFlagName: "config"}),
		RunE: func(c *cobra.Command, _ []string) error {
			return runIQ(c.OutOrStdout(), cfg)
		},
	}

	return cmd
}

func oqCommand() *cobra.Command {
	var cfg *config.Config

	cmd := &cobra.Command{
		Use:               "oq",
		Short:             "Run Operational Qualification (config validity + functional ACOP run)",
		Args:              cobra.NoArgs,
		SilenceUsage:      true,
		PersistentPreRunE: config.NewInitializer(&cfg, config.InitializerOptions{ConfigFlagName: "config"}),
		RunE: func(c *cobra.Command, _ []string) error {
			return runOQ(c.Context(), c.OutOrStdout(), cfg)
		},
	}

	return cmd
}

func runIQ(out io.Writer, cfg *config.Config) error {
	store := qa.NewStore(qarun.DefaultQABaseDir())

	_, runDir, err := store.NewRunDir()
	if err != nil {
		return fmt.Errorf("creating qa run dir: %w", err)
	}

	res := qa.RunIQ(cfg, runDir, qa.Options{ExecutorProbe: qarun.NewExecutorProbe()})
	if err := store.Save(runDir, qa.KindIQ, res); err != nil {
		return fmt.Errorf("saving IQ report: %w", err)
	}

	printReport(out, "IQ", res.Status, res.Checks, runDir)

	return statusError("IQ", res.Status)
}

func runOQ(ctx context.Context, out io.Writer, cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("OQ requires a valid configuration")
	}

	store := qa.NewStore(qarun.DefaultQABaseDir())

	_, runDir, err := store.NewRunDir()
	if err != nil {
		return fmt.Errorf("creating qa run dir: %w", err)
	}

	provider := qarun.BuildExecutorProvider(cfg)

	res, runErr := qa.RunOQ(ctx, cfg, provider, summary.NewNONMEMSummarizer(), runDir, qa.Options{ExecutorProbe: qarun.NewExecutorProbe()})
	if res == nil {
		return fmt.Errorf("running OQ: %w", runErr)
	}

	if err := store.Save(runDir, qa.KindOQ, res); err != nil {
		return fmt.Errorf("saving OQ report: %w", err)
	}

	printReport(out, "OQ", res.Status, res.Checks, runDir)

	if runErr != nil {
		return fmt.Errorf("running OQ: %w", runErr)
	}

	return statusError("OQ", res.Status)
}

// printReport writes a concise human-readable summary to out. Using Fprintf to a
// writer (rather than fmt.Print to stdout) keeps it idiomatic and lint-clean.
func printReport(out io.Writer, kind string, status qa.Status, checks []qa.Check, runDir string) {
	fmt.Fprintf(out, "%s: %s\n", kind, status)

	for _, c := range checks {
		if c.Reason != "" {
			fmt.Fprintf(out, "  [%s] %s — %s\n", c.Status, c.Name, c.Reason)
		} else {
			fmt.Fprintf(out, "  [%s] %s\n", c.Status, c.Name)
		}
	}

	fmt.Fprintf(out, "report: %s\n", runDir)
}

// statusError returns a non-nil error when the qualification did not pass, so
// the command exits non-zero.
func statusError(kind string, status qa.Status) error {
	if status == qa.StatusPass {
		return nil
	}

	return fmt.Errorf("%s qualification %s", strings.ToUpper(kind), status)
}
