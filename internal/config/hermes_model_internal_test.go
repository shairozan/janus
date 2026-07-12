//go:build unit
// +build unit

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMigrateFromLegacyFormat tests the migration function independently.
// This allows testing migration logic in isolation from file I/O.
func TestMigrateFromLegacyFormat(t *testing.T) {
	t.Run("migrates valid legacy format", func(t *testing.T) {
		legacyJSON := []byte(`{
			"correlation_strategy": "label-first",
			"image": "ghcr.io/test/image:v1",
			"container_command_path": "/opt/NONMEM/nm75/run/nmfe75",
			"resources": {
				"cpu_cores": 4,
				"memory": "8Gi"
			}
		}`)

		cfg, migrated, err := migrateFromLegacyFormat(legacyJSON)

		require.NoError(t, err)
		assert.True(t, migrated)
		require.NotNil(t, cfg)
		assert.Equal(t, "label-first", cfg.CorrelationStrategy)
		require.NotNil(t, cfg.Hermes)
		assert.Equal(t, "ghcr.io/test/image:v1", cfg.Hermes.Image)
		assert.Equal(t, "/opt/NONMEM/nm75/run/nmfe75", cfg.Hermes.ContainerCommandPath)
		assert.Equal(t, 4, cfg.Hermes.Resources.CPUCores)
		assert.Equal(t, "8Gi", cfg.Hermes.Resources.Memory)
	})

	t.Run("returns false for new format (no image at root)", func(t *testing.T) {
		newFormatJSON := []byte(`{
			"correlation_strategy": "positional",
			"hermes": {
				"image": "test-image",
				"resources": {"cpu_cores": 2, "memory": "4Gi"}
			}
		}`)

		cfg, migrated, err := migrateFromLegacyFormat(newFormatJSON)

		require.NoError(t, err)
		assert.False(t, migrated)
		assert.Nil(t, cfg)
	})

	t.Run("returns false for config without hermes or image", func(t *testing.T) {
		noHermesJSON := []byte(`{
			"correlation_strategy": "conservative"
		}`)

		cfg, migrated, err := migrateFromLegacyFormat(noHermesJSON)

		require.NoError(t, err)
		assert.False(t, migrated)
		assert.Nil(t, cfg)
	})

	t.Run("rejects empty image value", func(t *testing.T) {
		emptyImageJSON := []byte(`{
			"image": "",
			"resources": {"cpu_cores": 4, "memory": "8Gi"}
		}`)

		cfg, migrated, err := migrateFromLegacyFormat(emptyImageJSON)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "has 'image' key but value is empty")
		assert.False(t, migrated)
		assert.Nil(t, cfg)
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		invalidJSON := []byte(`{not valid json}`)

		cfg, migrated, err := migrateFromLegacyFormat(invalidJSON)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse")
		assert.False(t, migrated)
		assert.Nil(t, cfg)
	})

	t.Run("preserves deprecated nonmem_path field", func(t *testing.T) {
		legacyWithNonmemPath := []byte(`{
			"image": "test-image",
			"nonmem_path": "/opt/old/nmfe74",
			"resources": {"cpu_cores": 2, "memory": "4Gi"}
		}`)

		cfg, migrated, err := migrateFromLegacyFormat(legacyWithNonmemPath)

		require.NoError(t, err)
		assert.True(t, migrated)
		require.NotNil(t, cfg)
		require.NotNil(t, cfg.Hermes)
		assert.Equal(t, "/opt/old/nmfe74", cfg.Hermes.NonmemPath)
	})
}
