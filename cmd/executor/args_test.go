package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseExecutorFlags(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expectedFlags ExecutorFlags
		expectedArgs  []string
	}{
		{
			name:          "no executor flags",
			args:          []string{"model.mod", "model.lst"},
			expectedFlags: ExecutorFlags{},
			expectedArgs:  []string{"model.mod", "model.lst"},
		},
		{
			name:          "quiet flag",
			args:          []string{"--executor-quiet", "model.mod"},
			expectedFlags: ExecutorFlags{Quiet: true},
			expectedArgs:  []string{"model.mod"},
		},
		{
			name:          "help flag",
			args:          []string{"--executor-help"},
			expectedFlags: ExecutorFlags{Help: true},
			expectedArgs:  []string{},
		},
		{
			name:          "version flag",
			args:          []string{"--executor-version"},
			expectedFlags: ExecutorFlags{Version: true},
			expectedArgs:  []string{},
		},
		{
			name:          "no-runlog flag",
			args:          []string{"--executor-no-runlog", "model.mod"},
			expectedFlags: ExecutorFlags{NoRunlog: true},
			expectedArgs:  []string{"model.mod"},
		},
		{
			name:          "explicit config with equals syntax",
			args:          []string{"--executor-hermes-config=/path/to/config.json", "model.mod"},
			expectedFlags: ExecutorFlags{HermesConfig: "/path/to/config.json"},
			expectedArgs:  []string{"model.mod"},
		},
		{
			name:          "explicit config with separate arg",
			args:          []string{"--executor-hermes-config", "/path/to/config.json", "model.mod"},
			expectedFlags: ExecutorFlags{HermesConfig: "/path/to/config.json"},
			expectedArgs:  []string{"model.mod"},
		},
		{
			name:          "license with equals syntax",
			args:          []string{"--executor-license=/path/license.jwt", "model.mod"},
			expectedFlags: ExecutorFlags{License: "/path/license.jwt"},
			expectedArgs:  []string{"model.mod"},
		},
		{
			name:          "license with separate arg",
			args:          []string{"--executor-license", "/path/license.jwt", "model.mod"},
			expectedFlags: ExecutorFlags{License: "/path/license.jwt"},
			expectedArgs:  []string{"model.mod"},
		},
		{
			name:          "mixed executor and tool flags",
			args:          []string{"--executor-quiet", "model.mod", "--maxeval=9999", "-background"},
			expectedFlags: ExecutorFlags{Quiet: true},
			expectedArgs:  []string{"model.mod", "--maxeval=9999", "-background"},
		},
		{
			name: "multiple executor flags",
			args: []string{
				"--executor-quiet",
				"--executor-no-runlog",
				"--executor-license", "/path/license.jwt",
				"model.mod", "model.lst",
			},
			expectedFlags: ExecutorFlags{
				Quiet:    true,
				NoRunlog: true,
				License:  "/path/license.jwt",
			},
			expectedArgs: []string{"model.mod", "model.lst"},
		},
		{
			name:          "NONMEM-style arguments preserved",
			args:          []string{"model.mod", "model.lst", "-maxeval=9999", "-background"},
			expectedFlags: ExecutorFlags{},
			expectedArgs:  []string{"model.mod", "model.lst", "-maxeval=9999", "-background"},
		},
		{
			name:          "Stan-style arguments preserved",
			args:          []string{"model.stan", "--algorithm=hmc", "--num_samples=2000"},
			expectedFlags: ExecutorFlags{},
			expectedArgs:  []string{"model.stan", "--algorithm=hmc", "--num_samples=2000"},
		},
		{
			name:          "Monolix-style arguments preserved",
			args:          []string{"-p", "project.mlxtran", "--export"},
			expectedFlags: ExecutorFlags{},
			expectedArgs:  []string{"-p", "project.mlxtran", "--export"},
		},
		{
			name: "executor flags interspersed with tool args",
			args: []string{
				"--executor-quiet",
				"model.mod",
				"--executor-license=/license.jwt",
				"model.lst",
				"-maxeval=9999",
			},
			expectedFlags: ExecutorFlags{
				Quiet:   true,
				License: "/license.jwt",
			},
			expectedArgs: []string{"model.mod", "model.lst", "-maxeval=9999"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags, containerArgs := parseExecutorFlags(tt.args)

			assert.Equal(t, tt.expectedFlags, flags, "Flags should match")
			assert.Equal(t, tt.expectedArgs, containerArgs, "Container args should match")
		})
	}
}

func TestParseExecutorFlags_EdgeCases(t *testing.T) {
	t.Run("empty args", func(t *testing.T) {
		flags, containerArgs := parseExecutorFlags([]string{})

		assert.Equal(t, ExecutorFlags{}, flags)
		assert.Equal(t, []string{}, containerArgs)
	})

	t.Run("only executor flags", func(t *testing.T) {
		flags, containerArgs := parseExecutorFlags([]string{
			"--executor-quiet",
			"--executor-version",
		})

		assert.True(t, flags.Quiet)
		assert.True(t, flags.Version)
		assert.Equal(t, []string{}, containerArgs)
	})

	t.Run("license flag at end without value", func(t *testing.T) {
		// Edge case: --executor-license at end with no following arg
		// Should not crash, just leave License empty
		flags, containerArgs := parseExecutorFlags([]string{"model.mod", "--executor-license"})

		assert.Equal(t, "", flags.License)
		assert.Equal(t, []string{"model.mod"}, containerArgs)
	})

	t.Run("config flag at end without value", func(t *testing.T) {
		flags, containerArgs := parseExecutorFlags([]string{"model.mod", "--executor-hermes-config"})

		assert.Equal(t, "", flags.HermesConfig)
		assert.Equal(t, []string{"model.mod"}, containerArgs)
	})
}
