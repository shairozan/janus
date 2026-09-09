package gui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shairozan/janus/internal/config"
)

func TestInstallationsEditorRoundTrip(t *testing.T) {
	in := []config.NonmemInstall{
		{Name: "nm74", Path: "/opt/nm74", Binary: "nmfe74"},
		{Name: "nm75", Path: "/opt/nm75", Binary: "nmfe75", Default: true},
	}

	e := newInstallationsEditor(in)
	assert.Equal(t, in, e.installations())
}

func TestInstallationsEditorSkipsBlankRows(t *testing.T) {
	e := newInstallationsEditor(nil)
	e.addRow(config.NonmemInstall{}) // blank — should be skipped

	e.addRow(config.NonmemInstall{Name: "nm76", Path: "/opt/nm76", Binary: "nmfe76"})

	out := e.installations()
	require.Len(t, out, 1)
	assert.Equal(t, "nm76", out[0].Name)
}

func TestInstallationsEditorSingleDefault(t *testing.T) {
	e := newInstallationsEditor([]config.NonmemInstall{
		{Name: "a", Path: "/a", Default: true},
		{Name: "b", Path: "/b"},
	})

	// Marking the second default clears the first.
	e.rows[1].isDefault.SetChecked(true)

	out := e.installations()
	assert.False(t, out[0].Default)
	assert.True(t, out[1].Default)
}

func TestInstallationsEditorRemove(t *testing.T) {
	e := newInstallationsEditor([]config.NonmemInstall{
		{Name: "a", Path: "/a"},
		{Name: "b", Path: "/b"},
	})

	e.removeRow(e.rows[0])

	out := e.installations()
	require.Len(t, out, 1)
	assert.Equal(t, "b", out[0].Name)
}

func TestSerializeInstallsDetectsChanges(t *testing.T) {
	a := []config.NonmemInstall{{Name: "nm75", Path: "/opt/nm75", Default: true}}
	b := []config.NonmemInstall{{Name: "nm75", Path: "/opt/nm75", Default: false}}

	assert.Equal(t, serializeInstalls(a), serializeInstalls(a))
	assert.NotEqual(t, serializeInstalls(a), serializeInstalls(b))
	assert.Equal(t, "", serializeInstalls(nil))
}
