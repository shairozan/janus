package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/pharmalytica/janus/internal/config"
)

// findHermesConfig locates .janus.config.json using a multi-strategy approach.
//
// Discovery algorithm:
//  1. If explicitConfigPath is provided, use it (fail if not found)
//  2. Scan all args for file/directory paths, check each for .janus.config.json
//  3. Fall back to current working directory
//  4. Error if not found
//
// Returns: (configPath, modelPath, error)
//   - configPath: Absolute path to .janus.config.json
//   - modelPath: Path to model file (if discovered), empty string otherwise
//   - error: Error if config not found
func findHermesConfig(args []string, explicitConfigPath string) (string, string, error) {
	// Strategy 0: Explicit config path takes absolute precedence
	if explicitConfigPath != "" {
		absPath, err := filepath.Abs(explicitConfigPath)
		if err != nil {
			return "", "", fmt.Errorf("failed to resolve config path: %w", err)
		}

		if _, err := os.Stat(absPath); err != nil {
			return "", "", fmt.Errorf("explicit config not found: %s", absPath)
		}

		return absPath, "", nil
	}

	// Strategy 1: Scan all arguments for file/directory paths
	// Check each path for .janus.config.json in same directory
	for _, arg := range args {
		// Skip executor flags (defensive check - should already be filtered)
		if strings.HasPrefix(arg, "--executor-") {
			continue
		}

		// Check if argument is a path that exists
		fileInfo, err := os.Stat(arg)
		if err != nil {
			// Not a valid path or doesn't exist - try next arg
			continue
		}

		var searchDir string
		if fileInfo.IsDir() {
			searchDir = arg
		} else {
			// It's a file - look in its directory
			searchDir = filepath.Dir(arg)
		}

		// Make search directory absolute
		absSearchDir, err := filepath.Abs(searchDir)
		if err != nil {
			continue
		}

		configPath := filepath.Join(absSearchDir, ".janus.config.json")
		if _, err := os.Stat(configPath); err == nil {
			// Found config! Return it along with the model path
			absModelPath, _ := filepath.Abs(arg)

			return configPath, absModelPath, nil
		}
	}

	// Strategy 2: Fall back to current working directory
	cwd, err := os.Getwd()
	if err == nil {
		configPath := filepath.Join(cwd, ".janus.config.json")
		if _, err := os.Stat(configPath); err == nil {
			return configPath, "", nil
		}
	}

	// Strategy 3: Nothing found - return helpful error
	return "", "", fmt.Errorf("no .janus.config.json found\n" +
		"  Searched: argument file paths and current directory\n" +
		"  Hint: Run 'janus hermes init <model-file>' to create config\n" +
		"        Or use: --executor-hermes-config /path/to/config.json")
}

// loadJanusConfig loads the Janus configuration file using direct YAML unmarshaling.
// Tries multiple locations in priority order:
//  1. Explicit path (if provided)
//  2. Environment variable: $JANUS_CONFIG
//  3. Home directory: ~/.config/janus/config.yaml or config.yml
//  4. Current directory: ./janus.yaml or janus.yml
//
// Returns nil if no config file is found (executor will use defaults).
func loadJanusConfig(explicitPath string) (*config.Config, error) {
	var configPath string

	// Priority 1: Explicit path
	if explicitPath != "" {
		if _, err := os.Stat(explicitPath); err != nil {
			return nil, fmt.Errorf("explicit config not found: %s", explicitPath)
		}
		configPath = explicitPath
	} else if envPath := os.Getenv("JANUS_CONFIG"); envPath != "" {
		// Priority 2: Environment variable
		if _, err := os.Stat(envPath); err == nil {
			configPath = envPath
		}
	} else {
		// Priority 3: Home directory (try both .yaml and .yml)
		if home, err := os.UserHomeDir(); err == nil {
			for _, ext := range []string{"config.yaml", "config.yml"} {
				homePath := filepath.Join(home, ".config", "janus", ext)
				if _, err := os.Stat(homePath); err == nil {
					configPath = homePath

					break
				}
			}
		}

		// Priority 4: Current directory (try both .yaml and .yml)
		if configPath == "" {
			for _, ext := range []string{"janus.yaml", "janus.yml"} {
				if _, err := os.Stat(ext); err == nil {
					configPath = ext

					break
				}
			}
		}
	}

	// No config found - return nil (not an error)
	if configPath == "" {
		return nil, nil
	}

	// Read and unmarshal the config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	// Unmarshal into Input first (since Config embeds Input without tags)
	var input config.Input
	if err := yaml.Unmarshal(data, &input); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config from %s: %w", configPath, err)
	}

	// Use NewConfig to get the same processing as Janus GUI
	// This includes backward compatibility migrations (e.g., hermes.license.path -> nonmem.license.path)
	cfg, err := config.NewConfig(&input)
	if err != nil {
		return nil, fmt.Errorf("failed to process config from %s: %w", configPath, err)
	}

	// Debug output (only if explicitly enabled with true or 1)
	debugEnv := strings.ToLower(os.Getenv("EXECUTOR_DEBUG"))
	if debugEnv == "true" || debugEnv == "1" {
		fmt.Fprintf(os.Stderr, "DEBUG: Loaded Janus config from: %s\n", configPath)
		fmt.Fprintf(os.Stderr, "DEBUG: Organization: %s\n", cfg.Organization)
		fmt.Fprintf(os.Stderr, "DEBUG: Hermes.Container.Port: %d\n", cfg.Hermes.Container.Port)
		fmt.Fprintf(os.Stderr, "DEBUG: Hermes.Container.DockerSocket: '%s'\n", cfg.Hermes.Container.DockerSocket)
		fmt.Fprintf(os.Stderr, "DEBUG: Hermes.Container.Cleanup: %v\n", cfg.Hermes.Container.Cleanup)
	}

	return cfg, nil
}
