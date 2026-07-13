package runlog

import (
	"fmt"
	"sort"
	"strings"
)

// ChainGap is a hole in the chain: a sealed record that should exist and does not.
type ChainGap struct {
	AfterSequence    int    // the last sequence present before the hole
	MissingSequence  int    // the sequence that is absent
	ExpectedPrevHash string // what the following record expected to chain to
}

// ChainDuplicate is two or more sealed records claiming the same chain
// Sequence. Deletion is not the only way to tamper with a hash-chained log —
// inserting (or duplicating) a record at an already-occupied sequence is just
// as damaging, and is invisible to a check that only looks for holes.
type ChainDuplicate struct {
	Sequence int      // the sequence claimed by more than one record
	IDs      []string // the record IDs claiming it
}

// ChainReport is the result of verifying the run log AS A SET.
//
// Chain integrity is a property of the collection, not of any record. A record
// sitting next to a gap still has a perfectly valid signature and must still be
// reported as valid — its neighbour vanishing does not make it a forgery. So this
// report carries BOTH: the set-level findings, and the per-record status.
type ChainReport struct {
	OK            bool
	Sealed        int
	Gaps          []ChainGap
	Duplicates    []ChainDuplicate
	Unreadable    []string // record IDs whose files exist but could not be loaded
	TipMismatch   bool
	ExpectedTip   int
	ActualTip     int
	HeadUntrusted bool
	Records       map[string]VerificationResult
}

// Summary renders the report the way a human needs to read it: what is missing,
// where, and what is still fine.
func (r ChainReport) Summary() string {
	if r.OK {
		return fmt.Sprintf("Run log intact — %d sealed records, chain and tip verified.", r.Sealed)
	}

	var b strings.Builder

	b.WriteString("RUN LOG INCOMPLETE")

	if len(r.Gaps) > 0 {
		fmt.Fprintf(&b, " — %d record(s) missing.", len(r.Gaps))

		for _, gap := range r.Gaps {
			fmt.Fprintf(&b, "\n  Gap at sequence %d (record %d expects a predecessor that is not present)",
				gap.MissingSequence, gap.MissingSequence+1)
		}
	}

	if len(r.Duplicates) > 0 {
		fmt.Fprintf(&b, "\n  %d sequence(s) claimed by more than one record.", len(r.Duplicates))

		for _, dup := range r.Duplicates {
			fmt.Fprintf(&b, "\n  Duplicate at sequence %d: %s", dup.Sequence, strings.Join(dup.IDs, ", "))
		}
	}

	if len(r.Unreadable) > 0 {
		fmt.Fprintf(&b, "\n  %d record file(s) exist but could not be read (corrupt, not merely missing): %s",
			len(r.Unreadable), strings.Join(r.Unreadable, ", "))
	}

	if r.TipMismatch {
		switch {
		case r.ActualTip < r.ExpectedTip:
			fmt.Fprintf(&b, "\n  Tip mismatch: head declares sequence %d, highest present is %d — the newest record(s) were removed.",
				r.ExpectedTip, r.ActualTip)
		case r.ActualTip > r.ExpectedTip:
			fmt.Fprintf(&b, "\n  Tip mismatch: head declares sequence %d but records exist up to %d — the head was not advanced, or was rolled back.",
				r.ExpectedTip, r.ActualTip)
		default:
			fmt.Fprintf(&b, "\n  Tip mismatch: head declares sequence %d, highest present is %d.",
				r.ExpectedTip, r.ActualTip)
		}
	}

	if r.HeadUntrusted {
		b.WriteString("\n  The chain head is not signed by a trusted key.")
	}

	fmt.Fprintf(&b, "\n  %d sealed record(s) present and individually valid.", r.Sealed)

	return b.String()
}

// breakSignature reduces a report to the identity of the tamper it describes —
// which sequences are missing or duplicated, which record files are unreadable,
// whether the head is untrusted, and the direction (not magnitude) of any tip
// mismatch. It deliberately omits Sealed, ActualTip and ExpectedTip: those
// counters advance every time a new record is sealed, including an
// integrity-event record recording a PRIOR break, so comparing the full
// rendered Summary() would treat the very act of recording a break as a new,
// distinct one and duplicate the event on every subsequent verification. This
// is what AppendIntegrityEvent compares to recognise "this is the same break as
// last time."
func (r ChainReport) breakSignature() string {
	var b strings.Builder

	for _, gap := range r.Gaps {
		fmt.Fprintf(&b, "gap:%d;", gap.MissingSequence)
	}

	for _, dup := range r.Duplicates {
		fmt.Fprintf(&b, "dup:%d:%s;", dup.Sequence, strings.Join(dup.IDs, ","))
	}

	for _, id := range r.Unreadable {
		fmt.Fprintf(&b, "unreadable:%s;", id)
	}

	if r.HeadUntrusted {
		b.WriteString("head-untrusted;")
	}

	if r.TipMismatch {
		switch {
		case r.ActualTip < r.ExpectedTip:
			b.WriteString("tip-truncated;")
		case r.ActualTip > r.ExpectedTip:
			b.WriteString("tip-not-advanced;")
		default:
			b.WriteString("tip-mismatch;")
		}
	}

	return b.String()
}

// VerifyChain verifies the sealed records as a set: contiguous sequences, no
// sequence claimed twice, intact prev-hash links, and a tip that matches the
// signed head.
//
// records MUST be sealed records sorted ascending by Sequence.
func VerifyChain(records []*RunRecord, head *Head, trust TrustStore) ChainReport {
	report := ChainReport{
		OK:      true,
		Sealed:  len(records),
		Records: make(map[string]VerificationResult, len(records)),
	}

	for _, record := range records {
		report.Records[record.ID] = VerifyRecordStatus(record, trust)
	}

	// Sequence continuity. Sequences start at 1 and must not skip. Along the
	// way, also collect every record ID seen at each sequence — a duplicate
	// (two records claiming the same sequence) is a distinct failure mode from
	// a gap, and neither this loop nor the prev-hash loop below would notice
	// it on their own: a duplicate that does not exceed "expected" never
	// trips the gap check, and the prev-hash loop only ever compares adjacent
	// PAIRS, not "how many records share this sequence".
	expected := 1
	bySequence := make(map[int][]string, len(records))

	for _, record := range records {
		bySequence[record.Sequence] = append(bySequence[record.Sequence], record.ID)

		for record.Sequence > expected {
			report.Gaps = append(report.Gaps, ChainGap{
				AfterSequence:    expected - 1,
				MissingSequence:  expected,
				ExpectedPrevHash: record.PrevHash,
			})
			report.OK = false
			expected++
		}

		expected = record.Sequence + 1
	}

	sequences := make([]int, 0, len(bySequence))
	for sequence := range bySequence {
		sequences = append(sequences, sequence)
	}

	sort.Ints(sequences)

	for _, sequence := range sequences {
		ids := bySequence[sequence]
		if len(ids) < 2 {
			continue
		}

		sort.Strings(ids)
		report.OK = false
		report.Duplicates = append(report.Duplicates, ChainDuplicate{Sequence: sequence, IDs: ids})

		for _, id := range ids {
			result := report.Records[id]
			result.Status = VerificationChainBroken
			result.Message = fmt.Sprintf("Sequence %d is claimed by %d records — duplicate or inserted record (%s)",
				sequence, len(ids), strings.Join(ids, ", "))
			report.Records[id] = result
		}
	}

	// Prev-hash linkage between adjacent survivors. Only meaningful where no gap
	// separates them — across a gap the break is already reported above.
	for i := 1; i < len(records); i++ {
		previous, current := records[i-1], records[i]

		if current.Sequence != previous.Sequence+1 {
			continue
		}

		previousHash, err := RecordHash(previous)
		if err != nil {
			report.OK = false

			result := report.Records[current.ID]
			result.Status = VerificationChainBroken
			result.Message = fmt.Sprintf("Cannot verify chain link at sequence %d — record %d could not be hashed: %v",
				current.Sequence, previous.Sequence, err)
			report.Records[current.ID] = result

			continue
		}

		if current.PrevHash != previousHash {
			report.OK = false

			result := report.Records[current.ID]
			result.Status = VerificationChainBroken
			result.Message = fmt.Sprintf("Chain broken at sequence %d — prev_hash does not match record %d",
				current.Sequence, previous.Sequence)
			report.Records[current.ID] = result
		}
	}

	// The tip. This is what catches truncation: a shortened chain is internally
	// consistent, and only the signed head knows how long it was supposed to be.
	if len(records) > 0 {
		report.ActualTip = records[len(records)-1].Sequence
	}

	if head == nil {
		if len(records) > 0 {
			report.OK = false
			report.TipMismatch = true
			report.ExpectedTip = 0
		}

		return report
	}

	report.ExpectedTip = head.Sequence

	if identity, ok := trust.Describe(head.SignerFingerprint); !ok {
		report.HeadUntrusted = true
		report.OK = false
	} else if err := VerifyHead(head, identity.PublicKeyPEM); err != nil {
		report.HeadUntrusted = true
		report.OK = false
	}

	if head.Sequence != report.ActualTip {
		report.TipMismatch = true
		report.OK = false
	}

	return report
}
