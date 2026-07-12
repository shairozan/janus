//go:build unit
// +build unit

package runlog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRecordHermesExecution tests recording Hermes container execution events.
func TestRecordHermesExecution(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "hermes-runlog.jsonl")

	logger, err := NewRunLogger(true, logPath)
	require.NoError(t, err)
	defer logger.Close()

	// Create test data
	jobID := "job-hermes-123"
	binary := "/opt/NONMEM/nm76/run/nmfe76"
	args := []string{"model.mod", "model.lst"}
	stdout := "NONMEM execution started\nMinimization successful\n"
	stderr := ""
	exitCode := 0
	duration := 45 * time.Second

	hermesMetadata := &HermesMetadata{
		ExecutionID: "exec-abc123",
		ContainerID: "container-xyz789",
		Image: ContainerImage{
			Name:   "pharmalytica/hermes-nonmem",
			Tag:    "nm76",
			Digest: "sha256:abc123def456",
			Full:   "pharmalytica/hermes-nonmem:nm76",
		},
		Resources: HermesResources{
			CPUCores: 4,
			Memory:   "8Gi",
		},
		ModelConfig: "/path/to/.janus.config.json",
		FilesCount:  5,
		RuntimeSec:  42,
	}

	outputFiles := []string{"model.lst", "model.ext", "model.phi"}

	// Record execution
	err = logger.RecordHermesExecution(
		jobID,
		binary,
		args,
		stdout,
		stderr,
		exitCode,
		duration,
		"/workspace",
		hermesMetadata,
		outputFiles,
	)
	require.NoError(t, err)

	// Read and verify the log entry
	entries, err := ReadRunEntries(logPath)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	entry := entries[0]
	assert.Equal(t, jobID, entry.JobID)
	assert.Equal(t, binary, entry.Binary)
	assert.Equal(t, args, entry.Arguments)
	assert.Equal(t, stdout, entry.STDOUT)
	assert.Equal(t, stderr, entry.STDERR)
	assert.Equal(t, exitCode, entry.ExitCode)
	assert.Equal(t, duration.Milliseconds(), entry.Duration)
	assert.Equal(t, "/workspace", entry.WorkDir)
	assert.Equal(t, outputFiles, entry.OutputFiles)

	// Verify Hermes metadata
	require.NotNil(t, entry.Hermes)
	assert.Equal(t, "exec-abc123", entry.Hermes.ExecutionID)
	assert.Equal(t, "container-xyz789", entry.Hermes.ContainerID)
	assert.Equal(t, "pharmalytica/hermes-nonmem", entry.Hermes.Image.Name)
	assert.Equal(t, "nm76", entry.Hermes.Image.Tag)
	assert.Equal(t, "sha256:abc123def456", entry.Hermes.Image.Digest)
	assert.Equal(t, "pharmalytica/hermes-nonmem:nm76", entry.Hermes.Image.Full)
	assert.Equal(t, 4, entry.Hermes.Resources.CPUCores)
	assert.Equal(t, "8Gi", entry.Hermes.Resources.Memory)
	assert.Equal(t, "/path/to/.janus.config.json", entry.Hermes.ModelConfig)
	assert.Equal(t, 5, entry.Hermes.FilesCount)
	assert.Equal(t, int64(42), entry.Hermes.RuntimeSec)

	t.Logf("Successfully recorded and verified Hermes execution metadata")
}

// TestHermesMetadataJSONSerialization tests JSON serialization of Hermes metadata.
func TestHermesMetadataJSONSerialization(t *testing.T) {
	metadata := HermesMetadata{
		ExecutionID: "exec-test",
		ContainerID: "cont-test",
		Image: ContainerImage{
			Name:   "test/image",
			Tag:    "v1.0",
			Digest: "sha256:test123",
			Full:   "test/image:v1.0",
		},
		Resources: HermesResources{
			CPUCores: 2,
			Memory:   "4Gi",
		},
		ModelConfig: "/config/.janus.config.json",
		FilesCount:  3,
		RuntimeSec:  30,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(metadata)
	require.NoError(t, err)

	// Unmarshal back
	var decoded HermesMetadata
	err = json.Unmarshal(jsonData, &decoded)
	require.NoError(t, err)

	// Verify round-trip
	assert.Equal(t, metadata.ExecutionID, decoded.ExecutionID)
	assert.Equal(t, metadata.ContainerID, decoded.ContainerID)
	assert.Equal(t, metadata.Image.Name, decoded.Image.Name)
	assert.Equal(t, metadata.Image.Tag, decoded.Image.Tag)
	assert.Equal(t, metadata.Image.Digest, decoded.Image.Digest)
	assert.Equal(t, metadata.Resources.CPUCores, decoded.Resources.CPUCores)
	assert.Equal(t, metadata.Resources.Memory, decoded.Resources.Memory)
	assert.Equal(t, metadata.FilesCount, decoded.FilesCount)
	assert.Equal(t, metadata.RuntimeSec, decoded.RuntimeSec)
}

// TestContainerImageProvenance tests container image provenance capture (REQ-51).
func TestContainerImageProvenance(t *testing.T) {
	tests := []struct {
		name           string
		image          ContainerImage
		expectedName   string
		expectedTag    string
		expectedDigest string
		expectedFull   string
	}{
		{
			name: "DockerHub with digest",
			image: ContainerImage{
				Name:   "pharmalytica/hermes-nonmem",
				Tag:    "nm76",
				Digest: "sha256:abc123",
				Full:   "pharmalytica/hermes-nonmem:nm76",
			},
			expectedName:   "pharmalytica/hermes-nonmem",
			expectedTag:    "nm76",
			expectedDigest: "sha256:abc123",
			expectedFull:   "pharmalytica/hermes-nonmem:nm76",
		},
		{
			name: "Private registry without digest",
			image: ContainerImage{
				Name:   "registry.company.com/hermes",
				Tag:    "latest",
				Digest: "",
				Full:   "registry.company.com/hermes:latest",
			},
			expectedName:   "registry.company.com/hermes",
			expectedTag:    "latest",
			expectedDigest: "",
			expectedFull:   "registry.company.com/hermes:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedName, tt.image.Name)
			assert.Equal(t, tt.expectedTag, tt.image.Tag)
			assert.Equal(t, tt.expectedDigest, tt.image.Digest)
			assert.Equal(t, tt.expectedFull, tt.image.Full)

			// Verify JSON serialization
			jsonData, err := json.Marshal(tt.image)
			require.NoError(t, err)

			var decoded ContainerImage
			err = json.Unmarshal(jsonData, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.image, decoded)
		})
	}
}

// TestHermesResourcesConfiguration tests resource configuration logging (REQ-52).
func TestHermesResourcesConfiguration(t *testing.T) {
	tests := []struct {
		name             string
		resources        HermesResources
		expectedCPUCores int
		expectedMemory   string
	}{
		{
			name: "Standard configuration",
			resources: HermesResources{
				CPUCores: 4,
				Memory:   "8Gi",
			},
			expectedCPUCores: 4,
			expectedMemory:   "8Gi",
		},
		{
			name: "High performance configuration",
			resources: HermesResources{
				CPUCores: 16,
				Memory:   "32Gi",
			},
			expectedCPUCores: 16,
			expectedMemory:   "32Gi",
		},
		{
			name: "Low resource configuration",
			resources: HermesResources{
				CPUCores: 1,
				Memory:   "2Gi",
			},
			expectedCPUCores: 1,
			expectedMemory:   "2Gi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedCPUCores, tt.resources.CPUCores)
			assert.Equal(t, tt.expectedMemory, tt.resources.Memory)

			// Verify JSON serialization preserves resource values
			jsonData, err := json.Marshal(tt.resources)
			require.NoError(t, err)

			var decoded HermesResources
			err = json.Unmarshal(jsonData, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.resources, decoded)
		})
	}
}

// TestRecordHermesExecutionDisabled tests that logging is skipped when disabled.
func TestRecordHermesExecutionDisabled(t *testing.T) {
	logger, err := NewRunLogger(false, "/dev/null")
	require.NoError(t, err)
	defer logger.Close()

	// Attempt to record execution with disabled logger
	err = logger.RecordHermesExecution(
		"job-123",
		"binary",
		[]string{"arg1"},
		"stdout",
		"stderr",
		0,
		time.Second,
		"/workspace",
		&HermesMetadata{},
		[]string{},
	)

	// Should not error, just skip
	assert.NoError(t, err)
}

// TestMultipleHermesExecutions tests recording multiple Hermes executions to the same log.
func TestMultipleHermesExecutions(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "multi-hermes.jsonl")

	logger, err := NewRunLogger(true, logPath)
	require.NoError(t, err)
	defer logger.Close()

	// Record first execution
	err = logger.RecordHermesExecution(
		"job-1",
		"/opt/NONMEM/nm76/run/nmfe76",
		[]string{"model1.mod", "model1.lst"},
		"stdout1",
		"stderr1",
		0,
		30*time.Second,
		"/workspace",
		&HermesMetadata{
			ExecutionID: "exec-1",
			ContainerID: "cont-1",
			Image: ContainerImage{
				Name: "test/image",
				Tag:  "v1",
				Full: "test/image:v1",
			},
			Resources: HermesResources{
				CPUCores: 2,
				Memory:   "4Gi",
			},
		},
		[]string{"model1.lst"},
	)
	require.NoError(t, err)

	// Record second execution
	err = logger.RecordHermesExecution(
		"job-2",
		"/opt/NONMEM/nm76/run/nmfe76",
		[]string{"model2.mod", "model2.lst"},
		"stdout2",
		"stderr2",
		0,
		45*time.Second,
		"/workspace",
		&HermesMetadata{
			ExecutionID: "exec-2",
			ContainerID: "cont-2",
			Image: ContainerImage{
				Name: "test/image",
				Tag:  "v2",
				Full: "test/image:v2",
			},
			Resources: HermesResources{
				CPUCores: 4,
				Memory:   "8Gi",
			},
		},
		[]string{"model2.lst", "model2.ext"},
	)
	require.NoError(t, err)

	// Read and verify both entries
	entries, err := ReadRunEntries(logPath)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	// Verify first entry
	assert.Equal(t, "job-1", entries[0].JobID)
	assert.Equal(t, "exec-1", entries[0].Hermes.ExecutionID)
	assert.Equal(t, "v1", entries[0].Hermes.Image.Tag)
	assert.Equal(t, 2, entries[0].Hermes.Resources.CPUCores)

	// Verify second entry
	assert.Equal(t, "job-2", entries[1].JobID)
	assert.Equal(t, "exec-2", entries[1].Hermes.ExecutionID)
	assert.Equal(t, "v2", entries[1].Hermes.Image.Tag)
	assert.Equal(t, 4, entries[1].Hermes.Resources.CPUCores)

	t.Logf("Successfully recorded and verified %d Hermes executions", len(entries))
}

// TestHermesRunLogFileFormat tests the JSONL file format for Hermes entries.
func TestHermesRunLogFileFormat(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "format-test.jsonl")

	logger, err := NewRunLogger(true, logPath)
	require.NoError(t, err)
	defer logger.Close()

	// Record an execution
	err = logger.RecordHermesExecution(
		"job-format-test",
		"/opt/NONMEM/nm76/run/nmfe76",
		[]string{"test.mod"},
		"test output",
		"",
		0,
		10*time.Second,
		"/workspace",
		&HermesMetadata{
			ExecutionID: "exec-format",
			ContainerID: "cont-format",
			Image: ContainerImage{
				Name: "test/image",
				Tag:  "latest",
				Full: "test/image:latest",
			},
			Resources: HermesResources{
				CPUCores: 1,
				Memory:   "2Gi",
			},
		},
		[]string{"test.lst"},
	)
	require.NoError(t, err)

	// Read raw file content
	data, err := os.ReadFile(logPath)
	require.NoError(t, err)

	// Verify it's valid JSON
	var entry RunEntry
	err = json.Unmarshal(data, &entry)
	require.NoError(t, err)

	// Verify file ends with newline (JSONL format)
	assert.True(t, len(data) > 0 && data[len(data)-1] == '\n', "JSONL file should end with newline")
}
