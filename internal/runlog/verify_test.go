//go:build unit
// +build unit

package runlog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// sealN adds n completed runs to a signed store and returns them in chain order.
func sealN(t *testing.T, store *RunLogStore, n int) []*RunRecord {
	t.Helper()

	records := make([]*RunRecord, 0, n)

	for i := 0; i < n; i++ {
		record := &RunRecord{ModelFile: "model.mod", Status: "completed"}
		require.NoError(t, store.AddRun(record))
		records = append(records, record)
	}

	return records
}

func TestIntactChainVerifies(t *testing.T) {
	store, publicKeyPEM := newSignedStore(t)
	trust, err := NewLicenseTrust(publicKeyPEM, "johnny@example.com")
	require.NoError(t, err)

	sealN(t, store, 5)

	report, err := store.VerifyIntegrity(trust)
	require.NoError(t, err)

	require.True(t, report.OK, "an untouched chain must verify: %s", report.Summary())
	require.Equal(t, 5, report.Sealed)
	require.Empty(t, report.Gaps)
	require.False(t, report.TipMismatch)
}

// Johnny accidentally deletes a run in the middle of the log.
func TestMiddleDeletionIsDetectedAndLocated(t *testing.T) {
	store, publicKeyPEM := newSignedStore(t)
	trust, err := NewLicenseTrust(publicKeyPEM, "johnny@example.com")
	require.NoError(t, err)

	records := sealN(t, store, 5)

	// rm .janus/runlog/<id>.json  — record at sequence 3
	victim := records[2]
	require.Equal(t, 3, victim.Sequence)
	require.NoError(t, os.Remove(filepath.Join(store.runlogDir(), victim.ID+".json")))

	report, err := store.VerifyIntegrity(trust)
	require.NoError(t, err)

	require.False(t, report.OK, "a deleted record must break the chain")
	require.Len(t, report.Gaps, 1, "exactly one gap")
	require.Equal(t, 3, report.Gaps[0].MissingSequence, "the report must name WHICH record is gone")

	// The surviving records are intact and must still say so. Marking them red
	// would be a lie, and a lie that teaches users to ignore red.
	require.Equal(t, 4, report.Sealed)
	require.Equal(t, VerificationValid, report.Records[records[0].ID].Status)
	require.Equal(t, VerificationValid, report.Records[records[4].ID].Status)

	// The tip is untouched, so head still agrees.
	require.False(t, report.TipMismatch)
}

// Johnny deletes the newest runs. A pure hash chain cannot see this — the head can.
func TestTailTruncationIsDetectedByTheHead(t *testing.T) {
	store, publicKeyPEM := newSignedStore(t)
	trust, err := NewLicenseTrust(publicKeyPEM, "johnny@example.com")
	require.NoError(t, err)

	records := sealN(t, store, 5)

	for _, victim := range records[3:] { // sequences 4 and 5
		require.NoError(t, os.Remove(filepath.Join(store.runlogDir(), victim.ID+".json")))
	}

	report, err := store.VerifyIntegrity(trust)
	require.NoError(t, err)

	require.False(t, report.OK, "truncation must be detected")
	require.True(t, report.TipMismatch, "the surviving chain is self-consistent; only the head reveals the loss")
	require.Equal(t, 5, report.ExpectedTip)
	require.Equal(t, 3, report.ActualTip)
}

func TestSummaryNamesTheMissingSequence(t *testing.T) {
	report := ChainReport{
		OK:     false,
		Sealed: 4,
		Gaps:   []ChainGap{{AfterSequence: 2, MissingSequence: 3}},
	}

	require.Contains(t, report.Summary(), "3", "the summary must tell the user WHICH record is missing")
}
