package cmd

import (
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/pharmalytica/janus/cmd/gui" // Still needed for gui.RunGUI()
	"github.com/pharmalytica/janus/cmd/janus/commands/execute"
	"github.com/pharmalytica/janus/cmd/janus/commands/hermes"
	"github.com/pharmalytica/janus/cmd/janus/commands/mcp"
	"github.com/pharmalytica/janus/cmd/janus/commands/validate"
	"github.com/pharmalytica/janus/cmd/version"
	"github.com/pharmalytica/janus/internal/config"
)

func Command(assets embed.FS) *cobra.Command {
	var configuration *config.Config
	c := &cobra.Command{
		Use:   "janus [model-file]",
		Short: "NONMEM Grid Management Tool",
		Long:  `Janus is a modern, cost-effective replacement for Certara Pirana using Go + Fyne.io, with pluggable orchestrator backends and built-in CFR 21 Part 11 compliance.`,
		Args:  cobra.MaximumNArgs(1), // Accept 0 or 1 positional arguments
		PersistentPreRunE: func(c *cobra.Command, args []string) error {
			// Setup log file redirection if --logfile flag is set
			if err := setupLogging(c); err != nil {
				// If logging setup fails, try to write error to a fallback location
				fallbackLog := filepath.Join(os.TempDir(), "janus-error.log")
				if f, fErr := os.OpenFile(fallbackLog, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666); fErr == nil {
					fmt.Fprintf(f, "=== %v ===\nFailed to setup logging: %v\n", os.Args, err)
					f.Close()
				}

				return fmt.Errorf("failed to setup logging: %w", err)
			}

			// Run config initialization
			return config.NewInitializer(&configuration, config.InitializerOptions{
				ConfigFlagName: "config",
				SuppressOutput: true, // Suppress stderr output for GUI mode (prevents crash with -H windowsgui)
			})(c, args)
		},
		RunE: func(c *cobra.Command, args []string) error {
			// If no subcommand is specified, run the GUI by default
			// This allows `janus` to work the same as `janus gui`

			// Check for model from positional argument or flag
			modelPath, _ := c.Flags().GetString("model")
			var modelArgs []string

			if len(args) > 0 {
				// Positional argument takes precedence
				if modelPath != "" {
					return fmt.Errorf("model file specified both as positional argument and --model flag; use one or the other")
				}
				// Expand ~ in positional argument
				expandedPath, err := expandHomePath(args[0])
				if err != nil {
					return fmt.Errorf("failed to expand model path: %w", err)
				}
				modelArgs = []string{expandedPath}
			} else if modelPath != "" {
				// Expand ~ in --model flag
				expandedPath, err := expandHomePath(modelPath)
				if err != nil {
					return fmt.Errorf("failed to expand model path: %w", err)
				}
				modelArgs = []string{expandedPath}
			}

			// Get license path from flag
			licensePath, _ := c.Flags().GetString("license")

			return gui.RunGUI(c.Context(), configuration, modelArgs, licensePath, assets)
		},
	}

	attributes(c)

	// Note: gui subcommand removed - root command now launches GUI by default
	// This simplifies the CLI and makes file associations work correctly
	c.AddCommand(version.Command())
	c.AddCommand(hermes.Command())
	c.AddCommand(execute.Command())
	c.AddCommand(validate.Command())
	c.AddCommand(mcp.Command(assets))

	return c
}

func attributes(c *cobra.Command) {
	// Global flags
	c.PersistentFlags().String("config", "", "config file (default is ~/.config/janus/config.yml)")
	c.PersistentFlags().String("logfile", "", "path to log file (if not set, logs to stderr)")
	c.Flags().String("model", "", "model file to load automatically in GUI")
	c.Flags().String("license", getDefaultLicensePath(), "path to license JWT file")

	// Additional configuration flags with defaults
	c.PersistentFlags().String("organization", "BigPharma LLC", "organization name")
	c.PersistentFlags().String("default-directory", "~/models", "default directory for models")
	c.PersistentFlags().String("nonmem-path", "/opt/NONMEM/nm76/run", "path to NONMEM installation")
	c.PersistentFlags().String("scheduler", "SLURM", "job scheduler (SLURM, SGE, TORQUE)")
	c.PersistentFlags().Bool("projects", true, "enable project management")

	// Validation configuration flags
	c.PersistentFlags().String("validation.iq", "~/.config/janus/validation/iq-report.json", "IQ validation report output path")
	c.PersistentFlags().String("validation.oq", "~/.config/janus/validation/oq-report.json", "OQ validation report output path")

	// Bind all flags to viper
	_ = viper.BindPFlags(c.PersistentFlags())
	_ = viper.BindPFlags(c.Flags())
}

func getDefaultLicensePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// If home dir lookup fails, use executable directory instead of CWD
		// This ensures we look in the right place even when launched from shortcuts
		exePath, exeErr := os.Executable()
		if exeErr != nil {
			return "./license.jwt" // Last resort fallback
		}

		return filepath.Join(filepath.Dir(exePath), "license.jwt")
	}

	return filepath.Join(home, ".config", "janus", "license.jwt")
}

// expandHomePath expands ~ to the user's home directory in file paths.
// Returns the expanded path or an error if home directory cannot be determined.
func expandHomePath(path string) (string, error) {
	if path == "" || path[0] != '~' {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory for path expansion: %w", err)
	}

	// Handle both ~/path and ~ (just home)
	if len(path) == 1 {
		return home, nil
	}

	return filepath.Join(home, path[1:]), nil
}

// setupLogging configures log output based on the --logfile flag.
func setupLogging(cmd *cobra.Command) error {
	logfile, _ := cmd.Flags().GetString("logfile")
	if logfile == "" {
		// No logfile specified, use default stderr (current behavior)
		return nil
	}

	// Expand ~ to home directory
	expandedPath, err := expandHomePath(logfile)
	if err != nil {
		return err
	}
	logfile = expandedPath

	// Ensure parent directory exists
	logDir := filepath.Dir(logfile)
	if logDir != "" && logDir != "." && logDir != "/" {
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return fmt.Errorf("failed to create log directory %s: %w", logDir, err)
		}
	}

	// Open log file for writing (create if doesn't exist, append if it does)
	f, err := os.OpenFile(logfile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed to open log file %s: %w", logfile, err)
	}

	// Redirect standard logger output to the file
	log.SetOutput(f)
	log.Printf("=== Janus started, logging to %s ===", logfile)

	return nil
}
