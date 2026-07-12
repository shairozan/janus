package gui

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"

	"github.com/pharmalytica/janus/internal/modelmeta"
)

func TestStatusImportance(t *testing.T) {
	assert.Equal(t, widget.SuccessImportance, statusImportance(modelmeta.StatusSuccess))
	assert.Equal(t, widget.DangerImportance, statusImportance(modelmeta.StatusFailed))
	assert.Equal(t, widget.WarningImportance, statusImportance(modelmeta.StatusRunning))
	assert.Equal(t, widget.LowImportance, statusImportance(modelmeta.StatusNone))
	assert.Equal(t, widget.LowImportance, statusImportance("anything-else"))
}

func TestParseTags(t *testing.T) {
	assert.Equal(t, []string{"base", "covariate", "final"}, parseTags("base, covariate ,, final"))
	assert.Nil(t, parseTags(""))
	assert.Nil(t, parseTags("  ,  , "))
}

func TestPalette(t *testing.T) {
	names := paletteNames()
	assert.Equal(t, paletteNone, names[0])
	assert.Contains(t, names, "blue")

	// Known names resolve to a concrete color; unknown → transparent.
	assert.NotEqual(t, color.Transparent, colorForName("blue"))
	assert.Equal(t, color.Transparent, colorForName("not-a-color"))
	assert.Equal(t, color.Transparent, colorForName(paletteNone))
}

func TestExpandHomeDir(t *testing.T) {
	// A non-~ path is returned unchanged.
	assert.Equal(t, "/opt/models", expandHomeDir("/opt/models"))
}
