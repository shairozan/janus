package cmd

import (
	"embed"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/pharmalytica/janus/cmd/gui"
	"github.com/pharmalytica/janus/cmd/version"
	"github.com/pharmalytica/janus/internal/config"
)

func Command(assets embed.FS) *cobra.Command {
	var configuration *config.Config
	c := &cobra.Command{
		Use:   "janus",
		Short: "NONMEM Grid Management Tool",
		Long:  `Janus is a modern, cost-effective replacement for Certara Pirana using Go + Fyne.io, with pluggable orchestrator backends and built-in CFR 21 Part 11 compliance.`,
		PersistentPreRunE: config.NewInitializer(&configuration, config.InitializerOptions{
			ConfigFlagName: "config",
		}),
		RunE: func(c *cobra.Command, args []string) error {
			// If no subcommand is specified, run the GUI by default
			// This allows `janus` to work the same as `janus gui`

			// Check if model flag was provided
			modelPath, _ := c.Flags().GetString("model")
			var modelArgs []string
			if modelPath != "" {
				modelArgs = []string{modelPath}
			}

			// Get license path from flag
			licensePath, _ := c.Flags().GetString("license")

			return gui.RunGUI(c.Context(), configuration, modelArgs, licensePath, assets)
		},
	}

	attributes(c)

	c.AddCommand(gui.Command(assets))
	c.AddCommand(version.Command())

	return c
}

func attributes(c *cobra.Command) {
	// Global flags
	c.PersistentFlags().String("config", "", "config file (default is ~/.config/janus/config.yml)")
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
		return "./license.jwt"
	}

	return filepath.Join(home, ".config", "janus", "license.jwt")
}
