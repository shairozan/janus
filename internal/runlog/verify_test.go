//go:build unit
// +build unit

package runlog

import (
	"encoding/json"
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

// CRITICAL 1: the tip record is the ONE record no prev-hash link commits to —
// it has no successor. head.TipHash is the only thing that does, and it was
// computed, signed, stored, and never read back. Editing the newest sealed
// record left the log cheerfully reporting "chain and tip verified".
func TestTipRecordModificationIsDetected(t *testing.T) {
	store, publicKeyPEM := newSignedStore(t)
	trust, err := NewLicenseTrust(publicKeyPEM, "johnny@example.com")
	require.NoError(t, err)

	records := sealN(t, store, 3)

	tip := records[2]
	require.Equal(t, 3, tip.Sequence, "the record with no successor")

	// Edit a SIGNED field of the newest sealed record on disk.
	tipPath := filepath.Join(store.runlogDir(), tip.ID+".json")

	data, err := os.ReadFile(tipPath)
	require.NoError(t, err)

	var onDisk map[string]any
	require.NoError(t, json.Unmarshal(data, &onDisk))

	onDisk["command"] = "nmfe75 evil.mod evil.lst"
	onDisk["exit_code"] = 0

	edited, err := json.MarshalIndent(onDisk, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(tipPath, edited, 0600))

	// A FRESH store, as a fresh process opening this directory would be — no
	// in-memory cache to paper over the edit.
	fresh := NewRunLogStore(store.baseDir, "model.mod")
	require.NoError(t, fresh.Load())

	report, err := fresh.VerifyIntegrity(trust)
	require.NoError(t, err)

	require.False(t, report.OK, "modifying the newest sealed record must NOT verify as an intact chain")
	require.True(t, report.TipHashMismatch, "only head.TipHash commits to the tip record")
	require.True(t, report.HasIntegrityBreak(), "a modified record is a genuine integrity break")

	require.Equal(t, VerificationChainBroken, report.Records[tip.ID].Status,
		"the tampered tip record must be flagged by ID")

	summary := report.Summary()
	require.NotContains(t, summary, "chain and tip verified",
		"the summary must stop claiming the tip is verified when it is not")
	require.Contains(t, summary, "modified after it was sealed")

	// The untouched records are still intact and must still say so.
	require.Equal(t, VerificationValid, report.Records[records[0].ID].Status)
	require.Equal(t, VerificationValid, report.Records[records[1].ID].Status)
}

// CRITICAL 2: Bob opens a model directory Alice ran in. His trust store holds
// only his own key. NOTHING is wrong with the log — Janus must not say there is,
// and (see integrity_event_test.go) must not write history saying there is.
func TestUntrustedHeadIsNotAnIntegrityBreak(t *testing.T) {
	store, _ := newSignedStore(t)

	// A trust store holding a DIFFERENT key: Bob's, or Alice's own key after a
	// rotation — the two are indistinguishable, and neither is tampering.
	_, otherPublicKey := generateTestKeyPair(t)
	otherTrust, err := NewLicenseTrust(encodePublicKeyPEM(t, otherPublicKey), "bob@example.com")
	require.NoError(t, err)

	sealN(t, store, 3)

	report, err := store.VerifyIntegrity(otherTrust)
	require.NoError(t, err)

	require.False(t, report.OK, "an unverifiable head is genuinely not fully verified")
	require.True(t, report.HeadUntrusted)
	require.False(t, report.HasIntegrityBreak(),
		"an unrecognised signing key is evidence of an incomplete trust store, NOT of tampering")

	summary := report.Summary()
	require.Contains(t, summary, "not one this installation recognises")
	require.Contains(t, summary, "NOT evidence of tampering")
	require.NotContains(t, summary, "RUN LOG INCOMPLETE",
		"a perfectly intact log signed by a colleague must not be described as incomplete")
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

// Finding 4: a nil trust store must never panic VerifyChain, and a head it
// cannot vouch for must never verify as OK — it must degrade to untrusted,
// the same way VerifyRecordStatus already degrades a nil trust store to
// Unverifiable for individual records.
func TestVerifyChainWithNilTrustDoesNotPanicAndDoesNotVerify(t *testing.T) {
	record := &RunRecord{ID: "r1", Sequence: 1, Status: "completed"}

	head := &Head{Sequence: 1, TipHash: "irrelevant-for-this-test"}

	require.NotPanics(t, func() {
		report := VerifyChain([]*RunRecord{record}, head, nil)

		require.False(t, report.OK, "a chain head with no trust anchor to check it against must never verify as OK")
		require.True(t, report.HeadUntrusted, "no trust anchor means the head cannot be vouched for")
	})
}

// Finding 4, no-head variant: nil trust with no head at all (a fresh store) must
// also not panic — VerifyChain returns before ever consulting trust in that
// case, but this pins it so a future refactor can't reintroduce the panic path
// unnoticed.
func TestVerifyChainWithNilTrustAndNoHeadDoesNotPanic(t *testing.T) {
	require.NotPanics(t, func() {
		report := VerifyChain(nil, nil, nil)
		require.True(t, report.OK, "no records and no head is a fresh, untouched log")
	})
}
