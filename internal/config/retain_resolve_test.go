package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveRetain(t *testing.T) {
	fallback := []string{"*.fallback"}

	t.Run("per-model wins over global and fallback", func(t *testing.T) {
		got := ResolveRetain([]string{"*.model"}, []string{"*.global"}, fallback)
		assert.Equal(t, []string{"*.model"}, got)
	})

	t.Run("global used when per-model empty", func(t *testing.T) {
		got := ResolveRetain(nil, []string{"*.global"}, fallback)
		assert.Equal(t, []string{"*.global"}, got)
	})

	t.Run("fallback used when per-model and global empty", func(t *testing.T) {
		got := ResolveRetain(nil, nil, fallback)
		assert.Equal(t, fallback, got)
	})

	t.Run("returns a fresh slice that does not alias its inputs", func(t *testing.T) {
		perModel := []string{"*.model"}
		got := ResolveRetain(perModel, nil, fallback)
		got[0] = "mutated"
		assert.Equal(t, "*.model", perModel[0], "resolving must not mutate the per-model input")
	})
}

func TestLoadModelRetain(t *testing.T) {
	t.Run("returns the model's own retain", func(t *testing.T) {
		tempDir := t.TempDir()
		modelPath := filepath.Join(tempDir, "model.mod")
		require.NoError(t, SaveModelConfig(modelPath, &ModelConfig{
			Retain: []string{"*.lst", "sdtab*"},
		}))

		assert.Equal(t, []string{"*.lst", "sdtab*"}, LoadModelRetain(modelPath))
	})

	t.Run("nil when there is no config", func(t *testing.T) {
		tempDir := t.TempDir()
		modelPath := filepath.Join(tempDir, "model.mod")

		assert.Nil(t, LoadModelRetain(modelPath))
	})

	t.Run("works for a non-Hermes model (no image/resources)", func(t *testing.T) {
		tempDir := t.TempDir()
		modelPath := filepath.Join(tempDir, "model.mod")

		// A model that declares its output files but has no hermes section.
		require.NoError(t, os.WriteFile(ConfigPath(modelPath),
			[]byte(`{"retain": ["*.lst", "patab*"]}`), 0o600))

		assert.Equal(t, []string{"*.lst", "patab*"}, LoadModelRetain(modelPath))
	})
}
