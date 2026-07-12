package gui

import (
	"context"
	"embed"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/pharmalytica/janus/internal/config"
	"github.com/pharmalytica/janus/internal/gui"
	"github.com/pharmalytica/janus/internal/license/validator"
)

func Command(assets embed.FS) *cobra.Command {
	var cfg *config.Config

	guiCmd := &cobra.Command{
		Use:   "gui [model_file]",
		Short: "Launch Janus GUI",
		Long:  "Launch the Janus graphical user interface for NONMEM job management. Optionally provide a model file path to load automatically.",
		Args:  cobra.MaximumNArgs(1), // Accept 0 or 1 arguments
		PreRunE: func(c *cobra.Command, args []string) error {
			// For GUI command, we handle config differently to support setup wizard
			configPath := getConfigPath()
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				// No config file - we'll handle this in RunE with the setup wizard
				return nil
			}

			// Config exists, initialize viper and process it
			viper.SetEnvPrefix("JANUS")
			viper.AutomaticEnv()
			viper.SetConfigFile(configPath)

			// Bind flags to viper (needed for override functionality)
			_ = viper.BindPFlags(c.Flags())
			_ = viper.BindPFlags(c.PersistentFlags())

			if err := viper.ReadInConfig(); err != nil {
				return err
			}

			var err error
			cfg, err = config.Process()

			return err
		},
		RunE: func(c *cobra.Command, args []string) error {
			// Check for model flag first, then arguments
			modelPath, _ := c.Flags().GetString("model")
			var modelArgs []string

			if modelPath != "" {
				modelArgs = []string{modelPath}
			} else if len(args) > 0 {
				modelArgs = args
			}

			// Get license path from flag
			licensePath, _ := c.Flags().GetString("license")

			return RunGUI(c.Context(), cfg, modelArgs, licensePath, assets)
		},
	}

	// Add flags specific to GUI command
	guiCmd.Flags().String("model", "", "model file to load automatically")
	guiCmd.Flags().String("license", getDefaultLicensePath(), "path to license JWT file")

	return guiCmd
}

// RunGUI starts the GUI application with optional model file loading.
// This function can be called from both the gui subcommand and the root command.
func RunGUI(ctx context.Context, cfg *config.Config, args []string, licensePath string, assets embed.FS) error {
	// Initialize GUI application with context first (needed for dialogs)
	app := gui.NewApp(ctx)

	// Open and validate license file
	licenseFile, err := os.Open(licensePath)
	if err != nil {
		app.ShowLicenseError(fmt.Errorf("failed to open license file at %s: %w", licensePath, err))
		return fmt.Errorf("license file error: %w", err)
	}
	defer licenseFile.Close()

	// Validate license before proceeding
	licenseClaims, err := validateLicense(licenseFile, assets)
	if err != nil {
		// Show error dialog and exit
		app.ShowLicenseError(err)
		return fmt.Errorf("license validation error: %w", err)
	}

	// Log license information
	log.Printf("License validated: Org=%d, Tier=%s, Features=%v",
		licenseClaims.OrganizationID, licenseClaims.Tier, licenseClaims.Features)

	// Set license claims for feature gating
	app.SetLicenseClaims(licenseClaims)

	// Set up cleanup on context cancellation or app shutdown
	defer app.Cleanup()

	// Check if a model file was provided
	var modelFilePath string
	if len(args) > 0 {
		modelFilePath = args[0]
		// Convert to absolute path if relative
		if absPath, err := filepath.Abs(modelFilePath); err == nil {
			modelFilePath = absPath
		}
	}

	// Check if we need to run setup wizard
	if cfg == nil {
		// No config exists, show setup wizard first
		wizard := gui.NewSetupWizard(app.GetFyneApp(), func(input *config.Input) { //nolint:contextcheck
			// After setup is complete, create full config and show main app
			fullConfig, err := config.NewConfig(input)
			if err != nil {
				// TODO: Show error dialog to user
				return
			}

			// Set config and show main interface
			app.SetConfiguration(fullConfig)

			// Load model file if provided
			if modelFilePath != "" {
				if err := app.LoadModelFile(modelFilePath); err != nil {
					// TODO: Show error dialog to user about model loading failure
					log.Printf("Error loading model file: %v\n", err)
				}
			}

			app.ShowMainWindow()
		})

		// Show the wizard
		wizard.Show()

		// Run the app without showing main window - only wizard is visible
		// When setup completes, the main window will be shown
		app.RunWithoutShowing()

		return nil
	} else {
		// Config exists, start normally
		app.SetConfiguration(cfg)

		// Load model file if provided
		if modelFilePath != "" {
			if err := app.LoadModelFile(modelFilePath); err != nil {
				// TODO: Show error dialog to user about model loading failure
				log.Printf("Error loading model file: %v\n", err)
			}
		}

		app.Run() //nolint:contextcheck // This is just building the UI components

		return nil
	}
}

func getConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "./config.yml"
	}

	return filepath.Join(home, ".config", "janus", "config.yml")
}

func getDefaultLicensePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "./license.jwt"
	}

	return filepath.Join(home, ".config", "janus", "license.jwt")
}

// validateLicense reads and validates a license JWT from the provided reader.
func validateLicense(r io.Reader, assets embed.FS) (*validator.Claims, error) {
	// Read the JWT token from the reader
	tokenBytes, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read license: %w", err)
	}

	tokenString := string(tokenBytes)

	// Create validator with embedded assets
	v, err := validator.NewValidator(assets)
	if err != nil {
		return nil, fmt.Errorf("failed to create validator: %w", err)
	}

	// Validate the token
	claims, err := v.ValidateToken(tokenString)
	if err != nil {
		return nil, fmt.Errorf("invalid license token: %w", err)
	}

	return claims, nil
}
