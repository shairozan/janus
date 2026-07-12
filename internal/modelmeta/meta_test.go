package modelmeta

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/model"
	"github.com/pharmalytica/janus/internal/runlog"
)

func TestSidecarRoundTrip(t *testing.T) {
	modelPath := filepath.Join(t.TempDir(), "run1.mod")

	// Absent sidecar → zero value, no error.
	got, err := LoadSidecar(modelPath)
	require.NoError(t, err)
	assert.Equal(t, Sidecar{}, got)

	in := Sidecar{Notes: "final model", Tags: []string{"base", "covariate"}, Color: "blue"}
	require.NoError(t, SaveSidecar(modelPath, in))

	got, err = LoadSidecar(modelPath)
	require.NoError(t, err)
	assert.Equal(t, in, got)

	// The sidecar cohabits the model directory.
	assert.FileExists(t, modelPath+".janus.model.json")
}

func entry(model string, exitCode int, ts time.Time) runlog.RunEntry {
	return runlog.RunEntry{Arguments: []string{model}, ExitCode: exitCode, Timestamp: ts}
}

func TestDeriveStatusNoRuns(t *testing.T) {
	st := DeriveStatus(nil, "run1.mod")
	assert.Equal(t, StatusNone, st.Status)
	assert.Equal(t, ColorGray, st.Color)
	assert.Equal(t, 0, st.Runs)
}

func TestDeriveStatusSuccessAndFailure(t *testing.T) {
	base := time.Date(2026, 6, 26, 10, 0, 0, 0, time.UTC)

	ok := DeriveStatus([]runlog.RunEntry{entry("run1.mod", 0, base)}, "run1.mod")
	assert.Equal(t, StatusSuccess, ok.Status)
	assert.Equal(t, ColorGreen, ok.Color)

	bad := DeriveStatus([]runlog.RunEntry{entry("run1.mod", 1, base)}, "run1.mod")
	assert.Equal(t, StatusFailed, bad.Status)
	assert.Equal(t, ColorRed, bad.Color)
}

func TestDeriveStatusLatestRunWins(t *testing.T) {
	base := time.Date(2026, 6, 26, 10, 0, 0, 0, time.UTC)

	entries := []runlog.RunEntry{
		entry("run1.mod", 1, base),                  // earlier: failed
		entry("run1.mod", 0, base.Add(time.Hour)),   // latest: success
		entry("run1.lst", 1, base.Add(time.Minute)), // same model via .lst, middle
	}

	st := DeriveStatus(entries, "run1.mod")
	assert.Equal(t, StatusSuccess, st.Status)
	assert.Equal(t, 3, st.Runs) // all three reference the run1 model
	assert.Equal(t, base.Add(time.Hour), st.LastRun)
}

func TestDeriveStatusIgnoresOtherModels(t *testing.T) {
	base := time.Date(2026, 6, 26, 10, 0, 0, 0, time.UTC)

	entries := []runlog.RunEntry{
		entry("other.mod", 0, base),
		entry("run1.mod", 1, base),
	}

	st := DeriveStatus(entries, "run1.mod")
	assert.Equal(t, 1, st.Runs)
	assert.Equal(t, StatusFailed, st.Status)
}

func record(modelFile, status string, ofv *float64, ts time.Time) runlog.RunRecord {
	r := runlog.RunRecord{ModelFile: modelFile, Status: status, Timestamp: ts}
	if ofv != nil {
		r.Summary = &model.ModelSummary{
			GoodnessOfFit: model.GoodnessOfFitSummary{ObjectiveFunctionValue: ofv},
		}
	}

	return r
}

func TestDeriveStatusFromRecordsNoRuns(t *testing.T) {
	st := DeriveStatusFromRecords(nil, "run1.mod")
	assert.Equal(t, StatusNone, st.Status)
	assert.Equal(t, ColorGray, st.Color)
	assert.Nil(t, st.OFV)
}

func TestDeriveStatusFromRecordsLatestWinsWithOFV(t *testing.T) {
	base := time.Date(2026, 6, 26, 10, 0, 0, 0, time.UTC)
	ofv := -1234.5

	records := []runlog.RunRecord{
		record("/m/run1.mod", "failed", nil, base),
		record("/m/run1.mod", "completed", &ofv, base.Add(time.Hour)), // latest
		record("/m/run1.lst", "running", nil, base.Add(time.Minute)),  // same model via .lst
		record("/m/other.mod", "completed", nil, base.Add(2*time.Hour)),
	}

	st := DeriveStatusFromRecords(records, "run1.mod")
	assert.Equal(t, StatusSuccess, st.Status)
	assert.Equal(t, ColorGreen, st.Color)
	assert.Equal(t, 3, st.Runs) // .mod + .mod + .lst, not other.mod
	require.NotNil(t, st.OFV)
	assert.InDelta(t, -1234.5, *st.OFV, 1e-9)
}

func TestDeriveStatusFromRecordsRunningAndFallback(t *testing.T) {
	base := time.Date(2026, 6, 26, 10, 0, 0, 0, time.UTC)

	running := DeriveStatusFromRecords([]runlog.RunRecord{record("run1.mod", "running", nil, base)}, "run1.mod")
	assert.Equal(t, StatusRunning, running.Status)
	assert.Equal(t, ColorAmber, running.Color)

	// Missing status string falls back to exit code.
	rec := runlog.RunRecord{ModelFile: "run1.mod", Status: "", ExitCode: 1, Timestamp: base}
	failed := DeriveStatusFromRecords([]runlog.RunRecord{rec}, "run1.mod")
	assert.Equal(t, StatusFailed, failed.Status)
}

func TestEffectiveColorOverride(t *testing.T) {
	status := ModelStatus{Status: StatusSuccess, Color: ColorGreen}

	// No override → derived color.
	assert.Equal(t, ColorGreen, EffectiveColor(Sidecar{}, status))
	// Override wins.
	assert.Equal(t, "purple", EffectiveColor(Sidecar{Color: "purple"}, status))
}

func TestLoadSidecarRejectsBadJSON(t *testing.T) {
	modelPath := filepath.Join(t.TempDir(), "run1.mod")
	require.NoError(t, os.WriteFile(SidecarPath(modelPath), []byte("{not json"), 0o600))

	_, err := LoadSidecar(modelPath)
	assert.Error(t, err)
}
