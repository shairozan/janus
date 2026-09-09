// Package modelmeta surfaces per-model status, coloring, and user notes/tags —
// Janus's answer to Pirana's per-directory pirana.dir metadata.
//
// Run status and a default color are DERIVED from the run log (the
// authoritative source, internal/runlog); only user-authored notes, tags, and
// an optional color override are persisted, in a cohabiting .janus.model.json
// sidecar next to the model. This keeps a single source of truth for run state.
package modelmeta

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shairozan/janus/internal/runlog"
)

// sidecarSuffix is appended to a model path to form its metadata sidecar.
const sidecarSuffix = ".janus.model.json"

// Run status values.
const (
	StatusNone    = "none"
	StatusSuccess = "success"
	StatusFailed  = "failed"
	StatusRunning = "running"
)

// Colors derived from status.
const (
	ColorGray  = "gray"
	ColorGreen = "green"
	ColorRed   = "red"
	ColorAmber = "amber"
)

// Sidecar is the user-authored metadata persisted next to a model.
type Sidecar struct {
	Notes string   `json:"notes,omitempty"`
	Tags  []string `json:"tags,omitempty"`
	// Color optionally overrides the run-derived color (user coloring).
	Color string `json:"color,omitempty"`
}

// ModelStatus is the run-derived status for a model.
type ModelStatus struct {
	Status   string
	Color    string
	Runs     int
	LastRun  time.Time
	ExitCode int
	// OFV is the objective function value of the latest run, when available.
	OFV *float64
}

// SidecarPath returns the metadata sidecar path for a model.
func SidecarPath(modelPath string) string {
	return modelPath + sidecarSuffix
}

// LoadSidecar reads a model's sidecar, returning the zero value when absent.
func LoadSidecar(modelPath string) (Sidecar, error) {
	data, err := os.ReadFile(SidecarPath(modelPath))
	if err != nil {
		if os.IsNotExist(err) {
			return Sidecar{}, nil
		}

		return Sidecar{}, err
	}

	var s Sidecar
	if err := json.Unmarshal(data, &s); err != nil {
		return Sidecar{}, err
	}

	return s, nil
}

// SaveSidecar writes a model's sidecar.
func SaveSidecar(modelPath string, s Sidecar) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(SidecarPath(modelPath), data, 0o600)
}

// DeriveStatus computes a model's status from run-log entries: the most recent
// run referencing the model determines success/failure (and color); the number
// of matching runs is also reported. A model with no runs is StatusNone.
func DeriveStatus(entries []runlog.RunEntry, modelName string) ModelStatus {
	stem := stemOf(modelName)

	var (
		latest *runlog.RunEntry
		count  int
	)

	for i := range entries {
		if !entryMatchesModel(entries[i], stem) {
			continue
		}

		count++

		if latest == nil || entries[i].Timestamp.After(latest.Timestamp) {
			latest = &entries[i]
		}
	}

	if latest == nil {
		return ModelStatus{Status: StatusNone, Color: ColorGray}
	}

	status := StatusSuccess
	if latest.ExitCode != 0 {
		status = StatusFailed
	}

	return ModelStatus{
		Status:   status,
		Color:    StatusColor(status),
		Runs:     count,
		LastRun:  latest.Timestamp,
		ExitCode: latest.ExitCode,
	}
}

// DeriveStatusFromRecords computes a model's status from the GUI's run-log
// records (runlog.RunLogStore → []runlog.RunRecord), which carry an explicit
// ModelFile, Status, and typed OFV. The most recent run referencing the model
// (matched by file stem) determines the status, color, and OFV; the number of
// matching runs is also reported. A model with no runs is StatusNone.
//
// This is the GUI-side counterpart to DeriveStatus (which works off the
// file-based RunEntry stream); both reuse the same Sidecar and color helpers.
func DeriveStatusFromRecords(records []runlog.RunRecord, modelName string) ModelStatus {
	stem := stemOf(modelName)

	var (
		latest *runlog.RunRecord
		count  int
	)

	for i := range records {
		if stemOf(filepath.Base(records[i].ModelFile)) != stem {
			continue
		}

		count++

		if latest == nil || records[i].Timestamp.After(latest.Timestamp) {
			latest = &records[i]
		}
	}

	if latest == nil {
		return ModelStatus{Status: StatusNone, Color: ColorGray}
	}

	status := statusFromRecord(latest)

	ms := ModelStatus{
		Status:   status,
		Color:    StatusColor(status),
		Runs:     count,
		LastRun:  latest.Timestamp,
		ExitCode: latest.ExitCode,
	}

	if latest.Summary != nil {
		ms.OFV = latest.Summary.GoodnessOfFit.ObjectiveFunctionValue
	}

	return ms
}

// statusFromRecord maps a RunRecord's status string onto a canonical status,
// falling back to the exit code when the status field is absent.
func statusFromRecord(r *runlog.RunRecord) string {
	switch strings.ToLower(strings.TrimSpace(r.Status)) {
	case "completed":
		return StatusSuccess
	case "failed":
		return StatusFailed
	case "running":
		return StatusRunning
	}

	if r.ExitCode == 0 {
		return StatusSuccess
	}

	return StatusFailed
}

// StatusColor maps a run status to its default color.
func StatusColor(status string) string {
	switch status {
	case StatusSuccess:
		return ColorGreen
	case StatusFailed:
		return ColorRed
	case StatusRunning:
		return ColorAmber
	default:
		return ColorGray
	}
}

// EffectiveColor returns the sidecar's color override when set, else the
// run-derived color.
func EffectiveColor(sidecar Sidecar, status ModelStatus) string {
	if sidecar.Color != "" {
		return sidecar.Color
	}

	return status.Color
}

// entryMatchesModel reports whether a run-log entry references the model, by
// matching the file stem of any of its arguments (so run1.mod and run1.lst both
// associate with the "run1" model).
func entryMatchesModel(e runlog.RunEntry, stem string) bool {
	for _, a := range e.Arguments {
		if stemOf(filepath.Base(a)) == stem {
			return true
		}
	}

	return false
}

// stemOf returns a file name without its extension.
func stemOf(name string) string {
	return strings.TrimSuffix(name, filepath.Ext(name))
}
