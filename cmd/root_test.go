//go:build unit
// +build unit

package cmd

import (
	"fmt"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestRootCommand_ArgumentValidation(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "no arguments - valid",
			args:        []string{},
			expectError: false,
		},
		{
			name:        "one positional argument - valid",
			args:        []string{"testdata/model.mod"},
			expectError: false,
		},
		{
			name:        "too many positional args - error",
			args:        []string{"model1.mod", "model2.mod"},
			expectError: true,
			errorMsg:    "accepts at most 1 arg(s), received 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := Command()

			// Override RunE to avoid actually running the GUI
			cmd.RunE = func(c *cobra.Command, args []string) error {
				// Just validate that we got the expected args
				return nil
			}

			// Validate arguments
			cmd.SetArgs(tt.args)
			err := cmd.Execute()

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRootCommand_ArgumentParsing(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		modelFlag     string
		expectError   bool
		errorMsg      string
		expectedModel string
	}{
		{
			name:          "no arguments - no model",
			args:          []string{},
			modelFlag:     "",
			expectError:   false,
			expectedModel: "",
		},
		{
			name:          "positional argument",
			args:          []string{"model.mod"},
			modelFlag:     "",
			expectError:   false,
			expectedModel: "model.mod",
		},
		{
			name:          "flag argument",
			args:          []string{},
			modelFlag:     "model.mod",
			expectError:   false,
			expectedModel: "model.mod",
		},
		{
			name:        "both positional and flag - error",
			args:        []string{"model1.mod"},
			modelFlag:   "model2.mod",
			expectError: true,
			errorMsg:    "model file specified both as positional argument and --model flag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedModelArgs []string

			cmd := Command()

			// Override RunE to capture the model args without running GUI
			cmd.RunE = func(c *cobra.Command, args []string) error {
				modelPath, _ := c.Flags().GetString("model")

				if len(args) > 0 {
					if modelPath != "" {
						return fmt.Errorf("model file specified both as positional argument and --model flag; use one or the other")
					}
					capturedModelArgs = []string{args[0]}
				} else if modelPath != "" {
					capturedModelArgs = []string{modelPath}
				}

				return nil
			}

			// Set the --model flag if provided in test
			if tt.modelFlag != "" {
				cmd.Flags().Set("model", tt.modelFlag)
			}

			cmd.SetArgs(tt.args)
			err := cmd.Execute()

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				if tt.expectedModel == "" {
					assert.Empty(t, capturedModelArgs)
				} else {
					assert.Equal(t, []string{tt.expectedModel}, capturedModelArgs)
				}
			}
		})
	}
}

func TestRootCommand_Structure(t *testing.T) {
	cmd := Command()

	// Verify command structure
	assert.Equal(t, "janus [model-file]", cmd.Use)
	assert.Contains(t, cmd.Short, "NONMEM")
	assert.NotNil(t, cmd.RunE)

	// Verify subcommands exist
	subcommands := cmd.Commands()
	subcommandNames := make([]string, len(subcommands))
	for i, subcmd := range subcommands {
		subcommandNames[i] = subcmd.Name()
	}

	// Note: 'gui' subcommand removed - root command now launches GUI by default
	assert.Contains(t, subcommandNames, "version")
	assert.Contains(t, subcommandNames, "hermes")
	assert.Contains(t, subcommandNames, "execute")
}

func TestRootCommand_Flags(t *testing.T) {
	cmd := Command()

	// Verify important flags exist
	modelFlag := cmd.Flags().Lookup("model")
	assert.NotNil(t, modelFlag)
	assert.Equal(t, "string", modelFlag.Value.Type())

	configFlag := cmd.PersistentFlags().Lookup("config")
	assert.NotNil(t, configFlag)
}
