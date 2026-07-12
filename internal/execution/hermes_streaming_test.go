//go:build unit
// +build unit

package execution

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pharmalytica/janus/internal/config"
)

func TestSetOutputWriters(t *testing.T) {
	cfg := &config.Config{}
	modelConfig := &config.HermesModelConfig{
		Image: "test/image:latest",
		Resources: config.ResourceConfig{
			CPUCores: 4,
			Memory:   "8Gi",
		},
	}

	executor := NewHermesExecutor(cfg, modelConfig)

	// Initially should be nil
	assert.Nil(t, executor.stdoutWriter)
	assert.Nil(t, executor.stderrWriter)

	// Set writers
	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}
	executor.SetOutputWriters(stdoutBuf, stderrBuf)

	// Should be set
	assert.Equal(t, stdoutBuf, executor.stdoutWriter)
	assert.Equal(t, stderrBuf, executor.stderrWriter)

	// Can set to nil
	executor.SetOutputWriters(nil, nil)
	assert.Nil(t, executor.stdoutWriter)
	assert.Nil(t, executor.stderrWriter)
}

func TestSetOutputWriters_PartialNil(t *testing.T) {
	cfg := &config.Config{}
	modelConfig := &config.HermesModelConfig{
		Image: "test/image:latest",
		Resources: config.ResourceConfig{
			CPUCores: 4,
			Memory:   "8Gi",
		},
	}

	executor := NewHermesExecutor(cfg, modelConfig)

	// Set only stdout writer
	stdoutBuf := &bytes.Buffer{}
	executor.SetOutputWriters(stdoutBuf, nil)

	assert.Equal(t, stdoutBuf, executor.stdoutWriter)
	assert.Nil(t, executor.stderrWriter)

	// Set only stderr writer
	stderrBuf := &bytes.Buffer{}
	executor.SetOutputWriters(nil, stderrBuf)

	assert.Nil(t, executor.stdoutWriter)
	assert.Equal(t, stderrBuf, executor.stderrWriter)
}
