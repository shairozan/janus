package config

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/pharmalytica/janus/internal/version"
)

// Execution mode constants.
const (
	ExecutionModeNONMEM = "NONMEM"
	ExecutionModeBBI    = "BBI"
	ExecutionModePSN    = "PSN"
)

// Input represents configuration that comes from viper (files, flags, env vars).
type Input struct {
	// User Configuration
	Organization     string `mapstructure:"organization" yaml:"organization"`
	DefaultDirectory string `mapstructure:"default-directory" yaml:"default-directory"`
	NonmemPath       string `mapstructure:"nonmem-path" yaml:"nonmem-path"`
	NonmemBinary     string `mapstructure:"nonmem-binary" yaml:"nonmem-binary"`

	// Scheduler Configuration
	Scheduler string `mapstructure:"scheduler" yaml:"scheduler"`

	// Execution Configuration
	ExecutionMode string `mapstructure:"execution-mode" yaml:"execution-mode"`

	// Feature Flags
	ProjectsEnable bool `mapstructure:"projects" yaml:"projects"`

	// Validation Configuration for CFR 21 Part 11 compliance
	Validation ValidationControl `mapstructure:"validation" yaml:"validation"`

	// Future settings (commented in YAML but struct ready)
	SLURM        SLURMConfig    `mapstructure:"slurm" yaml:"slurm"`
	Audit        AuditConfig    `mapstructure:"audit" yaml:"audit"`
	ProjectsConf ProjectsConfig `mapstructure:"projects_config" yaml:"projects_config"`
}

// Config represents the complete runtime configuration for the application.
// This includes both viper-sourced configuration and external sources.
type Config struct {
	// External sources (not from viper)
	Version string // From build-time ldflags
	User    string // From OS user

	// Viper-sourced configuration (embedded)
	Input
}

// ValidationControl represents validation configuration for CFR 21 Part 11 compliance.
type ValidationControl struct {
	IQ string `mapstructure:"iq" yaml:"iq"` // Installation Qualification output path
	OQ string `mapstructure:"oq" yaml:"oq"` // Operational Qualification output path
}

// SLURM mode constants.
const (
	SLURMModeREST = "REST"
	SLURMModeCLI  = "CLI"
)

// SLURMConfig represents SLURM-specific configuration.
type SLURMConfig struct {
	// Mode determines how to interact with SLURM: "REST" or "CLI"
	Mode string `mapstructure:"mode" yaml:"mode"`

	// CLI mode configuration (legacy)
	Host    string `mapstructure:"host" yaml:"host"`
	Port    int    `mapstructure:"port" yaml:"port"`
	Timeout string `mapstructure:"timeout" yaml:"timeout"`

	// REST mode configuration
	REST SLURMRESTConfig `mapstructure:"rest" yaml:"rest"`
}

// SLURMRESTConfig represents SLURM REST API configuration.
type SLURMRESTConfig struct {
	// SocketPath is the path to the Unix domain socket for slurmrestd
	SocketPath string `mapstructure:"socket_path" yaml:"socket_path"`

	// APIVersion specifies which OpenAPI specification version to use
	// Common values: "v0.0.40", "v0.0.39", "v0.0.38"
	APIVersion string `mapstructure:"api_version" yaml:"api_version"`

	// Timeout for REST API requests
	Timeout string `mapstructure:"timeout" yaml:"timeout"`

	// AuthToken for SLURM REST API authentication (optional)
	AuthToken string `mapstructure:"auth_token" yaml:"auth_token"`
}

// AuditConfig represents audit engine configuration.
type AuditConfig struct {
	Backend string `mapstructure:"backend" yaml:"backend"`
	Path    string `mapstructure:"path" yaml:"path"`
}

// ProjectsConfig represents project management configuration.
type ProjectsConfig struct {
	DefaultTemplate string `mapstructure:"default-template" yaml:"default-template"`
	AutoBackup      bool   `mapstructure:"auto-backup" yaml:"auto-backup"`
}

// UnmarshalInputFromViper creates Input from Viper configuration.
func UnmarshalInputFromViper() (*Input, error) {
	input := &Input{}

	// Unmarshal from viper (defaults are handled by cobra flags)
	if err := viper.Unmarshal(input); err != nil {
		return nil, fmt.Errorf("failed to unmarshal input config: %w", err)
	}

	return input, nil
}

// NewConfig creates a complete Config by merging Input with external sources.
func NewConfig(input *Input) (*Config, error) {
	// Validate execution mode and required tools
	if err := ValidateExecutionMode(input.ExecutionMode); err != nil {
		return nil, fmt.Errorf("execution mode validation failed: %w", err)
	}

	cfg := &Config{
		Version: version.Get(),
		Input:   *input,
	}

	// Populate runtime fields
	if err := populateRuntimeFields(cfg); err != nil {
		return nil, fmt.Errorf("failed to populate runtime fields: %w", err)
	}

	return cfg, nil
}

// populateRuntimeFields fills in fields that are determined at runtime.
func populateRuntimeFields(cfg *Config) error {
	// Get current OS user
	currentUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	// Use username (not display name) for consistency
	cfg.User = currentUser.Username

	return nil
}

// Process creates a complete Config by processing viper configuration and external sources.
// This function combines UnmarshalInputFromViper and NewConfig to provide a complete
// runtime configuration that includes both viper-sourced and external data.
func Process() (*Config, error) {
	input, err := UnmarshalInputFromViper()
	if err != nil {
		return nil, err
	}

	return NewConfig(input)
}

// ValidateExecutionMode validates that the execution mode is a valid option.
// It does NOT check for tool existence - that validation happens at execution time.
func ValidateExecutionMode(mode string) error {
	switch mode {
	case ExecutionModeNONMEM, ExecutionModeBBI, ExecutionModePSN:
		// All valid execution modes - tool existence is validated at execution time
		return nil
	default:
		return fmt.Errorf("invalid execution mode: %q (must be one of: %s, %s, %s)",
			mode, ExecutionModeNONMEM, ExecutionModeBBI, ExecutionModePSN)
	}
}

// GetValidExecutionModes returns a list of valid execution modes.
func GetValidExecutionModes() []string {
	return []string{ExecutionModeNONMEM, ExecutionModeBBI, ExecutionModePSN}
}

// ValidateSLURMMode validates the SLURM mode configuration.
func ValidateSLURMMode(config SLURMConfig) error {
	if config.Mode == "" {
		// Default to CLI mode if not specified
		return nil
	}

	switch config.Mode {
	case SLURMModeREST:
		// REST mode validation
		if config.REST.SocketPath == "" {
			return fmt.Errorf("SLURM REST mode requires socket_path to be configured")
		}
		if config.REST.APIVersion == "" {
			return fmt.Errorf("SLURM REST mode requires api_version to be configured")
		}

		return nil

	case SLURMModeCLI:
		// CLI mode validation - no specific requirements for now
		return nil

	default:
		return fmt.Errorf("invalid SLURM mode: %q (must be one of: %s, %s)",
			config.Mode, SLURMModeREST, SLURMModeCLI)
	}
}

// GetValidSLURMModes returns a list of valid SLURM modes.
func GetValidSLURMModes() []string {
	return []string{SLURMModeREST, SLURMModeCLI}
}

// InitializerOptions holds configuration for the configuration initializer.
type InitializerOptions struct {
	ConfigFlagName    string // The name of the config flag to read
	DefaultConfigPath string // Default config file path
	SuppressOutput    bool   // Whether to suppress stdout/stderr output
}

// NewInitializer creates a PreRunE function that initializes viper configuration
// and unmarshals it onto the provided config pointer.
// This allows any cobra command to easily set up configuration.
func NewInitializer(configPtr **Config, opts InitializerOptions) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		// Get config file from flag
		var configFile string
		if opts.ConfigFlagName != "" {
			flagValue, err := cmd.Flags().GetString(opts.ConfigFlagName)
			if err == nil {
				configFile = flagValue
			}
		}

		// Determine config file path
		configPath := opts.DefaultConfigPath
		if configPath == "" {
			configPath = getDefaultConfigPath()
		}

		if configFile != "" {
			// Use config file from the flag
			viper.SetConfigFile(configFile)
		} else {
			// Use default config path
			viper.SetConfigFile(configPath)
		}

		// Try to read config file
		if err := viper.ReadInConfig(); err != nil {
			// Check if config file doesn't exist (handle both viper and filesystem errors)
			var configFileNotFoundErr viper.ConfigFileNotFoundError
			if errors.As(err, &configFileNotFoundErr) || os.IsNotExist(err) {
				// Config file not found - this will be handled by the GUI command
				// which will show the setup wizard. For non-GUI commands, we'll
				// need basic defaults to work with.
				if !opts.SuppressOutput {
					fmt.Fprintln(os.Stderr, "No config file found. Run 'janus gui' to set up initial configuration.")
				}

				return nil
			} else {
				// Config file was found but another error occurred
				if !opts.SuppressOutput {
					fmt.Fprintf(os.Stderr, "Error reading config file: %v\n", err)
				}

				return err
			}
		} else if !opts.SuppressOutput {
			fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
		}

		// Process configuration and assign to the pointer
		cfg, err := Process()
		if err != nil {
			return fmt.Errorf("failed to process configuration: %w", err)
		}

		*configPtr = cfg

		return nil
	}
}

// getDefaultConfigPath returns the default configuration file path.
func getDefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home directory can't be determined
		return "./config.yml"
	}

	return filepath.Join(home, ".config", "janus", "config.yml")
}
