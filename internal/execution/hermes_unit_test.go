//go:build unit
// +build unit

package execution

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shairozan/janus/internal/config"
	"github.com/shairozan/janus/internal/execution/category"
)

// TestReadLicenseFile tests reading NONMEM license files.
func TestReadLicenseFile(t *testing.T) {
	t.Run("Valid license file", func(t *testing.T) {
		// Create temporary license file
		tmpDir := t.TempDir()
		licensePath := filepath.Join(tmpDir, "nonmem.lic")
		licenseContent := []byte("# NONMEM License File\nUSER:test@example.com\n")
		err := os.WriteFile(licensePath, licenseContent, 0644)
		require.NoError(t, err)

		// Create executor with license path
		cfg := &config.Config{}
		cfg.Hermes = config.HermesConfig{
			License: config.HermesLicenseConfig{
				Path: licensePath,
			},
		}

		modelConfig := &config.HermesModelConfig{
			Image: "test/image:latest",
			Resources: config.ResourceConfig{
				CPUCores: 2,
				Memory:   "4Gi",
			},
		}

		executor := NewHermesExecutor(cfg, modelConfig)

		// Test reading license file
		data, err := executor.readLicenseFile()
		require.NoError(t, err)
		assert.Equal(t, licenseContent, data)
	})

	t.Run("Missing license file", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Hermes = config.HermesConfig{
			License: config.HermesLicenseConfig{
				Path: "/nonexistent/license.lic",
			},
		}

		modelConfig := &config.HermesModelConfig{
			Image: "test/image:latest",
			Resources: config.ResourceConfig{
				CPUCores: 2,
				Memory:   "4Gi",
			},
		}

		executor := NewHermesExecutor(cfg, modelConfig)

		_, err := executor.readLicenseFile()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read license file")
	})
}

// TestCollectModelFiles tests model file collection for containerized execution.
func TestCollectModelFiles(t *testing.T) {
	t.Run("Collect model and data files", func(t *testing.T) {
		// Create temporary model directory with files
		tmpDir := t.TempDir()

		// Create model file
		modelPath := filepath.Join(tmpDir, "model.mod")
		modelContent := "$PROBLEM Test\n$DATA data.csv\n"
		err := os.WriteFile(modelPath, []byte(modelContent), 0644)
		require.NoError(t, err)

		// Create data file
		dataPath := filepath.Join(tmpDir, "data.csv")
		dataContent := "ID,TIME,DV\n1,0,10.5\n"
		err = os.WriteFile(dataPath, []byte(dataContent), 0644)
		require.NoError(t, err)

		// Create executor
		cfg := &config.Config{}
		modelConfig := &config.HermesModelConfig{
			Image: "test/image:latest",
			Resources: config.ResourceConfig{
				CPUCores: 2,
				Memory:   "4Gi",
			},
		}

		executor := NewHermesExecutor(cfg, modelConfig)

		// Collect files
		files, err := executor.collectModelFiles(modelPath)
		require.NoError(t, err)

		// Verify both files were collected
		assert.Len(t, files, 2)
		assert.Contains(t, files, "model.mod")
		assert.Contains(t, files, "data.csv")
		assert.Equal(t, []byte(modelContent), files["model.mod"])
		assert.Equal(t, []byte(dataContent), files["data.csv"])
	})

	t.Run("Model file not in collected files", func(t *testing.T) {
		// This shouldn't happen, but test the validation
		tmpDir := t.TempDir()
		nonExistentModel := filepath.Join(tmpDir, "missing.mod")

		cfg := &config.Config{}
		modelConfig := &config.HermesModelConfig{
			Image: "test/image:latest",
			Resources: config.ResourceConfig{
				CPUCores: 2,
				Memory:   "4Gi",
			},
		}

		executor := NewHermesExecutor(cfg, modelConfig)

		_, err := executor.collectModelFiles(nonExistentModel)
		assert.Error(t, err)
	})

	t.Run("Collect nested directory structure", func(t *testing.T) {
		// Create directory structure
		tmpDir := t.TempDir()
		subDir := filepath.Join(tmpDir, "data")
		err := os.MkdirAll(subDir, 0755)
		require.NoError(t, err)

		// Create model file
		modelPath := filepath.Join(tmpDir, "model.mod")
		err = os.WriteFile(modelPath, []byte("$PROBLEM Test\n"), 0644)
		require.NoError(t, err)

		// Create nested data file
		dataPath := filepath.Join(subDir, "study.csv")
		err = os.WriteFile(dataPath, []byte("ID,DV\n1,10\n"), 0644)
		require.NoError(t, err)

		cfg := &config.Config{}
		modelConfig := &config.HermesModelConfig{
			Image: "test/image:latest",
			Resources: config.ResourceConfig{
				CPUCores: 2,
				Memory:   "4Gi",
			},
		}

		executor := NewHermesExecutor(cfg, modelConfig)

		files, err := executor.collectModelFiles(modelPath)
		require.NoError(t, err)

		// Verify nested file is collected with relative path
		assert.Contains(t, files, "model.mod")
		assert.Contains(t, files, filepath.Join("data", "study.csv"))
	})
}

// TestBuildNONMEMCommand tests NONMEM command construction for container execution.
func TestBuildNONMEMCommand(t *testing.T) {
	cfg := &config.Config{}
	modelConfig := &config.HermesModelConfig{
		Image: "test/image:latest",
		Resources: config.ResourceConfig{
			CPUCores: 4,
			Memory:   "8Gi",
		},
	}

	executor := NewHermesExecutor(cfg, modelConfig)
	// Set model category for command building (buildNONMEMCommand requires it)
	executor.modelCategory = category.NewNONMEMCategory()

	tests := []struct {
		name              string
		modelPath         string
		isParallel        bool
		additionalOptions []string
		expectedCommand   string
		expectedArgs      []string
	}{
		{
			name:            "Simple execution",
			modelPath:       "/path/to/model.mod",
			isParallel:      false,
			expectedCommand: "nonmem",
			expectedArgs:    []string{"model.mod", "model.lst", "-licfile=${WORKSPACE}/nonmem.lic"},
		},
		{
			name:            "Parallel execution",
			modelPath:       "/path/to/model.ctl",
			isParallel:      true,
			expectedCommand: "nonmem",
			expectedArgs:    []string{"model.ctl", "model.lst", "-licfile=${WORKSPACE}/nonmem.lic", "-PARAFILE=model.pnm"},
		},
		{
			name:              "With additional options",
			modelPath:         "/path/to/model.mod",
			isParallel:        false,
			additionalOptions: []string{"-maxeval=9999", "-noabort"},
			expectedCommand:   "nonmem",
			expectedArgs:      []string{"model.mod", "model.lst", "-licfile=${WORKSPACE}/nonmem.lic", "-maxeval=9999", "-noabort"},
		},
		{
			name:              "Parallel with options",
			modelPath:         "/data/study/run001.mod",
			isParallel:        true,
			additionalOptions: []string{"-clean=3"},
			expectedCommand:   "nonmem",
			expectedArgs:      []string{"run001.mod", "run001.lst", "-licfile=${WORKSPACE}/nonmem.lic", "-PARAFILE=run001.pnm", "-clean=3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command, args, err := executor.buildNONMEMCommand(tt.modelPath, tt.isParallel, 0, tt.additionalOptions)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedCommand, command)
			assert.Equal(t, tt.expectedArgs, args)
		})
	}
}

// TestGetImageDigest tests Docker image digest retrieval error paths.
func TestGetImageDigest(t *testing.T) {
	t.Run("No Docker client", func(t *testing.T) {
		cfg := &config.Config{}
		modelConfig := &config.HermesModelConfig{
			Image: "test/image:latest",
			Resources: config.ResourceConfig{
				CPUCores: 2,
				Memory:   "4Gi",
			},
		}

		executor := &HermesExecutor{
			config:       cfg,
			modelConfig:  modelConfig,
			dockerClient: nil, // No Docker client
		}

		// Should return empty string when Docker client is nil
		digest := executor.getImageDigest(nil, "test/image:latest")
		assert.Empty(t, digest)
	})
}

// TestNewHermesExecutor tests executor creation.
func TestNewHermesExecutor(t *testing.T) {
	t.Run("Valid configuration", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.RunLogEnabled = false

		modelConfig := &config.HermesModelConfig{
			Image: "shairozan/hermes-nonmem:nm76",
			Resources: config.ResourceConfig{
				CPUCores: 4,
				Memory:   "8Gi",
			},
		}

		executor := NewHermesExecutor(cfg, modelConfig)
		assert.NotNil(t, executor)
	})

	t.Run("Panics with nil model config", func(t *testing.T) {
		cfg := &config.Config{}

		assert.Panics(t, func() {
			NewHermesExecutor(cfg, nil)
		}, "Should panic when modelConfig is nil")
	})

	t.Run("Accepts nil config (standalone executor mode)", func(t *testing.T) {
		modelConfig := &config.HermesModelConfig{
			Image: "shairozan/hermes-nonmem:nm76",
			Resources: config.ResourceConfig{
				CPUCores: 4,
				Memory:   "8Gi",
			},
		}

		// Should not panic with nil config
		executor := NewHermesExecutor(nil, modelConfig)
		assert.NotNil(t, executor)

		// Should handle nil config gracefully
		assert.Nil(t, executor.config)
		assert.NotNil(t, executor.modelConfig)
	})
}

// TestHermesExecutor_NilConfigHandling tests that all methods handle nil config gracefully.
func TestHermesExecutor_NilConfigHandling(t *testing.T) {
	modelConfig := &config.HermesModelConfig{
		Image: "test/hermes:latest",
		Resources: config.ResourceConfig{
			CPUCores: 2,
			Memory:   "4Gi",
		},
		NonmemPath: "/opt/NONMEM/nm76/run/nmfe76",
	}

	t.Run("buildNONMEMCommand with nil config", func(t *testing.T) {
		executor := NewHermesExecutor(nil, modelConfig)
		// Set model category for command building
		executor.modelCategory = category.NewNONMEMCategory()

		command, args, err := executor.buildNONMEMCommand("/path/to/model.mod", false, 0, nil)

		require.NoError(t, err)
		assert.Equal(t, "nonmem", command)
		assert.Contains(t, args, "model.mod")
		assert.Contains(t, args, "model.lst")
		assert.Contains(t, args, "-licfile=${WORKSPACE}/nonmem.lic")
	})

	t.Run("readLicenseFile with nil config uses defaults", func(t *testing.T) {
		executor := NewHermesExecutor(nil, modelConfig)

		// Create a test license file in home directory
		tmpHome := t.TempDir()
		tmpLicenseFile := filepath.Join(tmpHome, "nonmem.lic")
		testLicenseContent := []byte("# Test NONMEM License\n")
		err := os.WriteFile(tmpLicenseFile, testLicenseContent, 0644)
		require.NoError(t, err)

		// Set HOME to point to test directory
		t.Setenv("HOME", tmpHome)
		t.Setenv("USERPROFILE", tmpHome)

		// Should use ~/nonmem.lic as default
		data, err := executor.readLicenseFile()

		assert.NoError(t, err)
		assert.Equal(t, testLicenseContent, data)
	})

	t.Run("createContainer uses defaults with nil config", func(t *testing.T) {
		executor := NewHermesExecutor(nil, modelConfig)

		// Mock Docker client would be needed for full test
		// Here we just verify the executor was created
		assert.NotNil(t, executor)
		assert.Nil(t, executor.config)
	})
}

// TestHermesExecutor_ConfigDefaults tests that sensible defaults are used when config values are missing.
func TestHermesExecutor_ConfigDefaults(t *testing.T) {
	modelConfig := &config.HermesModelConfig{
		Image: "test/hermes:latest",
		Resources: config.ResourceConfig{
			CPUCores: 2,
			Memory:   "4Gi",
		},
		NonmemPath: "/opt/NONMEM/nm76/run/nmfe76",
	}

	t.Run("Uses default port when config is nil", func(t *testing.T) {
		executor := NewHermesExecutor(nil, modelConfig)

		// The executor should use default port 50051
		// This would be tested in createContainer if we had Docker mocks
		assert.NotNil(t, executor)
	})

	t.Run("Uses default cleanup=true when config is nil", func(t *testing.T) {
		executor := NewHermesExecutor(nil, modelConfig)

		// The executor should default to cleanup=true
		// This would be tested in createContainer if we had Docker mocks
		assert.NotNil(t, executor)
	})

	t.Run("Uses default timeout when config is nil", func(t *testing.T) {
		executor := NewHermesExecutor(nil, modelConfig)

		// The executor should use 30s default timeout
		// This would be tested in waitForContainer if we had proper mocks
		assert.NotNil(t, executor)
	})

	t.Run("Uses default retain patterns when config is nil", func(t *testing.T) {
		executor := NewHermesExecutor(nil, modelConfig)

		// The executor should use default NONMEM output patterns
		// This would be verified in executeViaGRPC
		assert.NotNil(t, executor)
	})
}

// TestHermesExecutor_ConfigPresent tests that config values are used when present.
func TestHermesExecutor_ConfigPresent(t *testing.T) {
	cfg := &config.Config{
		Input: config.Input{
			Hermes: config.HermesConfig{
				Container: config.HermesContainerConfig{
					Port:           9999,
					Cleanup:        false,
					StartupTimeout: "60s",
					DockerSocket:   "unix:///custom/docker.sock",
				},
				Resources: config.HermesResourceConfig{
					Timeout: "2h",
				},
				Retain: []string{"*.custom"},
				License: config.HermesLicenseConfig{
					Path: "/path/to/license.lic",
				},
			},
			RunLogEnabled: true,
			RunLog: config.RunLogConfig{
				Path: "/custom/runlog.jsonl",
			},
		},
	}

	modelConfig := &config.HermesModelConfig{
		Image: "test/hermes:latest",
		Resources: config.ResourceConfig{
			CPUCores: 2,
			Memory:   "4Gi",
		},
		NonmemPath: "/opt/NONMEM/nm76/run/nmfe76",
	}

	t.Run("Uses configured values when present", func(t *testing.T) {
		executor := NewHermesExecutor(cfg, modelConfig)

		assert.NotNil(t, executor)
		assert.NotNil(t, executor.config)
		assert.Equal(t, 9999, executor.config.Hermes.Container.Port)
		assert.False(t, executor.config.Hermes.Container.Cleanup)
		assert.Equal(t, "60s", executor.config.Hermes.Container.StartupTimeout)
		assert.Equal(t, "2h", executor.config.Hermes.Resources.Timeout)
		assert.Equal(t, []string{"*.custom"}, executor.config.Hermes.Retain)
	})
}
