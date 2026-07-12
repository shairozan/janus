package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultNonmemInstallationFlagWins(t *testing.T) {
	in := Input{Installations: []NonmemInstall{
		{Name: "nm74", Path: "/opt/nm74", Binary: "nmfe74"},
		{Name: "nm75", Path: "/opt/nm75", Binary: "nmfe75", Default: true},
	}}

	def := in.DefaultNonmemInstallation()
	assert.Equal(t, "nm75", def.Name)
	assert.Equal(t, "/opt/nm75", def.Path)
}

func TestDefaultNonmemInstallationFirstWhenNoFlag(t *testing.T) {
	in := Input{Installations: []NonmemInstall{
		{Name: "nm74", Path: "/opt/nm74", Binary: "nmfe74"},
		{Name: "nm75", Path: "/opt/nm75", Binary: "nmfe75"},
	}}

	assert.Equal(t, "nm74", in.DefaultNonmemInstallation().Name)
}

func TestDefaultNonmemInstallationSyntheticFromLegacy(t *testing.T) {
	in := Input{NonmemPath: "/opt/nm", NonmemBinary: "nmfe76"}

	def := in.DefaultNonmemInstallation()
	assert.Equal(t, "default", def.Name)
	assert.Equal(t, "/opt/nm", def.Path)
	assert.Equal(t, "nmfe76", def.Binary)
}

func TestNonmemInstallationByName(t *testing.T) {
	in := Input{Installations: []NonmemInstall{{Name: "nm75", Path: "/opt/nm75"}}}

	got, ok := in.NonmemInstallation("NM75")
	require.True(t, ok)
	assert.Equal(t, "/opt/nm75", got.Path)

	_, ok = in.NonmemInstallation("missing")
	assert.False(t, ok)
}

func TestNonmemInstallationSyntheticDefault(t *testing.T) {
	in := Input{NonmemPath: "/opt/nm", NonmemBinary: "nmfe76"}

	got, ok := in.NonmemInstallation("default")
	require.True(t, ok)
	assert.Equal(t, "/opt/nm", got.Path)
}

func TestInstallationNames(t *testing.T) {
	assert.Equal(t, []string{"default"}, Input{}.InstallationNames())

	in := Input{Installations: []NonmemInstall{{Name: "a"}, {Name: "b"}}}
	assert.Equal(t, []string{"a", "b"}, in.InstallationNames())
}

func TestNewConfigPopulatesLegacyFromDefaultInstall(t *testing.T) {
	cfg, err := NewConfig(&Input{
		ExecutionMode: ExecutionModeNONMEM,
		Installations: []NonmemInstall{
			{Name: "nm74", Path: "/opt/nm74", Binary: "nmfe74"},
			{Name: "nm75", Path: "/opt/nm75", Binary: "nmfe75", Default: true},
		},
	})
	require.NoError(t, err)

	assert.Equal(t, "/opt/nm75", cfg.NonmemPath)
	assert.Equal(t, "nmfe75", cfg.NonmemBinary)
}

func TestNewConfigKeepsExplicitLegacyPath(t *testing.T) {
	cfg, err := NewConfig(&Input{
		ExecutionMode: ExecutionModeNONMEM,
		NonmemPath:    "/explicit",
		NonmemBinary:  "nmfeX",
		Installations: []NonmemInstall{{Name: "a", Path: "/opt/a", Binary: "nmfeA", Default: true}},
	})
	require.NoError(t, err)

	// An explicit legacy path is not overridden by the installations default.
	assert.Equal(t, "/explicit", cfg.NonmemPath)
	assert.Equal(t, "nmfeX", cfg.NonmemBinary)
}
