package gui

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/shairozan/janus/internal/modelmeta"
	"github.com/shairozan/janus/internal/runlog"
)

// modelExtensions are the file types listed in the model browser.
var modelExtensions = map[string]bool{".mod": true, ".ctl": true}

// browserRow is one model in the browser: its run-derived status and its
// user-authored sidecar (notes/tags/color).
type browserRow struct {
	path    string
	name    string
	status  modelmeta.ModelStatus
	sidecar modelmeta.Sidecar
}

// modelBrowser is the "Models" tab: a per-directory overview of models with
// run status / OFV / color, plus a notes/tags/color editor. Status is derived
// from the run log (the authoritative source via internal/modelmeta); only the
// sidecar is user-authored.
type modelBrowser struct {
	app *App

	dir  string
	rows []browserRow

	dirLabel *widget.Label
	table    *widget.Table

	selected    int // index into rows, or -1
	notesEntry  *widget.Entry
	tagsEntry   *widget.Entry
	colorSelect *widget.Select
	swatch      *canvas.Rectangle
	detailTitle *widget.Label
}

// buildModelBrowserTab constructs the Models tab content.
func (a *App) buildModelBrowserTab() fyne.CanvasObject {
	b := &modelBrowser{app: a, selected: -1}
	a.modelBrowser = b

	b.dirLabel = widget.NewLabel("")
	b.dirLabel.Truncation = fyne.TextTruncateClip

	chooseBtn := widget.NewButton("📁 Choose folder", b.chooseFolder)
	refreshBtn := widget.NewButton("🔄 Refresh", func() { b.refresh() })

	header := container.NewBorder(nil, nil, chooseBtn, refreshBtn, b.dirLabel)

	b.table = b.buildTable()
	detail := b.buildDetail()

	split := container.NewHSplit(b.table, detail)
	split.SetOffset(0.62)

	// Initial directory: the current model's folder, else the default directory.
	b.dir = b.initialDir()
	b.refresh()

	return container.NewBorder(header, nil, nil, nil, split)
}

// initialDir picks the directory to browse on first open.
func (b *modelBrowser) initialDir() string {
	b.app.modelMu.Lock()
	current := b.app.currentFilePath
	b.app.modelMu.Unlock()

	if current != "" {
		return filepath.Dir(current)
	}

	if b.app.config != nil && b.app.config.DefaultDirectory != "" {
		return expandHomeDir(b.app.config.DefaultDirectory)
	}

	return ""
}

// chooseFolder opens a folder picker and refreshes the browser.
func (b *modelBrowser) chooseFolder() {
	dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil || uri == nil {
			return
		}

		b.dir = uri.Path()
		b.selected = -1
		b.refresh()
	}, b.app.window)
}

// refresh rescans the directory, re-deriving status from the run log and
// reloading sidecars, then updates the table.
func (b *modelBrowser) refresh() {
	b.rows = b.scan(b.dir)

	if b.dir == "" {
		b.dirLabel.SetText("(no folder selected — choose a folder)")
	} else {
		b.dirLabel.SetText(fmt.Sprintf("%s  —  %d model(s)", b.dir, len(b.rows)))
	}

	if b.table != nil {
		b.table.Refresh()
	}
}

// scan lists models in dir and joins each with its run-derived status and
// sidecar. It is best-effort: unreadable dirs/run logs yield an empty/partial
// result rather than an error.
func (b *modelBrowser) scan(dir string) []browserRow {
	if dir == "" {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	// One run-log store per directory; records carry the model file name.
	store := runlog.NewRunLogStore(dir, "")
	_ = store.Load() // absent run log → no records
	records := store.GetAllRuns()

	var rows []browserRow

	for _, e := range entries {
		if e.IsDir() || !modelExtensions[strings.ToLower(filepath.Ext(e.Name()))] {
			continue
		}

		path := filepath.Join(dir, e.Name())
		sidecar, _ := modelmeta.LoadSidecar(path)

		rows = append(rows, browserRow{
			path:    path,
			name:    e.Name(),
			status:  modelmeta.DeriveStatusFromRecords(records, e.Name()),
			sidecar: sidecar,
		})
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })

	return rows
}

// browserColumns defines the table columns.
var browserColumns = []struct {
	title string
	width float32
}{
	{"", 30},       // status dot
	{"Model", 200}, // file name
	{"Last run", 150},
	{"Runs", 60},
	{"OFV", 110},
	{"Tags", 180},
}

func (b *modelBrowser) buildTable() *widget.Table {
	t := widget.NewTable(
		func() (int, int) { return len(b.rows), len(browserColumns) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			label, ok := obj.(*widget.Label)
			if !ok || id.Row < 0 || id.Row >= len(b.rows) {
				return
			}

			b.updateCell(label, b.rows[id.Row], id.Col)
		},
	)

	t.ShowHeaderRow = true
	t.CreateHeader = func() fyne.CanvasObject { return widget.NewLabel("") }
	t.UpdateHeader = func(id widget.TableCellID, obj fyne.CanvasObject) {
		if label, ok := obj.(*widget.Label); ok && id.Col >= 0 && id.Col < len(browserColumns) {
			label.SetText(browserColumns[id.Col].title)
			label.TextStyle = fyne.TextStyle{Bold: true}
		}
	}

	for i, c := range browserColumns {
		t.SetColumnWidth(i, c.width)
	}

	t.OnSelected = func(id widget.TableCellID) { b.selectRow(id.Row) }

	return t
}

// updateCell renders one table cell.
func (b *modelBrowser) updateCell(label *widget.Label, row browserRow, col int) {
	label.TextStyle = fyne.TextStyle{}
	label.Importance = widget.MediumImportance

	switch col {
	case 0: // status dot
		label.SetText("●")
		label.Importance = statusImportance(row.status.Status)
	case 1:
		label.SetText(row.name)
	case 2:
		if row.status.Runs > 0 {
			label.SetText(row.status.LastRun.Format("2006-01-02 15:04"))
		} else {
			label.SetText("-")
		}
	case 3:
		label.SetText(fmt.Sprintf("%d", row.status.Runs))
	case 4:
		if row.status.OFV != nil {
			label.SetText(fmt.Sprintf("%.2f", *row.status.OFV))
		} else {
			label.SetText("-")
		}
	case 5:
		label.SetText(strings.Join(row.sidecar.Tags, ", "))
	}
}

func (b *modelBrowser) buildDetail() fyne.CanvasObject {
	b.detailTitle = widget.NewLabelWithStyle("Select a model", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	b.notesEntry = widget.NewMultiLineEntry()
	b.notesEntry.SetPlaceHolder("Notes…")
	b.notesEntry.Wrapping = fyne.TextWrapWord

	b.tagsEntry = widget.NewEntry()
	b.tagsEntry.SetPlaceHolder("comma, separated, tags")

	b.swatch = canvas.NewRectangle(color.Transparent)
	b.swatch.SetMinSize(fyne.NewSize(24, 24))

	b.colorSelect = widget.NewSelect(paletteNames(), func(name string) {
		b.swatch.FillColor = colorForName(name)
		b.swatch.Refresh()
	})

	saveBtn := widget.NewButton("Save", b.saveSelected)
	saveBtn.Importance = widget.HighImportance
	openBtn := widget.NewButton("Open in Model Run", b.openSelected)

	form := container.NewVBox(
		b.detailTitle,
		widget.NewSeparator(),
		widget.NewLabel("Notes:"),
		b.notesEntry,
		widget.NewLabel("Tags:"),
		b.tagsEntry,
		widget.NewLabel("Color:"),
		container.NewHBox(b.colorSelect, b.swatch),
		widget.NewSeparator(),
		container.NewHBox(saveBtn, openBtn),
	)

	return container.NewVScroll(form)
}

// selectRow loads a model's metadata into the detail editor.
func (b *modelBrowser) selectRow(rowIdx int) {
	if rowIdx < 0 || rowIdx >= len(b.rows) {
		return
	}

	b.selected = rowIdx
	row := b.rows[rowIdx]

	b.detailTitle.SetText(row.name)
	b.notesEntry.SetText(row.sidecar.Notes)
	b.tagsEntry.SetText(strings.Join(row.sidecar.Tags, ", "))

	colorName := row.sidecar.Color
	if colorName == "" {
		colorName = paletteNone
	}
	b.colorSelect.SetSelected(colorName)

	b.swatch.FillColor = colorForName(modelmeta.EffectiveColor(row.sidecar, row.status))
	b.swatch.Refresh()
}

// saveSelected writes the edited notes/tags/color to the model's sidecar.
func (b *modelBrowser) saveSelected() {
	if b.selected < 0 || b.selected >= len(b.rows) {
		return
	}

	row := &b.rows[b.selected]

	row.sidecar.Notes = b.notesEntry.Text
	row.sidecar.Tags = parseTags(b.tagsEntry.Text)
	row.sidecar.Color = ""
	if b.colorSelect.Selected != "" && b.colorSelect.Selected != paletteNone {
		row.sidecar.Color = b.colorSelect.Selected
	}

	if err := modelmeta.SaveSidecar(row.path, row.sidecar); err != nil {
		dialog.ShowError(fmt.Errorf("failed to save model metadata: %w", err), b.app.window)

		return
	}

	b.table.Refresh()
}

// openSelected loads the selected model into the Model Run tab.
func (b *modelBrowser) openSelected() {
	if b.selected < 0 || b.selected >= len(b.rows) {
		return
	}

	path := b.rows[b.selected].path

	if err := b.app.loadModelFile(path); err != nil {
		dialog.ShowError(fmt.Errorf("failed to load model file: %w", err), b.app.window)

		return
	}

	if b.app.modelEntry != nil {
		b.app.modelEntry.SetText(path)
	}

	b.app.setModelLoaded(true)

	if b.app.mainTabs != nil {
		b.app.mainTabs.SelectIndex(0) // "Model Run" is the first tab
	}
}

// statusImportance maps a canonical model status to a Fyne widget importance
// (color), matching the convention used by the run-history table.
func statusImportance(status string) widget.Importance {
	switch status {
	case modelmeta.StatusSuccess:
		return widget.SuccessImportance
	case modelmeta.StatusFailed:
		return widget.DangerImportance
	case modelmeta.StatusRunning:
		return widget.WarningImportance
	default:
		return widget.LowImportance
	}
}

// parseTags splits a comma-separated tag string into trimmed, non-empty tags.
func parseTags(s string) []string {
	var tags []string
	for _, t := range strings.Split(s, ",") {
		if trimmed := strings.TrimSpace(t); trimmed != "" {
			tags = append(tags, trimmed)
		}
	}

	return tags
}

// expandHomeDir expands a leading ~ to the user's home directory.
func expandHomeDir(path string) string {
	if strings.HasPrefix(path, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[1:])
		}
	}

	return path
}

// --- color palette (small fixed set; arbitrary RGB is a follow-up) ---

const paletteNone = "(none)"

var palette = []struct {
	name string
	c    color.Color
}{
	{paletteNone, color.Transparent},
	{"blue", color.NRGBA{R: 0x42, G: 0x85, B: 0xf4, A: 0xff}},
	{"green", color.NRGBA{R: 0x34, G: 0xa8, B: 0x53, A: 0xff}},
	{"amber", color.NRGBA{R: 0xfb, G: 0xbc, B: 0x05, A: 0xff}},
	{"red", color.NRGBA{R: 0xea, G: 0x43, B: 0x35, A: 0xff}},
	{"purple", color.NRGBA{R: 0xa1, G: 0x42, B: 0xf4, A: 0xff}},
	{"gray", color.NRGBA{R: 0x9e, G: 0x9e, B: 0x9e, A: 0xff}},
}

// paletteNames returns the selectable color names.
func paletteNames() []string {
	names := make([]string, len(palette))
	for i, p := range palette {
		names[i] = p.name
	}

	return names
}

// colorForName resolves a palette/status color name to a color, transparent
// when unknown.
func colorForName(name string) color.Color {
	for _, p := range palette {
		if p.name == name {
			return p.c
		}
	}

	return color.Transparent
}
