//go:build unit
// +build unit

package runlog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
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

// Johnny (or something worse) copies a sealed record's file to a second
// filename on disk instead of deleting one. Deletion is detected by the gap
// check; this is the opposite failure — an INSERTION — and nothing in the
// continuity loop compared record.Sequence == previous.Sequence before this
// fix, so two records claiming the same sequence verified as a perfectly
// intact chain.
func TestDuplicateSequenceIsDetected(t *testing.T) {
	store, publicKeyPEM := newSignedStore(t)
	trust, err := NewLicenseTrust(publicKeyPEM, "johnny@example.com")
	require.NoError(t, err)

	records := sealN(t, store, 3)

	victim := records[1]
	require.Equal(t, 2, victim.Sequence)

	originalPath := filepath.Join(store.runlogDir(), victim.ID+".json")
	data, err := os.ReadFile(originalPath)
	require.NoError(t, err)

	// cp <sequence-2 record>.json <new-uuid>.json — a second file now claims
	// sequence 2.
	duplicatePath := filepath.Join(store.runlogDir(), uuid.Must(uuid.NewV7()).String()+".json")
	require.NoError(t, os.WriteFile(duplicatePath, data, 0600))

	// Force the index to notice the new file, the way a fresh process opening
	// this directory would.
	require.NoError(t, os.Remove(store.indexPath()))
	require.NoError(t, store.Load())

	report, err := store.VerifyIntegrity(trust)
	require.NoError(t, err)

	require.False(t, report.OK, "a duplicated sequence must never verify as an intact chain")
	require.Equal(t, 4, report.Sealed, "reproduces the false '4 sealed records' symptom from only 3 real records")
	require.Len(t, report.Duplicates, 1, "exactly one sequence is duplicated")
	require.Equal(t, 2, report.Duplicates[0].Sequence, "the report must name WHICH sequence collided")
	require.Contains(t, report.Summary(), "2", "the summary must mention the duplicated sequence")
}

// Finding 2: the wording must not lie about which direction the tip moved.
func TestSummaryDescribesTipDirectionCorrectly(t *testing.T) {
	truncated := ChainReport{
		OK:          false,
		Sealed:      3,
		TipMismatch: true,
		ExpectedTip: 5,
		ActualTip:   3,
	}
	require.Contains(t, truncated.Summary(), "the newest record(s) were removed",
		"head ahead of the records present means records were removed")

	notAdvanced := ChainReport{
		OK:          false,
		Sealed:      5,
		TipMismatch: true,
		ExpectedTip: 3,
		ActualTip:   5,
	}
	summary := notAdvanced.Summary()
	require.NotContains(t, summary, "the newest record(s) were removed",
		"records ahead of the head is the OPPOSITE of records being removed")
	require.Contains(t, summary, "the head was not advanced",
		"the summary must describe what actually happened: the head fell behind")
}
