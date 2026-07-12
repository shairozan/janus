package gui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// serializeStrings renders a string list to a stable string for change detection.
func serializeStrings(items []string) string {
	return strings.Join(items, "\n")
}

// serializeRecordValues renders record rows to a stable string for change
// detection (a unit separator avoids collisions with field content).
func serializeRecordValues(vals []recordValue) string {
	var b strings.Builder

	for _, v := range vals {
		b.WriteString(strings.Join(v.Fields, "\x1f"))
		b.WriteString("|")
		if v.Default {
			b.WriteString("1")
		}
		b.WriteString("\n")
	}

	return b.String()
}

// recordColumn describes one text column of a recordListEditor.
type recordColumn struct {
	placeholder string
}

// recordValue is one edited row: the per-column text plus the optional default flag.
type recordValue struct {
	Fields  []string
	Default bool
}

// recordRow holds the widgets for a single editor row.
type recordRow struct {
	cells     []*widget.Entry
	isDefault *widget.Check // nil when the editor has no default column
	container *fyne.Container
}

// recordListEditor is a reusable add/edit/remove list editor for records with a
// fixed set of text columns and an optional single-select "Default" column. It
// generalizes the NONMEM installations editor so the same widget backs remote
// mounts, PsN presets, integration hooks/tools, and scheduler profiles.
type recordListEditor struct {
	columns    []recordColumn
	hasDefault bool
	rows       []*recordRow
	rowsBox    *fyne.Container
	root       *fyne.Container
}

// newRecordListEditor builds an editor with the given add-button label, an
// optional single-select default column, and the supplied text columns.
func newRecordListEditor(addLabel string, hasDefault bool, columns ...recordColumn) *recordListEditor {
	e := &recordListEditor{
		columns:    columns,
		hasDefault: hasDefault,
		rowsBox:    container.NewVBox(),
	}

	addBtn := widget.NewButtonWithIcon(addLabel, theme.ContentAddIcon(), func() {
		e.addRow(recordValue{})
	})

	e.root = container.NewVBox(e.rowsBox, addBtn)

	return e
}

// widget returns the editor's root canvas object.
func (e *recordListEditor) widget() fyne.CanvasObject {
	return e.root
}

// addRow appends an editable row pre-filled from v.
func (e *recordListEditor) addRow(v recordValue) {
	r := &recordRow{cells: make([]*widget.Entry, len(e.columns))}

	cols := make([]fyne.CanvasObject, 0, len(e.columns)+1)
	for i, c := range e.columns {
		entry := widget.NewEntry()
		entry.SetPlaceHolder(c.placeholder)
		if i < len(v.Fields) {
			entry.SetText(v.Fields[i])
		}

		r.cells[i] = entry
		cols = append(cols, entry)
	}

	if e.hasDefault {
		r.isDefault = widget.NewCheck("Default", nil)
		r.isDefault.SetChecked(v.Default)
		// Only one row may be the default.
		r.isDefault.OnChanged = func(checked bool) {
			if checked {
				e.clearOtherDefaults(r)
			}
		}

		cols = append(cols, r.isDefault)
	}

	removeBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() { e.removeRow(r) })

	fields := container.NewGridWithColumns(len(cols), cols...)
	r.container = container.NewBorder(nil, nil, nil, removeBtn, fields)

	e.rows = append(e.rows, r)
	e.rowsBox.Add(r.container)
	e.rowsBox.Refresh()
}

// removeRow drops a row from the editor.
func (e *recordListEditor) removeRow(target *recordRow) {
	for i, r := range e.rows {
		if r == target {
			e.rows = append(e.rows[:i], e.rows[i+1:]...)
			e.rowsBox.Remove(target.container)
			e.rowsBox.Refresh()

			return
		}
	}
}

// clearOtherDefaults unchecks the default box on every row except keep.
func (e *recordListEditor) clearOtherDefaults(keep *recordRow) {
	for _, r := range e.rows {
		if r != keep && r.isDefault != nil && r.isDefault.Checked {
			r.isDefault.SetChecked(false)
		}
	}
}

// values returns the edited rows, skipping rows whose text columns are all blank.
func (e *recordListEditor) values() []recordValue {
	var out []recordValue

	for _, r := range e.rows {
		fields := make([]string, len(r.cells))
		blank := true

		for i, cell := range r.cells {
			fields[i] = strings.TrimSpace(cell.Text)
			if fields[i] != "" {
				blank = false
			}
		}

		if blank {
			continue
		}

		out = append(out, recordValue{
			Fields:  fields,
			Default: r.isDefault != nil && r.isDefault.Checked,
		})
	}

	return out
}

// stringRow holds the widgets for a single-column list row.
type stringRow struct {
	entry     *widget.Entry
	container *fyne.Container
}

// stringListEditor is a reusable add/edit/remove editor for a simple list of
// strings (e.g. cleanup/retain glob patterns).
type stringListEditor struct {
	placeholder string
	rows        []*stringRow
	rowsBox     *fyne.Container
	root        *fyne.Container
}

// newStringListEditor builds a single-column list editor pre-filled with items.
func newStringListEditor(addLabel, placeholder string, items ...string) *stringListEditor {
	e := &stringListEditor{placeholder: placeholder, rowsBox: container.NewVBox()}

	addBtn := widget.NewButtonWithIcon(addLabel, theme.ContentAddIcon(), func() {
		e.addRow("")
	})

	e.root = container.NewVBox(e.rowsBox, addBtn)

	for _, item := range items {
		e.addRow(item)
	}

	return e
}

// widget returns the editor's root canvas object.
func (e *stringListEditor) widget() fyne.CanvasObject {
	return e.root
}

// addRow appends an editable row pre-filled with value.
func (e *stringListEditor) addRow(value string) {
	r := &stringRow{entry: widget.NewEntry()}
	r.entry.SetPlaceHolder(e.placeholder)
	r.entry.SetText(value)

	removeBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() { e.removeRow(r) })
	r.container = container.NewBorder(nil, nil, nil, removeBtn, r.entry)

	e.rows = append(e.rows, r)
	e.rowsBox.Add(r.container)
	e.rowsBox.Refresh()
}

// removeRow drops a row from the editor.
func (e *stringListEditor) removeRow(target *stringRow) {
	for i, r := range e.rows {
		if r == target {
			e.rows = append(e.rows[:i], e.rows[i+1:]...)
			e.rowsBox.Remove(target.container)
			e.rowsBox.Refresh()

			return
		}
	}
}

// items returns the non-blank, trimmed entries.
func (e *stringListEditor) items() []string {
	var out []string

	for _, r := range e.rows {
		if v := strings.TrimSpace(r.entry.Text); v != "" {
			out = append(out, v)
		}
	}

	return out
}
