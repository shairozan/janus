package gui

import (
	"fmt"
	"strings"

	"github.com/shairozan/janus/internal/config"
)

// serializeInstalls renders an installations list to a stable string for
// change detection.
func serializeInstalls(installs []config.NonmemInstall) string {
	var b strings.Builder

	for _, in := range installs {
		fmt.Fprintf(&b, "%s|%s|%s|%t\n", in.Name, in.Path, in.Binary, in.Default)
	}

	return b.String()
}

// installationsEditor is an add/edit/remove list editor for NONMEM
// installations, backed by the reusable recordListEditor. The single
// nonmem-path/binary fields remain the implicit default when no installations
// are configured (backward compatible); when installations exist, the one
// marked default drives them.
type installationsEditor struct {
	*recordListEditor
}

// newInstallationsEditor builds an editor pre-populated with installs.
func newInstallationsEditor(installs []config.NonmemInstall) *installationsEditor {
	e := &installationsEditor{newRecordListEditor("Add installation", true,
		recordColumn{placeholder: "name"},
		recordColumn{placeholder: "path"},
		recordColumn{placeholder: "binary (e.g. nmfe75)"},
	)}

	for _, in := range installs {
		e.addRow(in)
	}

	return e
}

// addRow appends an editable row for an installation.
func (e *installationsEditor) addRow(in config.NonmemInstall) {
	e.recordListEditor.addRow(recordValue{
		Fields:  []string{in.Name, in.Path, in.Binary},
		Default: in.Default,
	})
}

// installations returns the edited list, skipping rows with no name and no path.
func (e *installationsEditor) installations() []config.NonmemInstall {
	var out []config.NonmemInstall

	for _, v := range e.values() {
		name, path, binary := v.Fields[0], v.Fields[1], v.Fields[2]
		if name == "" && path == "" {
			continue
		}

		out = append(out, config.NonmemInstall{
			Name:    name,
			Path:    path,
			Binary:  binary,
			Default: v.Default,
		})
	}

	return out
}
