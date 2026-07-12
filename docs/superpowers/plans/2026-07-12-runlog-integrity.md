# Run-log Integrity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make removal of a run record evident, make forged records untrusted, and fix the live bug where a completed run is written with a stale signature and rendered as tampered.

**Architecture:** Records gain a draft → sealed lifecycle. `AddRun` no longer signs; sealing happens once, at terminal status, assigning `Sequence` + `PrevHash` and *then* signing — so a signed record is never mutated. Sealed records form a hash chain; a signed `head.json` records the expected tip, catching tail truncation. A new collection-level `VerifyChain` reports gaps precisely. A `TrustStore` decides which signer fingerprints are authorized, so verification stops trusting the key a record supplies about itself.

**Tech Stack:** Go. `crypto/rsa` + `crypto/sha256` via the existing `internal/signing` package. `github.com/gofrs/flock` for the cross-process index lock (already used). Tests: `testify` (`require`/`assert`) under the `unit` build tag.

## Global Constraints

- **Go idiom, per CLAUDE.md:** newline before `return`; unused params become `_`; errors are never abandoned or silently handled; no global variables; nothing constructed in lower layers — dependencies are built high and handed down.
- **No data migration.** Janus is not live. There are no legacy unchained records. The chain starts at sequence 1. Do not write migration code, compatibility shims, or "legacy record" branches.
- **Tests use the `unit` build tag.** Every test file starts with `//go:build unit` and `// +build unit`.
- **Run tests with:** `go test -tags unit ./internal/runlog/`
- **Reuse the existing test helpers** in `internal/runlog/signer_test.go`: `generateTestKeyPair(t)`, `writePrivateKeyPEM(t, path, key)`, `encodePublicKeyPEM(t, key)`. Do not write new ones.
- **Module path is `github.com/pharmalytica/janus`** (unchanged despite the shairozan move).
- **Hashing must use `json.Marshal` (compact), never `json.MarshalIndent`.** The file on disk is written with `MarshalIndent`; the *hash* must not depend on formatting.
- **Existing failing regression test that this work must turn green:** `internal/runlog/update_signature_test.go` → `TestUpdateRunPreservesSignatureValidity`. It currently fails with `crypto/rsa: verification error`. Do not delete or weaken it.
- Spec: `docs/superpowers/specs/2026-07-12-runlog-integrity-design.md`

---

## File Structure

| File | Responsibility |
|---|---|
| `internal/runlog/chain.go` | **Create.** Chain primitives: `RecordHash`, the `Head` type, head read/write/sign/verify. Pure functions plus head file I/O. No store, no GUI. |
| `internal/runlog/trust.go` | **Create.** `TrustStore` interface and the `KeySet` implementation, with constructors for the license-backed and keyring-backed cases. |
| `internal/runlog/verify.go` | **Create.** Collection-level `VerifyChain` + `ChainReport`. The set-level check that does not exist today. |
| `internal/runlog/types.go` | **Modify.** `RunRecord` gains `Sequence`, `PrevHash`, `Amends`, `Sealed`. `VerificationStatus` gains `VerificationUntrusted`, `VerificationChainBroken`. |
| `internal/runlog/signer.go` | **Modify.** `VerifyRecord`/`VerifyRecordStatus` take a `TrustStore` and stop preferring `record.SignerPublicKey`. |
| `internal/runlog/store.go` | **Modify.** `AddRun` writes drafts; new `Seal`; `UpdateRun` refuses sealed records; `AppendIntegrityEvent`. |
| `internal/gui/app.go` | **Modify.** Verification column + the incomplete-log banner. |
| `internal/mcpservice/result.go` | **Modify.** Completion path seals instead of mutating a signed record. |

The chain lives in `chain.go` rather than `store.go` because it is pure logic over records and must be testable without a store, a disk, or a signer. `store.go` is already ~600 lines and doing enough.

---

## Task 1: Chain primitives — record hash and the signed head

**Files:**
- Create: `internal/runlog/chain.go`
- Create: `internal/runlog/chain_test.go`
- Modify: `internal/runlog/types.go` (add the four `RunRecord` fields)

**Interfaces:**
- Produces, relied on by every later task:
  - `func RecordHash(r *RunRecord) (string, error)` — hex SHA-256 of `json.Marshal(r)` over the **full** record including its signature.
  - `type Head struct { Sequence int; TipHash string; SignedAt time.Time; Signature string; SignerFingerprint string }`
  - `func SignHead(h *Head, signer *signing.Signer) error`
  - `func VerifyHead(h *Head, publicKeyPEM string) error`
  - `func ReadHead(path string) (*Head, error)` — returns `(nil, nil)` when the file does not exist (genesis).
  - `func WriteHead(path string, h *Head) error` — atomic write via temp file + rename.
  - `RunRecord.Sequence int`, `RunRecord.PrevHash string`, `RunRecord.Amends string`, `RunRecord.Sealed bool`

- [ ] **Step 1: Add the record fields**

In `internal/runlog/types.go`, inside `RunRecord`, immediately after the `Kind`/`ParentID` block (currently lines 86-87):

```go
	// Chain linkage. Assigned once, at seal time, and covered by the signature —
	// an unsigned sequence number would be worthless. Sequence is monotonic per
	// model starting at 1; PrevHash is the RecordHash of the previous sealed
	// record ("" for the genesis record). Drafts have Sealed=false and carry
	// neither.
	Sequence int    `json:"sequence,omitempty"`
	PrevHash string `json:"prev_hash,omitempty"`
	Sealed   bool   `json:"sealed,omitempty"`

	// Amends names the ID of a sealed record this record corrects. Sealed records
	// are immutable, so a correction is an append, never a rewrite.
	Amends string `json:"amends,omitempty"`
```

- [ ] **Step 2: Write the failing test for `RecordHash` and head round-tripping**

Create `internal/runlog/chain_test.go`:

```go
//go:build unit
// +build unit

package runlog

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/signing"
)

func TestRecordHashIsStableAndContentSensitive(t *testing.T) {
	a := &RunRecord{ID: "r1", Status: "completed", Sequence: 1}
	b := &RunRecord{ID: "r1", Status: "completed", Sequence: 1}

	ha, err := RecordHash(a)
	require.NoError(t, err)
	hb, err := RecordHash(b)
	require.NoError(t, err)
	require.Equal(t, ha, hb, "identical records must hash identically")
	require.Len(t, ha, 64, "hex sha256 is 64 chars")

	b.Status = "failed"
	hc, err := RecordHash(b)
	require.NoError(t, err)
	require.NotEqual(t, ha, hc, "a content change must change the hash")
}

func TestRecordHashCoversTheSignature(t *testing.T) {
	// PrevHash must chain the previous record's signature too, otherwise an
	// attacker could swap a signature without breaking the chain.
	a := &RunRecord{ID: "r1", Status: "completed", Signature: "sigA"}
	before, err := RecordHash(a)
	require.NoError(t, err)

	a.Signature = "sigB"
	after, err := RecordHash(a)
	require.NoError(t, err)

	require.NotEqual(t, before, after, "RecordHash must cover the signature field")
}

func TestHeadSignVerifyRoundTrip(t *testing.T) {
	dir := t.TempDir()
	privateKey, publicKey := generateTestKeyPair(t)
	keyPath := filepath.Join(dir, "k.pem")
	writePrivateKeyPEM(t, keyPath, privateKey)
	signer, err := signing.NewSigner(keyPath)
	require.NoError(t, err)
	publicKeyPEM := encodePublicKeyPEM(t, publicKey)

	h := &Head{Sequence: 7, TipHash: "abc123"}
	require.NoError(t, SignHead(h, signer))
	require.NotEmpty(t, h.Signature)
	require.NotEmpty(t, h.SignerFingerprint)

	require.NoError(t, VerifyHead(h, publicKeyPEM), "a freshly signed head must verify")

	// Truncation attempt: rewind the tip without re-signing.
	h.Sequence = 5
	require.Error(t, VerifyHead(h, publicKeyPEM), "a mutated head must not verify")
}

func TestReadHeadMissingFileIsGenesis(t *testing.T) {
	h, err := ReadHead(filepath.Join(t.TempDir(), "head.json"))
	require.NoError(t, err, "a missing head is genesis, not an error")
	require.Nil(t, h)
}

func TestWriteThenReadHead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "head.json")
	require.NoError(t, WriteHead(path, &Head{Sequence: 3, TipHash: "deadbeef", Signature: "s"}))

	got, err := ReadHead(path)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, 3, got.Sequence)
	require.Equal(t, "deadbeef", got.TipHash)
}
```

- [ ] **Step 3: Run it and watch it fail**

Run: `go test -tags unit -run 'TestRecordHash|TestHead|TestReadHead|TestWriteThenReadHead' ./internal/runlog/`
Expected: FAIL — `undefined: RecordHash`, `undefined: Head`, etc.

- [ ] **Step 4: Implement `internal/runlog/chain.go`**

```go
package runlog

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pharmalytica/janus/internal/signing"
)

// RecordHash returns the hex SHA-256 of a record's canonical JSON encoding.
//
// It hashes the COMPLETE record, signature included, so that a chain link commits
// to the previous record's signature as well as its content. Compact json.Marshal
// is used deliberately: the file on disk is written with MarshalIndent, and the
// hash must not depend on formatting.
func RecordHash(r *RunRecord) (string, error) {
	data, err := json.Marshal(r)
	if err != nil {
		return "", fmt.Errorf("marshaling record for hashing: %w", err)
	}

	sum := sha256.Sum256(data)

	return hex.EncodeToString(sum[:]), nil
}

// Head is the signed checkpoint declaring where the chain is supposed to end.
//
// A hash chain alone cannot detect tail truncation: lop off the newest records and
// what remains is internally consistent. The head is what makes the missing tail
// evident.
type Head struct {
	Sequence          int       `json:"sequence"`
	TipHash           string    `json:"tip_hash"`
	SignedAt          time.Time `json:"signed_at"`
	SignerFingerprint string    `json:"signer_fingerprint"`
	Signature         string    `json:"signature"`
}

// headPayload is the portion of the head covered by the signature.
func headPayload(h *Head) ([]byte, error) {
	copied := *h
	copied.Signature = ""

	data, err := json.Marshal(copied)
	if err != nil {
		return nil, fmt.Errorf("marshaling head for signing: %w", err)
	}

	return data, nil
}

// SignHead signs the head checkpoint in place.
func SignHead(h *Head, signer *signing.Signer) error {
	fingerprint, err := signer.PublicKeyFingerprint()
	if err != nil {
		return fmt.Errorf("head fingerprint: %w", err)
	}

	h.SignedAt = time.Now()
	h.SignerFingerprint = fingerprint
	h.Signature = ""

	payload, err := headPayload(h)
	if err != nil {
		return err
	}

	sig, err := signer.Sign(payload)
	if err != nil {
		return fmt.Errorf("signing head: %w", err)
	}

	h.Signature = base64.StdEncoding.EncodeToString(sig)

	return nil
}

// VerifyHead checks the head's signature against the given public key.
func VerifyHead(h *Head, publicKeyPEM string) error {
	if h.Signature == "" {
		return errors.New("head is unsigned")
	}

	publicKey, err := signing.LoadPublicKeyFromPEM(publicKeyPEM)
	if err != nil {
		return fmt.Errorf("parsing head public key: %w", err)
	}

	sig, err := base64.StdEncoding.DecodeString(h.Signature)
	if err != nil {
		return fmt.Errorf("decoding head signature: %w", err)
	}

	payload, err := headPayload(h)
	if err != nil {
		return err
	}

	if err := signing.Verify(publicKey, payload, sig); err != nil {
		return fmt.Errorf("head signature verification failed: %w", err)
	}

	return nil
}

// ReadHead loads the head checkpoint. A missing file is genesis — an empty chain —
// and returns (nil, nil), NOT an error.
func ReadHead(path string) (*Head, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("reading head: %w", err)
	}

	var h Head
	if err := json.Unmarshal(data, &h); err != nil {
		return nil, fmt.Errorf("parsing head: %w", err)
	}

	return &h, nil
}

// WriteHead writes the head atomically (temp file + rename).
func WriteHead(path string, h *Head) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating head directory: %w", err)
	}

	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling head: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("writing head temp file: %w", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("renaming head into place: %w", err)
	}

	return nil
}
```

- [ ] **Step 5: Run and confirm green**

Run: `go test -tags unit -run 'TestRecordHash|TestHead|TestReadHead|TestWriteThenReadHead' ./internal/runlog/ -v`
Expected: all PASS.

- [ ] **Step 6: Format, vet, commit**

```bash
gofmt -w internal/runlog/chain.go internal/runlog/chain_test.go internal/runlog/types.go
go vet -tags unit ./internal/runlog/
git add internal/runlog/chain.go internal/runlog/chain_test.go internal/runlog/types.go
git commit -m "feat(runlog): chain primitives — record hash and signed head checkpoint

RecordHash covers the full record including its signature, so a chain link
commits to the previous record's signature too. The signed head declares the
expected tip, which is the only way to detect tail truncation — a pure chain
cannot, since a truncated chain is still internally consistent."
```

---

## Task 2: Seal lifecycle — fix the live stale-signature bug

This is the task that turns `TestUpdateRunPreservesSignatureValidity` green.

**Files:**
- Modify: `internal/runlog/store.go` (`AddRun` at :302, `UpdateRun` at :555; add `Seal`, `nextChainLocked`, `headPath`)
- Create: `internal/runlog/seal_test.go`

**Interfaces:**
- Consumes: `RecordHash`, `Head`, `SignHead`, `ReadHead`, `WriteHead` (Task 1).
- Produces:
  - `func (s *RunLogStore) headPath() string`
  - `func (s *RunLogStore) Seal(record *RunRecord) error` — assigns `Sequence`/`PrevHash`, signs, writes record, advances head. Idempotent guard: sealing an already-sealed record is an error.
  - `func IsTerminal(status string) bool` — `"completed"`, `"failed"`, `"cancelled"`.
  - `AddRun` seals automatically when the record arrives already terminal; otherwise writes a draft.
  - `UpdateRun` returns an error when the on-disk record is sealed; seals when the update reaches terminal status.

- [ ] **Step 1: Write the failing tests**

Create `internal/runlog/seal_test.go`:

```go
//go:build unit
// +build unit

package runlog

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/signing"
)

// newSignedStore returns a store with a signer configured, plus the public key PEM.
func newSignedStore(t *testing.T) (*RunLogStore, string) {
	t.Helper()

	dir := t.TempDir()
	privateKey, publicKey := generateTestKeyPair(t)
	keyPath := filepath.Join(dir, "k.pem")
	writePrivateKeyPEM(t, keyPath, privateKey)

	signer, err := signing.NewSigner(keyPath)
	require.NoError(t, err)

	store := NewRunLogStore(dir, "model.mod")
	require.NoError(t, store.Load())
	store.SetSigner(signer, "johnny@example.com")

	return store, encodePublicKeyPEM(t, publicKey)
}

func TestDraftIsNotSigned(t *testing.T) {
	store, _ := newSignedStore(t)

	record := &RunRecord{ModelFile: "model.mod", Status: "running"}
	require.NoError(t, store.AddRun(record))

	require.False(t, record.Sealed, "a running record is a draft")
	require.Empty(t, record.Signature, "drafts must not be signed — signing is the LAST thing that happens")
	require.Zero(t, record.Sequence, "drafts hold no chain position")
}

func TestSealAssignsChainPositionAndSigns(t *testing.T) {
	store, publicKeyPEM := newSignedStore(t)

	first := &RunRecord{ModelFile: "model.mod", Status: "completed"}
	require.NoError(t, store.AddRun(first))

	require.True(t, first.Sealed)
	require.Equal(t, 1, first.Sequence, "the genesis record is sequence 1")
	require.Empty(t, first.PrevHash, "genesis has no predecessor")
	require.NotEmpty(t, first.Signature)
	require.NoError(t, VerifyRecord(first, publicKeyPEM))

	second := &RunRecord{ModelFile: "model.mod", Status: "completed"}
	require.NoError(t, store.AddRun(second))

	require.Equal(t, 2, second.Sequence)

	firstHash, err := RecordHash(first)
	require.NoError(t, err)
	require.Equal(t, firstHash, second.PrevHash, "record 2 must chain to record 1")
}

func TestSealAdvancesTheHead(t *testing.T) {
	store, publicKeyPEM := newSignedStore(t)

	record := &RunRecord{ModelFile: "model.mod", Status: "completed"}
	require.NoError(t, store.AddRun(record))

	head, err := ReadHead(store.headPath())
	require.NoError(t, err)
	require.NotNil(t, head, "sealing must write a head")
	require.Equal(t, 1, head.Sequence)

	recordHash, err := RecordHash(record)
	require.NoError(t, err)
	require.Equal(t, recordHash, head.TipHash)
	require.NoError(t, VerifyHead(head, publicKeyPEM), "the head must be signed")
}

func TestUpdateRunOnSealedRecordIsRefused(t *testing.T) {
	store, _ := newSignedStore(t)

	record := &RunRecord{ModelFile: "model.mod", Status: "completed"}
	require.NoError(t, store.AddRun(record))
	require.True(t, record.Sealed)

	record.Status = "failed"
	err := store.UpdateRun(record)

	require.Error(t, err, "a sealed record is immutable — corrections are amendments, not rewrites")
	require.Contains(t, err.Error(), "sealed")
}

func TestDraftUpdatedToTerminalGetsSealed(t *testing.T) {
	store, publicKeyPEM := newSignedStore(t)

	// This is the production path: record the run when it starts...
	record := &RunRecord{ModelFile: "model.mod", Status: "running"}
	require.NoError(t, store.AddRun(record))
	require.Empty(t, record.Signature)

	// ...then record the outcome when it finishes.
	record.Status = "completed"
	record.ExitCode = 0
	require.NoError(t, store.UpdateRun(record))

	require.True(t, record.Sealed)
	require.Equal(t, 1, record.Sequence)
	require.NotEmpty(t, record.Signature)

	reloaded, err := store.GetRun(record.ID)
	require.NoError(t, err)
	require.Equal(t, "completed", reloaded.Status)
	require.NoError(t, VerifyRecord(reloaded, publicKeyPEM),
		"the completed run must verify — this is the bug the whole task exists to fix")
}

func TestUnsignedStoreStillWorks(t *testing.T) {
	// No signer configured: records are written, never sealed, never signed.
	store := NewRunLogStore(t.TempDir(), "model.mod")
	require.NoError(t, store.Load())

	record := &RunRecord{ModelFile: "model.mod", Status: "completed"}
	require.NoError(t, store.AddRun(record))

	require.Empty(t, record.Signature)
	require.False(t, record.Sealed, "without a signer there is nothing to seal")
}
```

- [ ] **Step 2: Run and watch them fail**

Run: `go test -tags unit -run 'TestDraft|TestSeal|TestUpdateRunOnSealed|TestUnsignedStore' ./internal/runlog/`
Expected: FAIL — `store.headPath undefined`, `record.Sealed undefined` is already resolved by Task 1, so the failures are on `headPath` and on `AddRun` still signing.

- [ ] **Step 3: Add `headPath`, `IsTerminal`, and `nextChainLocked` to `internal/runlog/store.go`**

Add after `indexPath()` (currently ends line 90):

```go
// headPath returns the path to the signed chain head checkpoint.
func (s *RunLogStore) headPath() string {
	return filepath.Join(s.runlogDir(), "head.json")
}

// IsTerminal reports whether a status means the run is over and the record is now
// an audit fact rather than mutable state.
func IsTerminal(status string) bool {
	switch status {
	case "completed", "failed", "cancelled":
		return true
	default:
		return false
	}
}

// nextChainLocked returns the sequence and prev-hash the next sealed record must
// take. The caller holds s.mu AND the cross-process file lock. A missing head is
// genesis: sequence 1, no predecessor.
func (s *RunLogStore) nextChainLocked() (int, string, error) {
	head, err := ReadHead(s.headPath())
	if err != nil {
		return 0, "", fmt.Errorf("reading chain head: %w", err)
	}

	if head == nil {
		return 1, "", nil
	}

	return head.Sequence + 1, head.TipHash, nil
}
```

- [ ] **Step 4: Add `Seal` and `sealLocked` to `internal/runlog/store.go`**

Add immediately after `AddSaga` (currently ends line 363):

```go
// Seal makes a record an audit fact: it assigns the record's chain position,
// signs it, writes it, and advances the head — in that order, once, for good.
//
// Order matters. Sequence and PrevHash are assigned BEFORE signing so the
// signature covers them; an unsigned sequence number is worthless. And signing is
// the LAST mutation, which is what makes it impossible to sign a record and then
// change it — the defect this replaces.
func (s *RunLogStore) Seal(record *RunRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.sealLocked(record)
}

// sealLocked is Seal's body. The caller holds s.mu.
func (s *RunLogStore) sealLocked(record *RunRecord) error {
	if record.Sealed {
		return fmt.Errorf("record %s is already sealed; corrections are amendments, not rewrites", record.ID)
	}

	if s.signer == nil {
		// Nothing to seal with. The record stays a draft; this is not an error,
		// it is simply an unsigned deployment.
		return nil
	}

	// The chain head is shared across processes, so read-modify-write it under the
	// same advisory lock that guards the index.
	fileLock := flock.New(s.lockPath())
	if err := fileLock.Lock(); err != nil {
		return fmt.Errorf("acquiring chain lock: %w", err)
	}
	defer func() { _ = fileLock.Unlock() }()

	sequence, prevHash, err := s.nextChainLocked()
	if err != nil {
		return err
	}

	record.Sequence = sequence
	record.PrevHash = prevHash
	record.Sealed = true

	// Sign LAST — after every other field is final.
	if err := SignRecordWithInfo(record, s.signer, s.signerEmail); err != nil {
		record.Sealed = false

		return fmt.Errorf("signing record: %w", err)
	}

	if err := s.writeRunFileLocked(record); err != nil {
		return err
	}

	tipHash, err := RecordHash(record)
	if err != nil {
		return err
	}

	head := &Head{Sequence: sequence, TipHash: tipHash}
	if err := SignHead(head, s.signer); err != nil {
		return fmt.Errorf("signing chain head: %w", err)
	}

	if err := WriteHead(s.headPath(), head); err != nil {
		return err
	}

	return nil
}
```

- [ ] **Step 5: Rewrite `AddRun` so it no longer signs unconditionally**

Replace the body of `AddRun` (`store.go:302-330`) with:

```go
// AddRun adds a new run record. A run that is still executing is not yet an audit
// fact, so it is written as an unsigned DRAFT. It is signed and chained only when
// it reaches a terminal status — see Seal.
func (s *RunLogStore) AddRun(record *RunRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if record.ID == "" {
		record.ID = uuid.Must(uuid.NewV7()).String()
	}

	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now()
	}

	// A record that arrives already finished (the common case for a fast run, and
	// for saga children) is sealed immediately. sealLocked writes the file itself.
	if IsTerminal(record.Status) && s.signer != nil {
		if err := s.sealLocked(record); err != nil {
			return err
		}

		return s.upsertIndexForRecord(record)
	}

	if err := s.writeRunFileLocked(record); err != nil {
		return err
	}

	return s.upsertIndexForRecord(record)
}
```

- [ ] **Step 6: Rewrite `UpdateRun` so it refuses sealed records and seals drafts**

Replace the body of `UpdateRun` (`store.go:555-577`) with:

```go
// UpdateRun updates a DRAFT record — a run that is still in flight. When the
// update carries the run to a terminal status, the record is sealed: chained,
// signed, and made immutable.
//
// A sealed record cannot be updated. That is the point: it is an audit entry, and
// audit entries are appended to, never rewritten. This is what closes the defect
// where a signed record was mutated and saved with its now-stale signature,
// causing Janus to report its own completion path as tampering.
func (s *RunLogStore) UpdateRun(record *RunRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Trust the record on disk, not the caller's in-memory copy — the caller may
	// be holding a stale struct, and "am I sealed?" must be answered by the file.
	if existing, err := s.loadRunLocked(record.ID); err == nil && existing.Sealed {
		return fmt.Errorf(
			"run %s is sealed and cannot be modified; append an amendment record instead",
			record.ID,
		)
	}

	if IsTerminal(record.Status) && s.signer != nil {
		if err := s.sealLocked(record); err != nil {
			return err
		}

		return s.upsertIndexForRecord(record)
	}

	if err := s.writeRunFileLocked(record); err != nil {
		return err
	}

	return s.upsertIndexForRecord(record)
}
```

- [ ] **Step 7: Run the new tests AND the pre-existing regression test**

```bash
go test -tags unit -run 'TestDraft|TestSeal|TestUpdateRun|TestUnsignedStore' ./internal/runlog/ -v
```

Expected: all PASS — **including `TestUpdateRunPreservesSignatureValidity`**, which has been failing since it was written. That test is the whole point of this task; if it is still red, stop and diagnose rather than moving on.

- [ ] **Step 8: Run the whole package — this changes AddRun's contract, so other tests may legitimately break**

```bash
go test -tags unit ./internal/runlog/
```

Any failure here is a real signal: some existing test asserts that `AddRun` signs. That assertion is now wrong by design (drafts are unsigned). Update those tests to seal, or to assert draft semantics — do **not** weaken the new behaviour to satisfy an old test.

- [ ] **Step 9: Format, vet, commit**

```bash
gofmt -w internal/runlog/store.go internal/runlog/seal_test.go
go vet -tags unit ./internal/runlog/
git add internal/runlog/store.go internal/runlog/seal_test.go
git commit -m "fix(runlog): draft->sealed lifecycle; signing is now the last mutation

AddRun writes an unsigned draft; a record is signed and chained only when it
reaches a terminal status, and sealing assigns Sequence and PrevHash BEFORE
signing so the signature covers them.

This closes the live defect: AddRun signed at creation and UpdateRun re-signed
only when Signature == \"\" (never, since AddRun had just set it), so every run
that started and then completed was persisted with mutated content and a stale
signature and rendered as tampered. Sealed records are now immutable and
UpdateRun refuses them; corrections are amendments.

Turns TestUpdateRunPreservesSignatureValidity green."
```

---

## Task 3: Trust anchor — stop believing a record's own key

**Files:**
- Create: `internal/runlog/trust.go`
- Create: `internal/runlog/trust_test.go`
- Modify: `internal/runlog/types.go` (add `VerificationUntrusted`, `VerificationChainBroken`)
- Modify: `internal/runlog/signer.go` (`VerifyRecordStatus`)

**Interfaces:**
- Produces:
  - `type SignerIdentity struct { Fingerprint string; Email string; PublicKeyPEM string }`
  - `type TrustStore interface { Trusted(fingerprint string) bool; Describe(fingerprint string) (SignerIdentity, bool) }`
  - `type KeySet struct { ... }` implementing `TrustStore`
  - `func NewKeySet(identities ...SignerIdentity) *KeySet`
  - `func NewLicenseTrust(publicKeyPEM, email string) (*KeySet, error)` — the commercial anchor, built from `claims.SigningPublicKey`
  - `func NewKeyringTrust(path string) (*KeySet, error)` — the open-source anchor, from a YAML keyring
  - `func VerifyRecordStatus(record *RunRecord, trust TrustStore) VerificationResult` — **signature changed**: the `fallbackPublicKeyPEM string` param is replaced by a `TrustStore`.
  - `VerificationUntrusted`, `VerificationChainBroken`

**Note on the spec:** the spec described `LicenseTrustStore` and `KeyringTrustStore` as two implementations. One `KeySet` type with two constructors is equivalent and DRY-er — the difference between them is only *where the keys come from*, which is exactly what a constructor is for. Implement it this way.

- [ ] **Step 1: Add the two verification states**

In `internal/runlog/types.go`, extend the const block (currently lines 12-21) and the `String()` switch:

```go
const (
	// VerificationUnsigned indicates the record has no signature.
	VerificationUnsigned VerificationStatus = iota
	// VerificationValid indicates the signature is valid AND the signer is trusted.
	VerificationValid
	// VerificationInvalid indicates the signature verification failed (tampered).
	VerificationInvalid
	// VerificationUnverifiable indicates we cannot verify (no public key available).
	VerificationUnverifiable
	// VerificationUntrusted indicates the signature is cryptographically sound but
	// the signing key is not authorized. This is neither valid nor invalid, and it
	// must NEVER render as green — it is how a forged record announces itself.
	VerificationUntrusted
	// VerificationChainBroken indicates this record's PrevHash points at a record
	// that is not present, or its sequence leaves a gap.
	VerificationChainBroken
)
```

And in `String()`:

```go
	case VerificationUntrusted:
		return "Untrusted"
	case VerificationChainBroken:
		return "Chain broken"
```

- [ ] **Step 2: Write the failing trust tests**

Create `internal/runlog/trust_test.go`:

```go
//go:build unit
// +build unit

package runlog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/signing"
)

func TestForgedRecordIsUntrustedNotValid(t *testing.T) {
	dir := t.TempDir()

	// The legitimate signer.
	goodPriv, goodPub := generateTestKeyPair(t)
	goodPath := filepath.Join(dir, "good.pem")
	writePrivateKeyPEM(t, goodPath, goodPriv)
	goodSigner, err := signing.NewSigner(goodPath)
	require.NoError(t, err)

	// The attacker, with a perfectly valid keypair of their own.
	evilPriv, _ := generateTestKeyPair(t)
	evilPath := filepath.Join(dir, "evil.pem")
	writePrivateKeyPEM(t, evilPath, evilPriv)
	evilSigner, err := signing.NewSigner(evilPath)
	require.NoError(t, err)

	trust, err := NewLicenseTrust(encodePublicKeyPEM(t, goodPub), "johnny@example.com")
	require.NoError(t, err)

	// A record the attacker fabricated and signed with their own key. The signature
	// is cryptographically flawless. The key is simply not ours.
	forged := &RunRecord{ID: "forged", Status: "completed"}
	require.NoError(t, SignRecordWithInfo(forged, evilSigner, "attacker@evil.example"))

	result := VerifyRecordStatus(forged, trust)
	require.Equal(t, VerificationUntrusted, result.Status,
		"a record signed by an unauthorized key must be Untrusted, never Valid")
	require.NotEqual(t, VerificationValid, result.Status)

	// And the genuine article still passes.
	genuine := &RunRecord{ID: "genuine", Status: "completed"}
	require.NoError(t, SignRecordWithInfo(genuine, goodSigner, "johnny@example.com"))
	require.Equal(t, VerificationValid, VerifyRecordStatus(genuine, trust).Status)
}

func TestTamperedRecordIsInvalid(t *testing.T) {
	dir := t.TempDir()
	priv, pub := generateTestKeyPair(t)
	keyPath := filepath.Join(dir, "k.pem")
	writePrivateKeyPEM(t, keyPath, priv)
	signer, err := signing.NewSigner(keyPath)
	require.NoError(t, err)

	trust, err := NewLicenseTrust(encodePublicKeyPEM(t, pub), "johnny@example.com")
	require.NoError(t, err)

	record := &RunRecord{ID: "r", Status: "completed"}
	require.NoError(t, SignRecordWithInfo(record, signer, "johnny@example.com"))

	record.Status = "failed" // tamper, without re-signing

	require.Equal(t, VerificationInvalid, VerifyRecordStatus(record, trust).Status)
}

func TestKeyringTrustLoadsFingerprintsFromFile(t *testing.T) {
	dir := t.TempDir()
	_, pub := generateTestKeyPair(t)
	pubPEM := encodePublicKeyPEM(t, pub)

	keyringPath := filepath.Join(dir, "keys.yaml")
	contents := "keys:\n  - email: johnny@example.com\n    public_key: |\n"
	for _, line := range splitLines(pubPEM) {
		contents += "      " + line + "\n"
	}
	require.NoError(t, os.WriteFile(keyringPath, []byte(contents), 0600))

	trust, err := NewKeyringTrust(keyringPath)
	require.NoError(t, err)

	fingerprint, err := fingerprintOfPEM(pubPEM)
	require.NoError(t, err)

	require.True(t, trust.Trusted(fingerprint), "a key in the keyring must be trusted")
	require.False(t, trust.Trusted("not-a-real-fingerprint"))

	identity, ok := trust.Describe(fingerprint)
	require.True(t, ok)
	require.Equal(t, "johnny@example.com", identity.Email)
}

func TestEmptyTrustStoreTrustsNothing(t *testing.T) {
	trust := NewKeySet()
	require.False(t, trust.Trusted("anything"),
		"an empty trust store is not a permissive one")
}
```

Add this helper at the bottom of `trust_test.go`:

```go
// splitLines splits a PEM block into lines for YAML block-scalar indentation.
func splitLines(s string) []string {
	var out []string

	current := ""

	for _, r := range s {
		if r == '\n' {
			if current != "" {
				out = append(out, current)
			}

			current = ""

			continue
		}

		current += string(r)
	}

	if current != "" {
		out = append(out, current)
	}

	return out
}
```

- [ ] **Step 3: Run and watch fail**

Run: `go test -tags unit -run 'TestForged|TestTampered|TestKeyring|TestEmptyTrust' ./internal/runlog/`
Expected: FAIL — `undefined: NewLicenseTrust`, `undefined: NewKeySet`, `undefined: fingerprintOfPEM`.

- [ ] **Step 4: Implement `internal/runlog/trust.go`**

```go
package runlog

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// SignerIdentity is an authorized signer: who they are, and the key that proves it.
type SignerIdentity struct {
	Fingerprint  string
	Email        string
	PublicKeyPEM string
}

// TrustStore answers the only question that matters at verification time: is this
// signing key authorized?
//
// Before this existed, verification used the public key the record supplied about
// ITSELF, which means a record vouched for its own authenticity — an attacker
// signs a fabricated record with a keypair they generated and it renders as valid.
// A signature is only meaningful relative to a key you already trust.
type TrustStore interface {
	Trusted(fingerprint string) bool
	Describe(fingerprint string) (SignerIdentity, bool)
}

// KeySet is a TrustStore over a fixed set of authorized signers. Where the keys
// come from is the caller's business — that is precisely the seam that lets the
// commercial build anchor trust in a license and an open-source build anchor it in
// a user-managed keyring, with one verification path serving both.
type KeySet struct {
	byFingerprint map[string]SignerIdentity
}

// NewKeySet builds a trust store over the given identities. A KeySet with no
// identities trusts nothing — an empty trust store is not a permissive one.
func NewKeySet(identities ...SignerIdentity) *KeySet {
	byFingerprint := make(map[string]SignerIdentity, len(identities))
	for _, identity := range identities {
		byFingerprint[identity.Fingerprint] = identity
	}

	return &KeySet{byFingerprint: byFingerprint}
}

// Trusted reports whether the fingerprint belongs to an authorized signer.
func (k *KeySet) Trusted(fingerprint string) bool {
	if fingerprint == "" {
		return false
	}

	_, ok := k.byFingerprint[fingerprint]

	return ok
}

// Describe returns the identity behind a fingerprint.
func (k *KeySet) Describe(fingerprint string) (SignerIdentity, bool) {
	identity, ok := k.byFingerprint[fingerprint]

	return identity, ok
}

// fingerprintOfPEM computes the SHA-256 fingerprint of a PEM-encoded public key.
// It must agree with signing.Signer.PublicKeyFingerprint, which fingerprints the
// DER bytes of the PKIX encoding.
func fingerprintOfPEM(publicKeyPEM string) (string, error) {
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return "", fmt.Errorf("no PEM block found in public key")
	}

	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parsing public key: %w", err)
	}

	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", fmt.Errorf("re-encoding public key: %w", err)
	}

	sum := sha256.Sum256(der)

	return hex.EncodeToString(sum[:]), nil
}

// NewLicenseTrust builds the commercial trust anchor from the license's
// SigningPublicKey claim — the value ValidateSigningKeyPair already verifies at
// startup and, until now, threw away.
func NewLicenseTrust(publicKeyPEM, email string) (*KeySet, error) {
	fingerprint, err := fingerprintOfPEM(publicKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("license signing key: %w", err)
	}

	return NewKeySet(SignerIdentity{
		Fingerprint:  fingerprint,
		Email:        email,
		PublicKeyPEM: publicKeyPEM,
	}), nil
}

// keyringFile is the on-disk shape of the open-source trust anchor.
type keyringFile struct {
	Keys []struct {
		Email     string `yaml:"email"`
		PublicKey string `yaml:"public_key"`
	} `yaml:"keys"`
}

// NewKeyringTrust builds the open-source trust anchor from a user- or
// org-maintained keyring. This is the seam that lets Janus drop its license server
// without losing the meaning of a signature.
func NewKeyringTrust(path string) (*KeySet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading keyring %s: %w", path, err)
	}

	var parsed keyringFile
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("parsing keyring %s: %w", path, err)
	}

	identities := make([]SignerIdentity, 0, len(parsed.Keys))

	for i, key := range parsed.Keys {
		fingerprint, err := fingerprintOfPEM(key.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("keyring entry %d (%s): %w", i, key.Email, err)
		}

		identities = append(identities, SignerIdentity{
			Fingerprint:  fingerprint,
			Email:        key.Email,
			PublicKeyPEM: key.PublicKey,
		})
	}

	return NewKeySet(identities...), nil
}
```

- [ ] **Step 5: Rewrite `VerifyRecordStatus` in `internal/runlog/signer.go`**

Replace `VerifyRecordStatus` (currently `signer.go:143-203`) entirely:

```go
// VerifyRecordStatus verifies a run record and returns detailed status for the UI.
//
// It asks the TrustStore whether the signing key is authorized BEFORE it asks
// whether the maths checks out, because a signature from a key nobody authorized
// proves nothing — it is exactly what a forgery looks like. The record's own
// SignerPublicKey is never treated as authority for itself.
func VerifyRecordStatus(record *RunRecord, trust TrustStore) VerificationResult {
	if record.Signature == "" {
		return VerificationResult{
			Status:  VerificationUnsigned,
			Message: "Unsigned",
		}
	}

	signer := record.SignerEmail
	if signer == "" {
		signer = "Unknown"
	}

	if trust == nil {
		return VerificationResult{
			Status:  VerificationUnverifiable,
			Message: "Signed (cannot verify — no trust anchor configured)",
			Signer:  signer,
		}
	}

	identity, ok := trust.Describe(record.SignerFingerprint)
	if !ok {
		return VerificationResult{
			Status:  VerificationUntrusted,
			Message: fmt.Sprintf("Signed by an UNAUTHORIZED key (%s) — not trusted", signer),
			Signer:  signer,
		}
	}

	// Verify against the key the TRUST STORE holds, never the one the record
	// carries. Otherwise the record is still vouching for itself.
	if err := VerifyRecord(record, identity.PublicKeyPEM); err != nil {
		return VerificationResult{
			Status:  VerificationInvalid,
			Message: "Signature invalid — record may have been modified",
			Signer:  signer,
		}
	}

	return VerificationResult{
		Status:  VerificationValid,
		Message: fmt.Sprintf("Signed by %s", signer),
		Signer:  signer,
	}
}
```

Also change `VerifyRecord` (`signer.go:87-99`) to stop preferring the embedded key. Replace its key-selection block:

```go
func VerifyRecord(record *RunRecord, publicKeyPEM string) error {
	if record.Signature == "" {
		return fmt.Errorf("record has no signature")
	}

	if publicKeyPEM == "" {
		return fmt.Errorf("no public key available for verification")
	}
```

(The old lines 92-96, which fell back to `record.SignerPublicKey`, are deleted. `VerifyRecord` now verifies against exactly the key it is given — the caller decides which key that is, and `VerifyRecordStatus` gets it from the trust store.)

- [ ] **Step 6: Confirm the YAML dependency (already present — no action expected)**

`gopkg.in/yaml.v3 v3.0.1` is already a direct dependency in `go.mod:41`. Confirm and move on:

```bash
grep -n 'gopkg.in/yaml.v3' go.mod
```
Expected: `41:	gopkg.in/yaml.v3 v3.0.1`. Do not run `go get`; do not add a new dependency.

**Fingerprint agreement (verified, do not "fix" it):** `signing.PublicKeyFingerprint` is
`hex(sha256(x509.MarshalPKIXPublicKey(pub)))` (`internal/signing/signing.go:179-190`). `fingerprintOfPEM`
above decodes the PEM, parses PKIX, and re-marshals PKIX before hashing — the same DER bytes, hence the
same fingerprint. This agreement is what makes `trust.Describe(record.SignerFingerprint)` find anything at
all; if you change either function, change both.

- [ ] **Step 7: Fix the now-broken callers**

`VerifyRecordStatus`'s signature changed, so `internal/gui/app.go:1003` will not compile. Build to find every caller:

```bash
go build ./... 2>&1 | head -20
```

At `internal/gui/app.go:1003`, the call `runlog.VerifyRecordStatus(&run, "")` becomes `runlog.VerifyRecordStatus(&run, a.trustStore)`. Add a `trustStore runlog.TrustStore` field to the GUI app struct and set it during setup — per CLAUDE.md, construct it at the highest layer (`internal/appsetup`) and hand it down; do not build it inside the GUI.

- [ ] **Step 8: Run tests**

```bash
go test -tags unit ./internal/runlog/
go build ./...
```
Expected: PASS, and a clean build.

- [ ] **Step 9: Format, vet, commit**

```bash
gofmt -w internal/runlog/trust.go internal/runlog/trust_test.go internal/runlog/signer.go internal/runlog/types.go internal/gui/app.go
go vet -tags unit ./internal/runlog/
git add -u && git add internal/runlog/trust.go internal/runlog/trust_test.go
git commit -m "feat(runlog): trust anchor — a record may no longer vouch for itself

VerifyRecordStatus asked the record for the public key with which to verify the
record. An attacker generates a keypair, signs a fabricated run, embeds their own
public key and email, and the GUI renders a green check. SetFallbackKey was never
called in production and the GUI passed \"\".

Verification now consults a TrustStore. One KeySet type, two constructors:
NewLicenseTrust (from the license's SigningPublicKey claim, which was validated at
startup and then discarded) and NewKeyringTrust (from a user-managed keyring) —
which is the seam that lets Janus go open source without losing the meaning of a
signature.

New state: Untrusted. Cryptographically sound, not authorized. Never green."
```

---

## Task 4: `VerifyChain` — the set-level check that does not exist today

**Files:**
- Create: `internal/runlog/verify.go`
- Create: `internal/runlog/verify_test.go`
- Modify: `internal/runlog/store.go` (add `SealedRecords`, `VerifyIntegrity`)

**Interfaces:**
- Consumes: `RecordHash`, `Head`, `VerifyHead` (Task 1); `TrustStore`, `VerifyRecordStatus` (Task 3).
- Produces:
  - `type ChainGap struct { AfterSequence int; MissingSequence int; ExpectedPrevHash string }`
  - `type ChainReport struct { OK bool; Sealed int; Gaps []ChainGap; TipMismatch bool; ExpectedTip int; ActualTip int; HeadUntrusted bool; Records map[string]VerificationResult }`
  - `func (r ChainReport) Summary() string`
  - `func VerifyChain(records []*RunRecord, head *Head, trust TrustStore) ChainReport`
  - `func (s *RunLogStore) SealedRecords() ([]*RunRecord, error)` — sealed records only, sorted by `Sequence`.
  - `func (s *RunLogStore) VerifyIntegrity(trust TrustStore) (ChainReport, error)`

- [ ] **Step 1: Write the failing tests**

Create `internal/runlog/verify_test.go`:

```go
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
```

- [ ] **Step 2: Run and watch fail**

Run: `go test -tags unit -run 'TestIntactChain|TestMiddleDeletion|TestTailTruncation|TestSummary' ./internal/runlog/`
Expected: FAIL — `undefined: VerifyChain`, `store.VerifyIntegrity undefined`.

- [ ] **Step 3: Implement `internal/runlog/verify.go`**

```go
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
```

- [ ] **Step 4: Add `SealedRecords` and `VerifyIntegrity` to `internal/runlog/store.go`**

Add after `GetAllRuns`:

```go
// SealedRecords returns every sealed record, sorted ascending by chain sequence.
// Drafts are excluded: they hold no chain position and are not audit facts.
func (s *RunLogStore) SealedRecords() ([]*RunRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.index == nil {
		return nil, nil
	}

	sealed := make([]*RunRecord, 0, len(s.index.Entries))

	for _, entry := range s.index.Entries {
		record, err := s.loadRunLocked(entry.ID)
		if err != nil {
			// A missing file is exactly the thing we are here to detect. Do not
			// swallow it — the chain check will surface it as a gap.
			continue
		}

		if record.Sealed {
			sealed = append(sealed, record)
		}
	}

	sort.Slice(sealed, func(i, j int) bool {
		return sealed[i].Sequence < sealed[j].Sequence
	})

	return sealed, nil
}

// VerifyIntegrity verifies the run log as a set — the check that did not exist
// before: every prior verification path examined one record at a time and so could
// not, even in principle, notice one that had been removed.
func (s *RunLogStore) VerifyIntegrity(trust TrustStore) (ChainReport, error) {
	sealed, err := s.SealedRecords()
	if err != nil {
		return ChainReport{}, err
	}

	head, err := ReadHead(s.headPath())
	if err != nil {
		return ChainReport{}, fmt.Errorf("reading chain head: %w", err)
	}

	return VerifyChain(sealed, head, trust), nil
}
```

- [ ] **Step 5: Run and confirm green**

```bash
go test -tags unit ./internal/runlog/ -v -run 'TestIntactChain|TestMiddleDeletion|TestTailTruncation|TestSummary'
```
Expected: all PASS. In particular `TestMiddleDeletionIsDetectedAndLocated` must report `MissingSequence == 3` while records 1, 2, 4, 5 still verify as `VerificationValid`.

- [ ] **Step 6: Full package + build**

```bash
go test -tags unit ./internal/runlog/
go build ./...
```

- [ ] **Step 7: Format, vet, commit**

```bash
gofmt -w internal/runlog/verify.go internal/runlog/verify_test.go internal/runlog/store.go
go vet -tags unit ./internal/runlog/
git add internal/runlog/verify.go internal/runlog/verify_test.go internal/runlog/store.go
git commit -m "feat(runlog): VerifyChain — verify the run log as a SET

Until now every verification path examined one record at a time, so it could not
even in principle notice a record that had been removed: rebuildIndex globs the
disk and recounts, and loaders 'continue' past a missing file.

VerifyChain checks sequence continuity, prev-hash linkage, and the tip against the
signed head. A middle deletion is named precisely ('gap at sequence 3') while the
surviving records still report as valid — because they ARE valid, and marking them
red would teach users to ignore red. Tail truncation is caught by the head, which
is the only thing that knows how long the chain was supposed to be."
```

---

## Task 5: Surface it — integrity events and the incomplete-log banner

**Files:**
- Modify: `internal/runlog/store.go` (add `AppendIntegrityEvent`)
- Modify: `internal/gui/app.go` (banner + verification column)
- Create: `internal/runlog/integrity_event_test.go`

**Interfaces:**
- Consumes: `ChainReport` (Task 4), `Seal` (Task 2).
- Produces:
  - `const KindIntegrityEvent = "integrity-event"`
  - `func (s *RunLogStore) AppendIntegrityEvent(report ChainReport) error` — appends a sealed, chained record recording that a gap was observed. No-op when `report.OK`.

- [ ] **Step 1: Write the failing test**

Create `internal/runlog/integrity_event_test.go`:

```go
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
	sealed, err := store.SealedRecords()
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

	sealed, err := store.SealedRecords()
	require.NoError(t, err)
	require.Len(t, sealed, 2, "a healthy log must not accumulate integrity events")
}
```

- [ ] **Step 2: Run and watch fail**

Run: `go test -tags unit -run 'TestDetectedGap|TestNoIntegrityEvent' ./internal/runlog/`
Expected: FAIL — `undefined: KindIntegrityEvent`, `store.AppendIntegrityEvent undefined`.

- [ ] **Step 3: Add the kind constant**

In `internal/runlog/types.go`, in the kind const block (currently lines 62-67):

```go
	// KindIntegrityEvent marks a record that exists to testify that the chain was
	// found broken. The log thereby carries its own tamper history: an auditor sees
	// not only that a record vanished, but that Janus detected it, when, and under
	// whose key. It also makes delete-then-restore visible.
	KindIntegrityEvent = "integrity-event"
```

- [ ] **Step 4: Add `AppendIntegrityEvent` to `internal/runlog/store.go`**

```go
// AppendIntegrityEvent records a detected chain break INTO the chain, as a sealed
// and signed record. It is a no-op for a healthy log — a clean run log must not
// accumulate noise.
func (s *RunLogStore) AppendIntegrityEvent(report ChainReport) error {
	if report.OK {
		return nil
	}

	record := &RunRecord{
		ModelFile: s.modelFile,
		Kind:      KindIntegrityEvent,
		Status:    "completed",
		Timestamp: time.Now(),
	}

	if err := record.SetDescription(report.Summary()); err != nil {
		return fmt.Errorf("recording integrity event description: %w", err)
	}

	return s.AddRun(record)
}
```

- [ ] **Step 5: Run and confirm green**

```bash
go test -tags unit ./internal/runlog/
```
Expected: PASS.

- [ ] **Step 6: Wire the GUI — verification column and banner**

In `internal/gui/app.go`:

1. The verification cell at line 1003 (`runlog.VerifyRecordStatus(&run, "")`) was already changed in Task 3 to pass `a.trustStore`. Extend its rendering so `VerificationUntrusted` and `VerificationChainBroken` are visually distinct from `VerificationValid` and are **never green**.
2. On model load, after the store's `Load()`, call `store.VerifyIntegrity(a.trustStore)`. If `report.OK` is false: call `store.AppendIntegrityEvent(report)`, then display a persistent, non-dismissable banner above the run table containing `report.Summary()`. Do **not** block the user — they keep working; a modal here would simply train people to click through modals.

- [ ] **Step 7: Build and run the GUI test lane**

```bash
go build ./...
mage docker:gui
```

- [ ] **Step 8: Format, vet, commit**

```bash
gofmt -w internal/runlog/store.go internal/runlog/types.go internal/runlog/integrity_event_test.go internal/gui/app.go
go vet -tags unit ./internal/runlog/
git add -u && git add internal/runlog/integrity_event_test.go
git commit -m "feat(runlog): record detected gaps into the chain; warn without blocking

A detected break is appended as a sealed, signed integrity-event record, so the
log carries its own tamper history — an auditor sees not only that a record
vanished but that Janus noticed, when, and under whose key. It also makes
delete-then-restore visible.

The GUI warns loudly and keeps working. Blocking a scientist behind a modal only
teaches people to click through modals."
```

---

## Task 6: Correct the compliance documentation

The current claim is not true, and shipping the fix without correcting the claim leaves the more dangerous artifact in place.

**Files:**
- Modify: `documentation/features/signed_runlog.md` (the 11.10(e) claim at :231, and :7)

- [ ] **Step 1: Rewrite the integrity claims**

At `documentation/features/signed_runlog.md:7`, the claim *"Any modification to a signed record is detectable"* was true but incomplete — it said nothing about removal. Replace it and the 11.10(e) row so they state what the system now actually does, and — importantly — what it does not:

```markdown
Integrity guarantees:

- **Modification** of a sealed record is detected (RSA-SHA256 over the record).
- **Removal** of a sealed record is detected (hash chain over sequence numbers;
  the report names the missing sequence).
- **Reordering or renumbering** is detected (the chain commits to order).
- **Forgery** with an unauthorized key is detected (the signer's fingerprint must
  be present in the trust store; a record cannot vouch for its own key).
- **Truncation of the newest records** is detected via the signed head checkpoint.

Known limit, stated plainly: an attacker holding the signing private key AND write
access to the run-log directory can delete the newest records and re-sign a shorter
head. No purely local audit log can prevent this; detecting it requires an external
witness (a remote append-only log or a countersignature), which Janus does not yet
implement. Accidental loss — the realistic case — is fully covered, including the
tail, because an accident does not re-sign the head.
```

- [ ] **Step 2: Commit**

```bash
git add documentation/features/signed_runlog.md
git commit -m "docs: state run-log integrity guarantees honestly

The 11.10(e) claim rested on the word 'complete'. Each record was individually
tamper-evident; the SET had no integrity protection, so silent deletion of an
audit entry — precisely the threat the control exists to stop — was invisible.

States what is now detected (modification, removal, reordering, forgery,
truncation) and what is not (an attacker with the private key AND disk write
access can still truncate; that needs an external witness)."
```

---

## Self-Review

**Spec coverage.** Draft→sealed lifecycle → Task 2. Sealed immutability / amendments → Task 2 (`Amends` field added in Task 1). Hash chain → Tasks 1, 4. Signed head / tail truncation → Tasks 1, 4. `VerifyChain` + `ChainReport` → Task 4. `TrustStore` (license + keyring) → Task 3. `Untrusted` + `ChainBroken` states → Task 3. Gap → sealed integrity event → Task 5. Warn-loudly-keep-working → Task 5. Honest limits documented → Task 6. Live bug regression test green → Task 2 Step 7. No gaps.

**Placeholders.** None. Every code step carries real code.

**Type consistency.** `RecordHash` (Task 1) is used identically in Tasks 2 and 4. `Head` fields (`Sequence`, `TipHash`, `SignerFingerprint`, `Signature`) are consistent across `SignHead`/`VerifyHead`/`WriteHead`/`nextChainLocked`/`VerifyChain`. `TrustStore.Describe` returns `(SignerIdentity, bool)` and every caller destructures it that way. `VerifyRecordStatus(record, trust)` has the same two-arg shape in Tasks 3, 4, 5 and at the GUI call site. `IsTerminal` is defined once (Task 2) and used in `AddRun` and `UpdateRun`.

**Three risks worth naming, since they are where this will actually go wrong:**

1. **`VerifyRecord`'s signature changes meaning in Task 3** — it no longer falls back to the record's embedded key. `TestUpdateRunPreservesSignatureValidity` and the Task 2 tests call `VerifyRecord(record, publicKeyPEM)` with an explicit key, so they keep working. But any *existing* test in `signer_test.go` that relies on the embedded-key fallback will break, and it must be **fixed, not reverted** — that fallback is the vulnerability.

2. **`AddSaga` seals each child independently** (it calls `AddRun` per record). Each child therefore takes its own chain sequence, and the parent takes one too. That is correct — they are separate audit facts — but it means a saga of 50 fits consumes 51 sequence numbers, and `AddSaga` is not atomic across the chain: a crash mid-saga leaves a valid but shorter chain. Acceptable, and worth a comment at `AddSaga`; do not try to make sagas a single chain entry.

3. **`SealedRecords` skips records it cannot load** (`continue`), which looks like the very bug being fixed. It is not: the skip is what *creates* the sequence gap that `VerifyChain` then reports. The detection has simply moved from the loader to the chain check, which is the only place it can live. Leave the `continue`, and leave the comment explaining why.
