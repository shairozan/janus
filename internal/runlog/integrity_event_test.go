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
	trust, err := NewLicenseTrust(publicKeyPEM, "johnny@example.com")
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
	trust, err := NewLicenseTrust(publicKeyPEM, "johnny@example.com")
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
	trust, err := NewLicenseTrust(publicKeyPEM, "johnny@example.com")
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
