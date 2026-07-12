package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultNONMEMRetain(t *testing.T) {
	got := DefaultNONMEMRetain()

	// The original core set plus the enriched table/diagnostic globs.
	for _, want := range []string{
		"*.lst", "*.ext", "*.xml", "*.phi", "*.cov", "*.cor", "*.coi", "*.shk",
		"*.shm", "*.grd", "*.cpu",
		"sdtab*", "patab*", "cotab*", "catab*",
	} {
		assert.Contains(t, got, want, "missing default retain glob %q", want)
	}
}

func TestDefaultPSNRetain(t *testing.T) {
	assert.Equal(t, []string{"psn_janus/**"}, DefaultPSNRetain())
}

func TestDefaultRetainReturnsFreshSlice(t *testing.T) {
	// Callers may mutate/append; each call must return an independent backing
	// array so the canonical value can't be corrupted.
	a := DefaultNONMEMRetain()
	b := DefaultNONMEMRetain()
	a[0] = "mutated"
	assert.Equal(t, "*.lst", b[0], "second call must be unaffected by mutating the first")
}
