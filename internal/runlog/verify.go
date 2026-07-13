package runlog

import (
	"fmt"
	"strings"
)

// ChainGap is a hole in the chain: a sealed record that should exist and does not.
type ChainGap struct {
	AfterSequence    int    // the last sequence present before the hole
	MissingSequence  int    // the sequence that is absent
	ExpectedPrevHash string // what the following record expected to chain to
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

	if r.TipMismatch {
		fmt.Fprintf(&b, "\n  Tip mismatch: head declares sequence %d, highest present is %d — the newest record(s) were removed.",
			r.ExpectedTip, r.ActualTip)
	}

	if r.HeadUntrusted {
		b.WriteString("\n  The chain head is not signed by a trusted key.")
	}

	fmt.Fprintf(&b, "\n  %d sealed record(s) present and individually valid.", r.Sealed)

	return b.String()
}

// VerifyChain verifies the sealed records as a set: contiguous sequences, intact
// prev-hash links, and a tip that matches the signed head.
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

	// Sequence continuity. Sequences start at 1 and must not skip.
	expected := 1

	for _, record := range records {
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

	// Prev-hash linkage between adjacent survivors. Only meaningful where no gap
	// separates them — across a gap the break is already reported above.
	for i := 1; i < len(records); i++ {
		previous, current := records[i-1], records[i]

		if current.Sequence != previous.Sequence+1 {
			continue
		}

		previousHash, err := RecordHash(previous)
		if err != nil || current.PrevHash != previousHash {
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
