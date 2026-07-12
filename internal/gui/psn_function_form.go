package gui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// psnFunctionParams holds the typed inputs for a PsN analysis.
type psnFunctionParams struct {
	samples    string // bootstrap, vpc
	stratifyOn string // bootstrap, vpc
	dofv       bool   // bootstrap
	idv        string // vpc
	autoBin    bool   // vpc
	configFile string // scm
	direction  string // scm
}

// buildPSNFunctionArgs builds the PsN CLI args for a function from typed values.
// Empty values are omitted so PsN defaults apply. Pure and unit-tested.
func buildPSNFunctionArgs(fn string, p psnFunctionParams) []string {
	var args []string

	switch fn {
	case "bootstrap":
		if p.samples != "" {
			args = append(args, "-samples="+p.samples)
		}
		if p.stratifyOn != "" {
			args = append(args, "-stratify_on="+p.stratifyOn)
		}
		if p.dofv {
			args = append(args, "-dofv")
		}
	case "vpc":
		if p.samples != "" {
			args = append(args, "-samples="+p.samples)
		}
		if p.idv != "" {
			args = append(args, "-idv="+p.idv)
		}
		if p.stratifyOn != "" {
			args = append(args, "-stratify_on="+p.stratifyOn)
		}
		if p.autoBin {
			args = append(args, "-auto_bin=auto")
		}
	case "scm":
		if p.configFile != "" {
			args = append(args, "-config_file="+p.configFile)
		}
		if p.direction != "" {
			args = append(args, "-search_direction="+p.direction)
		}
	}

	return args
}

// psnFunctionForm holds the per-function parameter controls, swapping the visible
// card to match the selected analysis.
type psnFunctionForm struct {
	root *fyne.Container

	// onChanged, when set, is invoked whenever a parameter control changes, so
	// the caller can refresh anything derived from the form (e.g. the command
	// preview).
	onChanged func()

	bootSamples  *widget.Entry
	bootStratify *widget.Entry
	bootDofv     *widget.Check

	vpcSamples  *widget.Entry
	vpcIDV      *widget.Entry
	vpcStratify *widget.Entry
	vpcAutoBin  *widget.Check

	scmConfig    *widget.Entry
	scmDirection *widget.Select

	bootCard fyne.CanvasObject
	vpcCard  fyne.CanvasObject
	scmCard  fyne.CanvasObject
}

// newPSNFunctionForm builds the (initially hidden) per-function forms.
func newPSNFunctionForm() *psnFunctionForm {
	f := &psnFunctionForm{
		bootSamples:  numberedEntry("number of samples (e.g. 200)"),
		bootStratify: widget.NewEntry(),
		bootDofv:     widget.NewCheck("Include dOFV", nil),
		vpcSamples:   numberedEntry("number of samples (e.g. 500)"),
		vpcIDV:       widget.NewEntry(),
		vpcStratify:  widget.NewEntry(),
		vpcAutoBin:   widget.NewCheck("Auto-bin", nil),
		scmConfig:    widget.NewEntry(),
		scmDirection: widget.NewSelect([]string{"", "forward", "backward", "both"}, nil),
	}

	f.bootStratify.SetPlaceHolder("stratify on (column, optional)")
	f.vpcIDV.SetPlaceHolder("independent variable (e.g. TIME)")
	f.vpcStratify.SetPlaceHolder("stratify on (column, optional)")
	f.scmConfig.SetPlaceHolder("path to scm config file (required)")

	// Notify on any parameter change so the command preview can update live.
	for _, e := range []*widget.Entry{
		f.bootSamples, f.bootStratify, f.vpcSamples, f.vpcIDV, f.vpcStratify, f.scmConfig,
	} {
		e.OnChanged = func(string) { f.notify() }
	}
	f.bootDofv.OnChanged = func(bool) { f.notify() }
	f.vpcAutoBin.OnChanged = func(bool) { f.notify() }
	f.scmDirection.OnChanged = func(string) { f.notify() }

	f.bootCard = widget.NewCard("Bootstrap", "", container.NewVBox(
		widget.NewLabel("Samples:"), f.bootSamples,
		widget.NewLabel("Stratify on:"), f.bootStratify,
		f.bootDofv,
	))
	f.vpcCard = widget.NewCard("VPC", "", container.NewVBox(
		widget.NewLabel("Samples:"), f.vpcSamples,
		widget.NewLabel("IDV:"), f.vpcIDV,
		widget.NewLabel("Stratify on:"), f.vpcStratify,
		f.vpcAutoBin,
	))
	f.scmCard = widget.NewCard("SCM (covariate search)", "", container.NewVBox(
		widget.NewLabel("Config file:"), f.scmConfig,
		widget.NewLabel("Search direction:"), f.scmDirection,
	))

	f.root = container.NewVBox(f.bootCard, f.vpcCard, f.scmCard)
	f.show("")

	return f
}

// numberedEntry is a plain entry with a placeholder (kept simple; PsN validates
// the value itself).
func numberedEntry(placeholder string) *widget.Entry {
	e := widget.NewEntry()
	e.SetPlaceHolder(placeholder)

	return e
}

// widget returns the form's root object.
func (f *psnFunctionForm) widget() fyne.CanvasObject {
	return f.root
}

// notify invokes the onChanged callback if one is set.
func (f *psnFunctionForm) notify() {
	if f.onChanged != nil {
		f.onChanged()
	}
}

// show reveals the card for fn (bootstrap/vpc/scm) and hides the rest. For any
// other value the whole form is hidden.
func (f *psnFunctionForm) show(fn string) {
	f.bootCard.Hide()
	f.vpcCard.Hide()
	f.scmCard.Hide()

	switch fn {
	case "bootstrap":
		f.bootCard.Show()
		f.root.Show()
	case "vpc":
		f.vpcCard.Show()
		f.root.Show()
	case "scm":
		f.scmCard.Show()
		f.root.Show()
	default:
		f.root.Hide()
	}

	f.root.Refresh()
}

// args returns the PsN CLI args for fn from the current form values.
func (f *psnFunctionForm) args(fn string) []string {
	switch fn {
	case "bootstrap":
		return buildPSNFunctionArgs(fn, psnFunctionParams{
			samples:    strings.TrimSpace(f.bootSamples.Text),
			stratifyOn: strings.TrimSpace(f.bootStratify.Text),
			dofv:       f.bootDofv.Checked,
		})
	case "vpc":
		return buildPSNFunctionArgs(fn, psnFunctionParams{
			samples:    strings.TrimSpace(f.vpcSamples.Text),
			idv:        strings.TrimSpace(f.vpcIDV.Text),
			stratifyOn: strings.TrimSpace(f.vpcStratify.Text),
			autoBin:    f.vpcAutoBin.Checked,
		})
	case "scm":
		return buildPSNFunctionArgs(fn, psnFunctionParams{
			configFile: strings.TrimSpace(f.scmConfig.Text),
			direction:  f.scmDirection.Selected,
		})
	default:
		return nil
	}
}

// scmConfigMissing reports whether scm is selected without a config file (scm
// cannot run without one).
func (f *psnFunctionForm) scmConfigMissing(fn string) bool {
	return fn == "scm" && strings.TrimSpace(f.scmConfig.Text) == ""
}
