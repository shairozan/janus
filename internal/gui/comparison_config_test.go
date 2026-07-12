//go:build gui
// +build gui

package gui

import (
	"testing"
	"time"

	"github.com/pharmalytica/janus/internal/runlog"
)

func TestMostRecentModelFile(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("returns the newest record's model file", func(t *testing.T) {
		records := []*runlog.RunRecord{
			{ID: "a", Timestamp: base, ModelFile: "/runs/old/model.mod"},
			{ID: "b", Timestamp: base.Add(2 * time.Hour), ModelFile: "/runs/new/model.mod"},
			{ID: "c", Timestamp: base.Add(time.Hour), ModelFile: "/runs/mid/model.mod"},
		}

		if got := mostRecentModelFile(records); got != "/runs/new/model.mod" {
			t.Errorf("got %q, want /runs/new/model.mod", got)
		}
	})

	t.Run("ignores records without a model file", func(t *testing.T) {
		records := []*runlog.RunRecord{
			{ID: "a", Timestamp: base.Add(3 * time.Hour), ModelFile: ""},
			{ID: "b", Timestamp: base, ModelFile: "/runs/has/model.mod"},
		}

		if got := mostRecentModelFile(records); got != "/runs/has/model.mod" {
			t.Errorf("got %q, want /runs/has/model.mod", got)
		}
	})

	t.Run("returns empty when no record has a model file", func(t *testing.T) {
		records := []*runlog.RunRecord{
			{ID: "a", Timestamp: base},
			nil,
		}

		if got := mostRecentModelFile(records); got != "" {
			t.Errorf("got %q, want empty string", got)
		}
	})
}
