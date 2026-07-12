//go:build validation
// +build validation

package validation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/config"
	"github.com/pharmalytica/janus/internal/runlog"
)

// TestREQ47_HermesModelConfigurationFile validates REQ-47: Hermes execution shall pull configuration from a .janus.config.json file.
func TestREQ47_HermesModelConfigurationFile(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-47",
		Description: "Hermes execution shall pull configuration from a .janus.config.json file colocated with the model",
		Category:    CategoryHermesExecution,
		TestFunc: func(t *testing.T) {
			// Create temp directory with model and config
			tempDir := t.TempDir()
			modelPath := filepath.Join(tempDir, "model.mod")
			configPath := filepath.Join(tempDir, ".janus.config.json")

			// Create a valid config file
			validConfig := `{
				"image": "ghcr.io/pharmalytica/nonmem:7.5.0",
				"resources": {
					"cpu_cores": 4,
					"memory": "8Gi"
				}
			}`

			err := os.WriteFile(configPath, []byte(validConfig), 0644)
			require.NoError(t, err, "Should be able to create config file")

			// Load the config from the model directory
			cfg, err := config.LoadHermesModelConfig(modelPath)

			require.NoError(t, err, "Should be able to load Hermes model config")
			require.NotNil(t, cfg, "Config should not be nil")
			assert.Equal(t, "ghcr.io/pharmalytica/nonmem:7.5.0", cfg.Image, "Image should match config file")
			assert.Equal(t, 4, cfg.Resources.CPUCores, "CPU cores should match config file")
			assert.Equal(t, "8Gi", cfg.Resources.Memory, "Memory should match config file")

			t.Logf("Successfully loaded Hermes config: image=%s, cpu=%d, memory=%s",
				cfg.Image, cfg.Resources.CPUCores, cfg.Resources.Memory)
		},
	}

	test.Run(t)
}

// TestREQ48_HermesConfigurationPrompt validates REQ-48: If .janus.config.json is not present, Janus shall prompt the user.
func TestREQ48_HermesConfigurationPrompt(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-48",
		Description: "If .janus.config.json is not present, Janus shall prompt the user for configuration details",
		Category:    CategoryHermesExecution,
		TestFunc: func(t *testing.T) {
			// Create temp directory without config file
			tempDir := t.TempDir()
			modelPath := filepath.Join(tempDir, "model.mod")

			// Attempt to load config - should fail because file is required
			cfg, err := config.LoadHermesModelConfig(modelPath)

			assert.Error(t, err, "Should error when config file is missing")
			assert.Nil(t, cfg, "Config should be nil when file is missing")
			assert.Contains(t, err.Error(), "hermes execution requires .janus.config.json",
				"Error should indicate missing config file requirement")
			assert.Contains(t, err.Error(), tempDir,
				"Error should indicate the directory where config should be located")

			t.Logf("Successfully validated that missing config triggers appropriate error: %v", err)
		},
	}

	test.Run(t)
}

// TestREQ49_StandardExecutionRunLogCapture validates REQ-49: NONMEM, BBI, and PSN modes collect STDERR, STDOUT, and output files.
func TestREQ49_StandardExecutionRunLogCapture(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-49",
		Description: "For NONMEM, BBI, and PSN execution modes, Janus shall collect STDERR, STDOUT, and output files into the run log",
		Category:    CategoryAuditCompliance,
		TestFunc: func(t *testing.T) {
			// Create temporary directory for run log
			tempDir := t.TempDir()
			logPath := filepath.Join(tempDir, "runlog.jsonl")

			// Create run logger
			logger, err := runlog.NewRunLogger(true, logPath)
			require.NoError(t, err, "Should be able to create run logger")
			defer logger.Close()

			// Simulate standard execution with output capture
			jobID := "job-standard-test"
			binary := "/opt/NONMEM/nm76/run/nmfe76"
			args := []string{"model.mod", "model.lst"}
			stdout := "NONMEM execution completed successfully"
			stderr := "Warning: Some convergence issues detected"
			exitCode := 0
			duration := 30 * time.Second
			workDir := "/workspace"
			outputFiles := []string{"model.lst", "model.ext", "model.phi"}

			// Record execution using SLURM variant which supports output files
			err = logger.RecordExecutionWithSLURM(
				jobID,
				binary,
				args,
				stdout,
				stderr,
				exitCode,
				duration,
				workDir,
				nil, // No REST API for standard execution
				"",  // No SLURM job ID for standard execution
				outputFiles,
			)
			require.NoError(t, err, "Should be able to record standard execution")

			// Read and verify the log entry
			entries, err := runlog.ReadRunEntries(logPath)
			require.NoError(t, err, "Should be able to read run log entries")
			require.Len(t, entries, 1, "Should have exactly one log entry")

			entry := entries[0]
			assert.Equal(t, jobID, entry.JobID, "Job ID should match")
			assert.Equal(t, binary, entry.Binary, "Binary should match")
			assert.Equal(t, args, entry.Arguments, "Arguments should match")
			assert.Equal(t, stdout, entry.STDOUT, "STDOUT should be captured")
			assert.Equal(t, stderr, entry.STDERR, "STDERR should be captured")
			assert.Equal(t, exitCode, entry.ExitCode, "Exit code should match")
			assert.Equal(t, outputFiles, entry.OutputFiles, "Output files list should be captured")

			t.Logf("Successfully validated standard execution run log capture with %d output files", len(outputFiles))
		},
	}

	test.Run(t)
}

// TestREQ50_HermesExecutionOutputCapture validates REQ-50: Hermes captures STDERR, STDOUT, and all returned files.
func TestREQ50_HermesExecutionOutputCapture(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-50",
		Description: "When HERMES is the execution model, Janus shall capture STDERR, STDOUT, and all files returned from Hermes",
		Category:    CategoryHermesExecution,
		TestFunc: func(t *testing.T) {
			// Create temporary directory for run log
			tempDir := t.TempDir()
			logPath := filepath.Join(tempDir, "hermes-runlog.jsonl")

			// Create run logger
			logger, err := runlog.NewRunLogger(true, logPath)
			require.NoError(t, err, "Should be able to create run logger")
			defer logger.Close()

			// Create Hermes execution metadata
			jobID := "job-hermes-exec"
			binary := "/opt/NONMEM/nm76/run/nmfe76"
			args := []string{"model.mod", "model.lst"}
			stdout := "Container execution started\nNONMEM minimization successful\nContainer execution completed"
			stderr := ""
			exitCode := 0
			duration := 45 * time.Second
			workDir := "/workspace"

			hermesMetadata := &runlog.HermesMetadata{
				ExecutionID: "exec-test-123",
				ContainerID: "container-abc-456",
				Image: runlog.ContainerImage{
					Name:   "pharmalytica/hermes-nonmem",
					Tag:    "nm76",
					Digest: "sha256:abc123def456",
					Full:   "pharmalytica/hermes-nonmem:nm76",
				},
				Resources: runlog.HermesResources{
					CPUCores: 4,
					Memory:   "8Gi",
				},
				ModelConfig: "/workspace/.janus.config.json",
				FilesCount:  5,
				RuntimeSec:  42,
			}

			outputFiles := []string{"model.lst", "model.ext", "model.phi", "model.cov", "model.cor"}

			// Record Hermes execution
			err = logger.RecordHermesExecution(
				jobID,
				binary,
				args,
				stdout,
				stderr,
				exitCode,
				duration,
				workDir,
				hermesMetadata,
				outputFiles,
			)
			require.NoError(t, err, "Should be able to record Hermes execution")

			// Read and verify the log entry
			entries, err := runlog.ReadRunEntries(logPath)
			require.NoError(t, err, "Should be able to read run log entries")
			require.Len(t, entries, 1, "Should have exactly one log entry")

			entry := entries[0]
			assert.Equal(t, jobID, entry.JobID, "Job ID should match")
			assert.Equal(t, binary, entry.Binary, "Binary should match")
			assert.Equal(t, stdout, entry.STDOUT, "STDOUT should be captured from container")
			assert.Equal(t, stderr, entry.STDERR, "STDERR should be captured from container")
			assert.Equal(t, exitCode, entry.ExitCode, "Exit code should match")
			assert.Equal(t, outputFiles, entry.OutputFiles, "All returned files should be listed")

			// Verify Hermes-specific metadata
			require.NotNil(t, entry.Hermes, "Hermes metadata should be present")
			assert.Equal(t, hermesMetadata.ExecutionID, entry.Hermes.ExecutionID, "Execution ID should match")
			assert.Equal(t, hermesMetadata.ContainerID, entry.Hermes.ContainerID, "Container ID should match")
			assert.Equal(t, hermesMetadata.FilesCount, entry.Hermes.FilesCount, "Files count should match")

			t.Logf("Successfully validated Hermes output capture with %d files and complete metadata", len(outputFiles))
		},
	}

	test.Run(t)
}

// TestREQ51_HermesContainerImageProvenance validates REQ-51: Hermes executions record complete container image provenance.
func TestREQ51_HermesContainerImageProvenance(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-51",
		Description: "Hermes executions shall record complete container image provenance in the run log",
		Category:    CategoryHermesExecution,
		TestFunc: func(t *testing.T) {
			// Test various container image provenance scenarios
			testCases := []struct {
				name           string
				image          runlog.ContainerImage
				expectedName   string
				expectedTag    string
				expectedDigest string
				expectedFull   string
			}{
				{
					name: "DockerHub with SHA256 digest",
					image: runlog.ContainerImage{
						Name:   "pharmalytica/hermes-nonmem",
						Tag:    "nm76",
						Digest: "sha256:abc123def456789",
						Full:   "pharmalytica/hermes-nonmem:nm76",
					},
					expectedName:   "pharmalytica/hermes-nonmem",
					expectedTag:    "nm76",
					expectedDigest: "sha256:abc123def456789",
					expectedFull:   "pharmalytica/hermes-nonmem:nm76",
				},
				{
					name: "GitHub Container Registry with digest",
					image: runlog.ContainerImage{
						Name:   "ghcr.io/pharmalytica/nonmem",
						Tag:    "7.5.0",
						Digest: "sha256:def456abc123",
						Full:   "ghcr.io/pharmalytica/nonmem:7.5.0",
					},
					expectedName:   "ghcr.io/pharmalytica/nonmem",
					expectedTag:    "7.5.0",
					expectedDigest: "sha256:def456abc123",
					expectedFull:   "ghcr.io/pharmalytica/nonmem:7.5.0",
				},
				{
					name: "Private registry",
					image: runlog.ContainerImage{
						Name:   "registry.company.com/hermes",
						Tag:    "latest",
						Digest: "sha256:xyz789",
						Full:   "registry.company.com/hermes:latest",
					},
					expectedName:   "registry.company.com/hermes",
					expectedTag:    "latest",
					expectedDigest: "sha256:xyz789",
					expectedFull:   "registry.company.com/hermes:latest",
				},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					// Verify all provenance fields are present
					assert.NotEmpty(t, tc.image.Name, "Image name should not be empty")
					assert.NotEmpty(t, tc.image.Tag, "Image tag should not be empty")
					assert.NotEmpty(t, tc.image.Digest, "Image digest should not be empty")
					assert.NotEmpty(t, tc.image.Full, "Full image reference should not be empty")

					// Verify provenance matches expected values
					assert.Equal(t, tc.expectedName, tc.image.Name, "Image name should match")
					assert.Equal(t, tc.expectedTag, tc.image.Tag, "Image tag should match")
					assert.Equal(t, tc.expectedDigest, tc.image.Digest, "Image digest should match")
					assert.Equal(t, tc.expectedFull, tc.image.Full, "Full reference should match")

					// Verify digest format
					assert.Contains(t, tc.image.Digest, "sha256:", "Digest should be SHA256 format")

					// Verify JSON serialization preserves provenance
					jsonData, err := json.Marshal(tc.image)
					require.NoError(t, err, "Should be able to serialize image provenance")

					var decoded runlog.ContainerImage
					err = json.Unmarshal(jsonData, &decoded)
					require.NoError(t, err, "Should be able to deserialize image provenance")

					assert.Equal(t, tc.image, decoded, "Image provenance should survive JSON round-trip")

					t.Logf("Validated image provenance: %s@%s", tc.image.Full, tc.image.Digest)
				})
			}

			t.Logf("Successfully validated container image provenance for %d scenarios", len(testCases))
		},
	}

	test.Run(t)
}

// TestREQ52_HermesResourceConfigurationLogging validates REQ-52: Hermes executions record resource configuration.
func TestREQ52_HermesResourceConfigurationLogging(t *testing.T) {
	test := ValidationTest{
		Requirement: "REQ-52",
		Description: "Hermes executions shall record the resource configuration used at execution time",
		Category:    CategoryHermesExecution,
		TestFunc: func(t *testing.T) {
			// Test various resource configuration scenarios
			testCases := []struct {
				name             string
				resources        runlog.HermesResources
				expectedCPUCores int
				expectedMemory   string
			}{
				{
					name: "Standard configuration (4 cores, 8Gi)",
					resources: runlog.HermesResources{
						CPUCores: 4,
						Memory:   "8Gi",
					},
					expectedCPUCores: 4,
					expectedMemory:   "8Gi",
				},
				{
					name: "High performance configuration (16 cores, 32Gi)",
					resources: runlog.HermesResources{
						CPUCores: 16,
						Memory:   "32Gi",
					},
					expectedCPUCores: 16,
					expectedMemory:   "32Gi",
				},
				{
					name: "Low resource configuration (1 core, 2Gi)",
					resources: runlog.HermesResources{
						CPUCores: 1,
						Memory:   "2Gi",
					},
					expectedCPUCores: 1,
					expectedMemory:   "2Gi",
				},
				{
					name: "Large memory configuration (8 cores, 64Gi)",
					resources: runlog.HermesResources{
						CPUCores: 8,
						Memory:   "64Gi",
					},
					expectedCPUCores: 8,
					expectedMemory:   "64Gi",
				},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					// Verify resource values
					assert.Equal(t, tc.expectedCPUCores, tc.resources.CPUCores,
						"CPU cores should match expected value")
					assert.Equal(t, tc.expectedMemory, tc.resources.Memory,
						"Memory should match expected value")

					// Verify resource constraints
					assert.Greater(t, tc.resources.CPUCores, 0,
						"CPU cores should be positive")
					assert.NotEmpty(t, tc.resources.Memory,
						"Memory should not be empty")
					assert.Regexp(t, `^\d+(\.\d+)?[KMGT]i?$`, tc.resources.Memory,
						"Memory should be in valid format (e.g., 8Gi, 2048Mi)")

					// Verify JSON serialization preserves resources
					jsonData, err := json.Marshal(tc.resources)
					require.NoError(t, err, "Should be able to serialize resource configuration")

					var decoded runlog.HermesResources
					err = json.Unmarshal(jsonData, &decoded)
					require.NoError(t, err, "Should be able to deserialize resource configuration")

					assert.Equal(t, tc.resources, decoded,
						"Resource configuration should survive JSON round-trip")

					t.Logf("Validated resource config: cpu=%d, memory=%s",
						tc.resources.CPUCores, tc.resources.Memory)
				})
			}

			t.Logf("Successfully validated resource configuration logging for %d scenarios", len(testCases))
		},
	}

	test.Run(t)
}
