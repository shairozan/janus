package execution

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pharmalytica/janus/internal/config"
	"github.com/pharmalytica/janus/internal/execution/category"
)

// TestResolveRetainPatterns exercises the four-tier precedence:
// PsN-tool mode > per-model retain > global retain > category default.
func TestResolveRetainPatterns(t *testing.T) {
	nonmem := category.NewNONMEMCategory()

	globalCfg := func(retain ...string) *config.Config {
		c := &config.Config{}
		c.Hermes.Retain = retain

		return c
	}

	tests := []struct {
		name string
		exec *HermesExecutor
		want []string
	}{
		{
			name: "PsN mode wins over per-model and global",
			exec: &HermesExecutor{
				psnFunction:   "vpc",
				retain:        []string{"*.custom"},
				config:        globalCfg("*.global"),
				modelCategory: nonmem,
			},
			want: []string{"psn_janus/**"},
		},
		{
			name: "per-model is authoritative over global",
			exec: &HermesExecutor{
				retain:        []string{"*.lst", "sdtab*"},
				config:        globalCfg("*.global"),
				modelCategory: nonmem,
			},
			want: []string{"*.lst", "sdtab*"},
		},
		{
			name: "global used when per-model empty",
			exec: &HermesExecutor{
				config:        globalCfg("*.global"),
				modelCategory: nonmem,
			},
			want: []string{"*.global"},
		},
		{
			name: "category default when per-model and global empty",
			exec: &HermesExecutor{
				config:        globalCfg(),
				modelCategory: nonmem,
			},
			want: config.DefaultNONMEMRetain(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.exec.resolveRetainPatterns())
		})
	}
}
