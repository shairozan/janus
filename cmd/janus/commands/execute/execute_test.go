//go:build unit
// +build unit

package execute

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExecuteCommand(t *testing.T) {
	cmd := Command()

	assert.NotNil(t, cmd)
	assert.Contains(t, cmd.Use, "execute")
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
}

func TestExecuteCommandHasHermesSubcommand(t *testing.T) {
	cmd := Command()

	// Check that hermes subcommand exists
	hermesCmd, _, err := cmd.Find([]string{"hermes"})
	assert.NoError(t, err)
	assert.NotNil(t, hermesCmd)
	assert.Contains(t, hermesCmd.Use, "hermes")
}

func TestHermesCommandStructure(t *testing.T) {
	cmd := hermesCommand()

	assert.NotNil(t, cmd)
	assert.Contains(t, cmd.Use, "hermes")
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
	assert.NotNil(t, cmd.Args)
}

func TestHermesCommandFlags(t *testing.T) {
	cmd := hermesCommand()

	// Check quiet flag
	quietFlag := cmd.Flags().Lookup("quiet")
	assert.NotNil(t, quietFlag)
	assert.Equal(t, "q", quietFlag.Shorthand)
	assert.Equal(t, "false", quietFlag.DefValue)

	// Check detach flag
	detachFlag := cmd.Flags().Lookup("detach")
	assert.NotNil(t, detachFlag)
	assert.Equal(t, "d", detachFlag.Shorthand)
	assert.Equal(t, "false", detachFlag.DefValue)
}

func TestHermesCommandRequiresArgument(t *testing.T) {
	cmd := hermesCommand()

	// Should fail with no arguments
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg(s), received 0")
}

func TestValidateFlagCombinations(t *testing.T) {
	tests := []struct {
		name        string
		quiet       bool
		detach      bool
		expectError bool
		errorMsg    string
	}{
		{
			name:        "quiet only - valid",
			quiet:       true,
			detach:      false,
			expectError: false,
		},
		{
			name:        "detach only - valid",
			quiet:       false,
			detach:      true,
			expectError: false,
		},
		{
			name:        "neither flag - valid (streaming by default)",
			quiet:       false,
			detach:      false,
			expectError: false,
		},
		{
			name:        "both flags - invalid",
			quiet:       true,
			detach:      true,
			expectError: true,
			errorMsg:    "cannot use --quiet and --detach together",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the flag validation logic
			err := runHermes(nil, "/tmp/nonexistent.mod", tt.quiet, tt.detach)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				// These should fail for other reasons (missing file, etc.)
				// but NOT for flag validation
				if err != nil && tt.errorMsg != "" {
					assert.NotContains(t, err.Error(), tt.errorMsg)
				}
			}
		})
	}
}
