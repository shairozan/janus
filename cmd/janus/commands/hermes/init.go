package hermes

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"

	"github.com/shairozan/janus/internal/config"
)

// initCommand creates the 'janus hermes init' command.
func initCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "init [model-path]",
		Short: "Initialize Hermes configuration for a model",
		Long: `Initialize Hermes configuration for a NONMEM model.

This creates a .janus.config.json file colocated with the model that specifies:
- Container image to use (e.g., ghcr.io/pharmalytica/nonmem:7.5.0)
- CPU cores to allocate
- Memory to allocate

The configuration file is required for Hermes execution.

Example:
  janus hermes init /path/to/model.mod
  janus hermes init model.mod --force  # Overwrite existing config`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(args[0], force)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "overwrite existing configuration file")

	return cmd
}

func runInit(modelPath string, force bool) error {
	// Validate model file exists
	absModelPath, err := filepath.Abs(modelPath)
	if err != nil {
		return fmt.Errorf("failed to resolve model path: %w", err)
	}

	if _, err := os.Stat(absModelPath); err != nil {
		return fmt.Errorf("model file not found: %s", absModelPath)
	}

	// Check if config already exists
	configPath := config.ConfigPath(absModelPath)
	if _, err := os.Stat(configPath); err == nil && !force {
		return fmt.Errorf("configuration file already exists: %s\nUse --force to overwrite", configPath)
	}

	log.Println("Initializing Hermes configuration")
	log.Printf("Model: %s", filepath.Base(absModelPath))

	// Interactive prompts
	hermesConfig, err := promptForConfig()
	if err != nil {
		return fmt.Errorf("failed to get configuration: %w", err)
	}

	// Validate configuration
	if err := hermesConfig.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Save configuration (pass model path, not config path)
	if err := saveConfig(absModelPath, hermesConfig); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	log.Println("Hermes configuration saved successfully")
	log.Printf("Config file: %s", configPath)
	log.Println("Next steps:")
	log.Printf("  janus execute hermes %s", absModelPath)

	return nil
}

func promptForConfig() (*config.HermesExecutionConfig, error) {
	var image string
	var cpuCoresStr string
	var memory string

	// Prompt 1: Container image
	imagePrompt := &survey.Input{
		Message: "Container image:",
		Default: "ghcr.io/pharmalytica/nonmem:7.5.0",
		Help:    "Full container image reference including registry and tag",
	}
	if err := survey.AskOne(imagePrompt, &image, survey.WithValidator(survey.Required)); err != nil {
		return nil, err
	}

	// Prompt 2: CPU cores
	cpuPrompt := &survey.Input{
		Message: "CPU cores:",
		Default: "4",
		Help:    "Number of CPU cores to allocate to the container (must be positive integer)",
	}
	if err := survey.AskOne(cpuPrompt, &cpuCoresStr, survey.WithValidator(validatePositiveInt)); err != nil {
		return nil, err
	}

	cpuCores, _ := strconv.Atoi(cpuCoresStr) // Already validated

	// Prompt 3: Memory
	memPrompt := &survey.Input{
		Message: "Memory:",
		Default: "8Gi",
		Help:    "Memory to allocate (e.g., 8Gi, 16Gi, 2048Mi, 4G, 512M)",
	}
	if err := survey.AskOne(memPrompt, &memory, survey.WithValidator(validateMemoryFormat)); err != nil {
		return nil, err
	}

	return &config.HermesExecutionConfig{
		Image: image,
		Resources: config.ResourceConfig{
			CPUCores: cpuCores,
			Memory:   memory,
		},
	}, nil
}

func validatePositiveInt(val interface{}) error {
	str, ok := val.(string)
	if !ok {
		return fmt.Errorf("invalid input type")
	}

	num, err := strconv.Atoi(str)
	if err != nil {
		return fmt.Errorf("must be a valid integer")
	}

	if num <= 0 {
		return fmt.Errorf("must be a positive integer")
	}

	return nil
}

func validateMemoryFormat(val interface{}) error {
	str, ok := val.(string)
	if !ok {
		return fmt.Errorf("invalid input type")
	}

	// Memory format: number followed by unit (Ki, Mi, Gi, Ti, K, M, G, T)
	// Examples: 8Gi, 2048Mi, 4G, 512M
	memoryPattern := regexp.MustCompile(`^(\d+(\.\d+)?)(Ki|Mi|Gi|Ti|K|M|G|T)$`)
	if !memoryPattern.MatchString(str) {
		return fmt.Errorf("invalid memory format. Expected format: 8Gi, 2048Mi, 4G, 512M")
	}

	return nil
}

func saveConfig(modelPath string, hermesConfig *config.HermesExecutionConfig) error {
	// Try to load existing config to preserve all settings (e.g., correlation_strategy)
	existingCfg, _ := config.LoadModelConfig(modelPath)

	var modelCfg *config.ModelConfig
	if existingCfg != nil {
		// Modify the existing config to preserve all current and future fields
		existingCfg.Hermes = hermesConfig
		modelCfg = existingCfg
	} else {
		// No existing config - create a new one
		modelCfg = &config.ModelConfig{
			Hermes: hermesConfig,
		}
	}

	// Seed the best-practice NONMEM retain set (a model-wide property) so it is
	// visible and editable in the file. Only seed when absent — never clobber a
	// retain the user already customized. Edit the file (or the GUI ⚙️ dialog) to
	// change it (#94).
	if len(modelCfg.Retain) == 0 {
		modelCfg.Retain = config.DefaultNONMEMRetain()
	}

	return config.SaveModelConfig(modelPath, modelCfg)
}
