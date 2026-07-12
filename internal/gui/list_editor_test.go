package gui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordListEditorRoundTrip(t *testing.T) {
	e := newRecordListEditor("Add", false,
		recordColumn{placeholder: "local"},
		recordColumn{placeholder: "remote"},
	)
	require.NotNil(t, e.widget())

	e.addRow(recordValue{Fields: []string{"C:\\models", "/home/jane/models"}})
	e.addRow(recordValue{Fields: []string{"D:\\data", "/data"}})

	got := e.values()
	require.Len(t, got, 2)
	assert.Equal(t, []string{"C:\\models", "/home/jane/models"}, got[0].Fields)
	assert.Equal(t, []string{"D:\\data", "/data"}, got[1].Fields)
}

func TestRecordListEditorSkipsBlankRows(t *testing.T) {
	e := newRecordListEditor("Add", false, recordColumn{placeholder: "a"}, recordColumn{placeholder: "b"})

	e.addRow(recordValue{})                             // fully blank → skipped
	e.addRow(recordValue{Fields: []string{"x", ""}})    // partial → kept
	e.addRow(recordValue{Fields: []string{"  ", "  "}}) // whitespace-only → skipped

	got := e.values()
	require.Len(t, got, 1)
	assert.Equal(t, "x", got[0].Fields[0])
}

func TestRecordListEditorSingleDefault(t *testing.T) {
	e := newRecordListEditor("Add", true, recordColumn{placeholder: "name"})
	e.addRow(recordValue{Fields: []string{"a"}, Default: true})
	e.addRow(recordValue{Fields: []string{"b"}})

	// Checking the second default clears the first.
	e.rows[1].isDefault.SetChecked(true)

	got := e.values()
	require.Len(t, got, 2)
	assert.False(t, got[0].Default)
	assert.True(t, got[1].Default)
}

func TestRecordListEditorRemove(t *testing.T) {
	e := newRecordListEditor("Add", false, recordColumn{placeholder: "name"})
	e.addRow(recordValue{Fields: []string{"a"}})
	e.addRow(recordValue{Fields: []string{"b"}})

	e.removeRow(e.rows[0])

	got := e.values()
	require.Len(t, got, 1)
	assert.Equal(t, "b", got[0].Fields[0])
}

func TestStringListEditor(t *testing.T) {
	e := newStringListEditor("Add glob", "e.g. FDATA", "FDATA", "FCON", "")
	require.NotNil(t, e.widget())

	// Blank seed entry is dropped; the two real ones remain.
	assert.Equal(t, []string{"FDATA", "FCON"}, e.items())

	e.addRow("FSTREAM")
	assert.Equal(t, []string{"FDATA", "FCON", "FSTREAM"}, e.items())

	e.removeRow(e.rows[0])
	assert.Equal(t, []string{"FCON", "FSTREAM"}, e.items())
}
