//go:build unit
// +build unit

package runlog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDetectedGapIsRecordedIntoTheChain(t *testing.T) {
	store, publicKeyPEM := newSignedStore(t)
	trust, err := singleKeyTrust(publicKeyPEM, "johnny@example.com")
	require.NoError(t, err)

	records := sealN(t, store, 3)

	require.NoError(t, os.Remove(filepath.Join(store.runlogDir(), records[1].ID+".json")))

	report, err := store.VerifyIntegrity(trust)
	require.NoError(t, err)
	require.False(t, report.OK)

	require.NoError(t, store.AppendIntegrityEvent(report))

	// The log now carries its own tamper history: an auditor sees not only that a
	// record vanished, but that Janus noticed, when, and under whose key.
	sealed, _, err := store.SealedRecords()
	require.NoError(t, err)

	var found *RunRecord

	for _, record := range sealed {
		if record.Kind == KindIntegrityEvent {
			found = record
		}
	}

	require.NotNil(t, found, "a detected gap must be appended to the chain")
	require.True(t, found.Sealed)
	require.NotEmpty(t, found.Signature)
	require.NoError(t, VerifyRecord(found, publicKeyPEM))

	description, err := found.GetDescription()
	require.NoError(t, err)
	require.Equal(t, report.Summary(), description)
}

func TestNoIntegrityEventWhenChainIsFine(t *testing.T) {
	store, publicKeyPEM := newSignedStore(t)
	trust, err := singleKeyTrust(publicKeyPEM, "johnny@example.com")
	require.NoError(t, err)

	sealN(t, store, 2)

	report, err := store.VerifyIntegrity(trust)
	require.NoError(t, err)
	require.True(t, report.OK)

	require.NoError(t, store.AppendIntegrityEvent(report))

	sealed, _, err := store.SealedRecords()
	require.NoError(t, err)
	require.Len(t, sealed, 2, "a healthy log must not accumulate integrity events")
}

func TestIntegrityEventIsNotDuplicatedOnRepeatedVerification(t *testing.T) {
	store, publicKeyPEM := newSignedStore(t)
	trust, err := singleKeyTrust(publicKeyPEM, "johnny@example.com")
	require.NoError(t, err)

	records := sealN(t, store, 3)

	require.NoError(t, os.Remove(filepath.Join(store.runlogDir(), records[1].ID+".json")))

	// Detect the same break twice — as would happen across two model loads — and
	// append an integrity event after each detection.
	report1, err := store.VerifyIntegrity(trust)
	require.NoError(t, err)
	require.False(t, report1.OK)
	require.NoError(t, store.AppendIntegrityEvent(report1))

	report2, err := store.VerifyIntegrity(trust)
	require.NoError(t, err)
	require.False(t, report2.OK)
	require.NoError(t, store.AppendIntegrityEvent(report2))

	sealed, _, err := store.SealedRecords()
	require.NoError(t, err)

	eventCount := 0

	for _, record := range sealed {
		if record.Kind == KindIntegrityEvent {
			eventCount++
		}
	}

	require.Equal(t, 1, eventCount, "the same detected break must not be recorded twice")
}

// Finding 2: with no signer configured, an "integrity event" would be an
// unsigned assertion — worthless as evidence — and, because dedup only ever
// consults sealed (signed) records, nothing would bound how many accumulate.
// AppendIntegrityEvent must refuse instead of writing anything.
func TestAppendIntegrityEventRequiresASigner(t *testing.T) {
	// A store with no signer can never seal a record in the first place (see
	// AddRun/sealLocked), so the only way to reach "sealed records with a
	// detected break" is to break a chain sealed by a signed store, then
	// reopen the same directory without a signer — exactly the reachable case
	// described in the finding: a license carries a SigningPublicKey but
	// signing.private_key_path is unset, so BuildSigner errors while
	// BuildTrustStore succeeds.
	signedStore, publicKeyPEM := newSignedStore(t)
	trust, err := singleKeyTrust(publicKeyPEM, "johnny@example.com")
	require.NoError(t, err)

	records := sealN(t, signedStore, 3)
	require.NoError(t, os.Remove(filepath.Join(signedStore.runlogDir(), records[1].ID+".json")))

	store := NewRunLogStore(signedStore.baseDir, "model.mod")
	require.NoError(t, store.Load())

	report, err := store.VerifyIntegrity(trust)
	require.NoError(t, err)
	require.False(t, report.OK)

	filesBefore, err := os.ReadDir(store.runlogDir())
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		err := store.AppendIntegrityEvent(report)
		require.Error(t, err, "call %d: an unsigned integrity event is not an audit fact and must not be written", i)
		require.ErrorIs(t, err, ErrIntegrityEventRequiresSigner)
	}

	filesAfter, err := os.ReadDir(store.runlogDir())
	require.NoError(t, err)
	require.Equal(t, len(filesBefore), len(filesAfter),
		"no record — signed, unsigned, sealed or draft — may be written when there is no signer")

	sealed, _, err := store.SealedRecords()
	require.NoError(t, err)

	for _, record := range sealed {
		require.NotEqual(t, KindIntegrityEvent, record.Kind, "no integrity event may be sealed without a signer")
	}
}

// CRITICAL 2: Bob opens a model directory Alice ran in, or a single user opens
// their own log after rotating their signing key (which the Security
// Considerations tell them to do after a compromise). The head is signed by a
// key this installation does not hold. NOTHING is wrong with the log.
//
// Janus must not seal a permanent, immutable, signed record into a regulated
// audit trail asserting the log was found broken. That is not detection; it is
// fabricated audit evidence.
func TestUntrustedHeadDoesNotWriteAnIntegrityEvent(t *testing.T) {
	store, _ := newSignedStore(t)

	_, otherPublicKey := generateTestKeyPair(t)
	otherTrust, err := singleKeyTrust(encodePublicKeyPEM(t, otherPublicKey), "bob@example.com")
	require.NoError(t, err)

	sealN(t, store, 3)

	filesBefore, err := os.ReadDir(store.runlogDir())
	require.NoError(t, err)

	// Repeated opens, as a colleague reopening the model would do.
	for i := 0; i < 3; i++ {
		report, err := store.VerifyIntegrity(otherTrust)
		require.NoError(t, err)

		require.False(t, report.OK, "call %d: an unverifiable head is genuinely not fully verified", i)
		require.False(t, report.HasIntegrityBreak(), "call %d: an unknown key is not a broken log", i)

		require.NoError(t, store.AppendIntegrityEvent(report))
	}

	filesAfter, err := os.ReadDir(store.runlogDir())
	require.NoError(t, err)
	require.Equal(t, len(filesBefore), len(filesAfter),
		"no record may be written into the audit trail because Janus does not recognise a signing key")

	sealed, _, err := store.SealedRecords()
	require.NoError(t, err)
	require.Len(t, sealed, 3)

	for _, record := range sealed {
		require.NotEqual(t, KindIntegrityEvent, record.Kind,
			"an untrusted-key-only report must append NOTHING to the chain")
	}
}

// Regression guard for the fix above: narrowing the append condition must not
// have narrowed it past the case it exists for. A real break still gets exactly
// one event.
func TestGenuineBreakStillWritesAnIntegrityEvent(t *testing.T) {
	store, publicKeyPEM := newSignedStore(t)
	trust, err := singleKeyTrust(publicKeyPEM, "johnny@example.com")
	require.NoError(t, err)

	records := sealN(t, store, 3)
	require.NoError(t, os.Remove(filepath.Join(store.runlogDir(), records[1].ID+".json")))

	report, err := store.VerifyIntegrity(trust)
	require.NoError(t, err)
	require.True(t, report.HasIntegrityBreak(), "a missing record IS a genuine break")

	require.NoError(t, store.AppendIntegrityEvent(report))

	sealed, _, err := store.SealedRecords()
	require.NoError(t, err)

	eventCount := 0

	for _, record := range sealed {
		if record.Kind == KindIntegrityEvent {
			eventCount++
		}
	}

	require.Equal(t, 1, eventCount, "a genuine break must still be recorded into the chain, exactly once")
}

// Finding 3: dedup must not silently swallow a break that recurs AFTER the log
// was healed. Healing (restoring the exact missing bytes) never itself seals a
// new record, so nothing else marks the recurrence as a fresh incident —
// that's exactly what ChainReport.healedSincePriorCheck exists to carry.
func TestBreakRecurrenceAfterHealIsRecorded(t *testing.T) {
	store, publicKeyPEM := newSignedStore(t)
	trust, err := singleKeyTrust(publicKeyPEM, "johnny@example.com")
	require.NoError(t, err)

	records := sealN(t, store, 3)

	victim := records[1]
	victimPath := filepath.Join(store.runlogDir(), victim.ID+".json")
	victimBytes, err := os.ReadFile(victimPath)
	require.NoError(t, err)

	// Break it, detect it, record it.
	require.NoError(t, os.Remove(victimPath))

	report1, err := store.VerifyIntegrity(trust)
	require.NoError(t, err)
	require.False(t, report1.OK)
	require.NoError(t, store.AppendIntegrityEvent(report1))

	// Heal it — restore the exact bytes, as fixing a deletion from backup
	// would. Healing does not seal any new record.
	require.NoError(t, os.WriteFile(victimPath, victimBytes, 0600))

	healthyReport, err := store.VerifyIntegrity(trust)
	require.NoError(t, err)
	require.True(t, healthyReport.OK, "restoring the exact bytes must heal the chain")

	// Break it again, the exact same way — the identical break recurring.
	require.NoError(t, os.Remove(victimPath))

	report2, err := store.VerifyIntegrity(trust)
	require.NoError(t, err)
	require.False(t, report2.OK)
	require.Equal(t, report1.breakSignature(), report2.breakSignature(),
		"the recurrence must be identical to the original break, or this test proves nothing")

	require.NoError(t, store.AppendIntegrityEvent(report2))

	sealed, _, err := store.SealedRecords()
	require.NoError(t, err)

	eventCount := 0

	for _, record := range sealed {
		if record.Kind == KindIntegrityEvent {
			eventCount++
		}
	}

	require.Equal(t, 2, eventCount,
		"a break that recurs after the chain was healed is a NEW incident and must get its own testimony")
}
