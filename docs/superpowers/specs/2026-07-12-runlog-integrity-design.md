# Run-log integrity: hash chain, seal lifecycle, and trust anchor

**Date:** 2026-07-12
**Status:** Approved, not yet implemented
**Scope:** `internal/runlog`, `internal/signing`, plus the completion paths in `internal/gui` and `internal/mcpservice`.
**Out of scope (separate spec):** open-sourcing Janus. This design is a prerequisite for it — see "Why this unblocks open source" at the end.

## The problem

Three defects, one of them live today.

### 1. Deletion of a whole record is undetectable

`.janus/runlog/index.json` is unsigned plain JSON, and its own struct comment states the design
(`internal/runlog/store.go:37`): *"This file is gitignored and auto-regenerates from individual run
files."*

`rebuildIndex` (`store.go:141-179`) enumerates runs by **globbing the disk** and sets
`TotalRuns = len(entries)` — a recount, never a comparison against a signed expectation. Delete a run
file and the store rebuilds a smaller index, **persists it back to disk**, and reports no error. Where a
loader does hold a list of expected IDs, its response to a missing file is `continue` (`store.go:457`,
and `GetAllRuns` at `store.go:541` does it without even a log line).

Each record is signed *individually and independently* — that is the explicit design (`store.go:334`).
So modifying a record is caught; **removing one is invisible**. There is no hash chain, no sequence, no
signed count, and no set-level verification function anywhere in the codebase. The only production
caller of verification is a GUI table cell (`internal/gui/app.go:1003`), which verifies records that
successfully loaded — it cannot, by construction, notice one that didn't.

`documentation/features/signed_runlog.md:231` claims 21 CFR 11.10(e) audit-trail coverage. The word
doing the work is *"complete"*. Each record is tamper-evident; the **set** has no integrity protection
at all. Silent deletion of an audit entry is precisely the threat that control exists to stop.

### 2. There is no trust anchor — forgery is trivial

`VerifyRecordStatus` prefers the public key **embedded in the record it is verifying**
(`internal/runlog/signer.go:93`, and again at `:159`):

```go
publicKeyPEM := record.SignerPublicKey
if publicKeyPEM == "" { publicKeyPEM = fallbackPublicKeyPEM }
```

`SetFallbackKey` (`store.go:76`) is never called in production — only in tests — and the GUI passes `""`
explicitly. So an attacker generates their own RSA keypair, signs a fabricated record, embeds their own
`signer_public_key` and `signer_email`, and the GUI renders a green ✓ *"Signed by whoever they typed."*

The license JWT's `claims.SigningPublicKey` — the one value that could anchor trust — is verified once at
startup by `ValidateSigningKeyPair` (`internal/license/validator/validator.go:173-210`) and then
**discarded**. It is never plumbed into verification.

### 3. `UpdateRun` silently invalidates signatures (live bug — CONFIRMED BY TEST)

**Reproduced**, not merely inferred. `internal/runlog/update_signature_test.go` walks the exact
production path (`AddRun` with a signer → mutate status/exit code → `UpdateRun` → reload → verify):

```
signature at creation:  mWYY6HDUpLCcZ1Eb6Y0BTNIfUQA5byG0...
signature after update: mWYY6HDUpLCcZ1Eb6Y0BTNIfUQA5byG0...   <- identical; never re-signed
status on disk:         completed                              <- but the content changed

signature verification failed: crypto/rsa: verification error
```

The signature is byte-identical across the update while the signed content changed. The record on disk
is signed over `status: running` but contains `status: completed`. `VerifyRecordStatus` returns
`VerificationInvalid` — Janus renders its own normal completion path as tampering. Every run that starts
and then finishes, with signing enabled, is permanently unverifiable.

The test is committed and currently **fails by design**: it is the regression test this work must turn
green.


The mechanism: `AddRun` signs at creation (`store.go:315-319`), unconditionally when a signer is present.
`UpdateRun` then mutates the record — status, exit code, description — and re-signs **only if
`record.Signature == ""`** (`store.go:560`), which it never is, because `AddRun` just set it. The guard's
own comment says *"isn't already signed"*, so it is deliberate, not a typo. `writeRunFileLocked` then
persists the **mutated content with the stale signature**.

Nothing malicious occurs; this is the app's own normal completion path
(`internal/mcpservice/result.go:80`, `internal/gui/app.go:4430`). It only manifests when a signer is
configured — i.e. exactly on the compliance path the feature exists to serve, which is why it has gone
unnoticed.

## The root cause

An audit trail is being treated as mutable state. A hash chain cannot be bolted onto a store whose
records change in place. Fixing (3) properly is what makes (1) possible.

## Design

### Record lifecycle: draft → sealed

A run that is still executing is not an audit fact. So:

- **`AddRun` writes a draft**: unsigned, unchained, no sequence. Free to update.
- **Sealing** happens once, when the run reaches a terminal status. In a single step, under the existing
  `s.mu` lock, the record is assigned `sequence` and `prev_hash`, then signed.
- **Sealed records are immutable.** `UpdateRun` on a sealed record returns an error rather than
  corrupting the chain.

This eliminates defect (3) by construction: there is no window in which a signed record is mutated,
because signing is the *last* thing that happens to it.

Corrections after sealing are **amendments**: a new record carrying `amends: <original-id>`, sealed and
chained like any other. The original remains. An auditor sees a correction, not a rewrite — which is what
Part 11 expects.

### The chain

`RunRecord` gains two fields:

```go
Sequence int    `json:"sequence,omitempty"`  // monotonic, per model, assigned at seal
PrevHash string `json:"prev_hash,omitempty"` // SHA-256 of the previous sealed record's canonical bytes
```

Both are inside the signed payload (they must be — an unsigned sequence number is worthless).

The chain covers **sealed records only**, ordered by `sequence`. Deleting a sealed record leaves the next
record's `prev_hash` pointing at nothing. Reordering and renumbering are equally caught.

`index.json` **stays exactly what it is today** — a rebuildable, gitignored cache, still globbed from
disk. That is now fine, because it is no longer load-bearing for integrity. Integrity lives in the chain,
which lives inside the records themselves.

### The signed head checkpoint

`.janus/runlog/head.json`, signed with the same key:

```json
{ "sequence": 42, "tip_hash": "3f9a1c…", "signed_at": "...", "signature": "..." }
```

This is what catches **tail truncation**. A pure chain cannot: lop off the newest records and the
remainder is internally consistent. The head declares where the chain is *supposed* to end.

### Collection-level verification (new — does not exist today)

```go
func VerifyChain(records []*RunRecord, head *Head, trust TrustStore) ChainReport
```

`ChainReport` names precisely what is wrong: gaps (with the sequence numbers), dangling `prev_hash`
values, tip mismatch against `head.json`, and per-record signature/trust status.

**Chain integrity is a property of the set, not of any record.** A record adjacent to a gap still has a
perfectly valid signature, and must still report as valid. Marking surviving records red would be a lie
that trains users to ignore red.

Worked example — Johnny accidentally deletes run 17 of 42:

```
Run log INCOMPLETE — 1 record missing.
  Gap at sequence 17 (record 18 expects prev_hash 3f9a1c…, not found)
  Records 1–16:  chain intact, signatures valid
  Records 18–42: chain intact, signatures valid
  Expected tip:  sequence 42  ✓ matches head.json
```

If Johnny instead deletes the tail (40–42), the surviving 1–39 chain is self-consistent, but `head.json`
declares tip 42 → **truncation detected**.

### The trust anchor

```go
type TrustStore interface {
    Trusted(fingerprint string) bool
    Describe(fingerprint string) (Signer, bool)
}
```

`VerifyRecordStatus` **stops preferring `record.SignerPublicKey`**. A record's self-supplied key may no
longer vouch for itself. The signer's fingerprint must be present in the `TrustStore` or the record is
`Untrusted`.

Two implementations, one verify path:

- **`LicenseTrustStore`** — populated from `claims.SigningPublicKey`, which `validator.go:173-210` already
  validates at startup and currently throws away.
- **`KeyringTrustStore`** — populated from a user/org-managed keyring file (`.janus/trust/keys.yaml`).

### Verification states

`VerificationStatus` (`internal/runlog/types.go:10-18`) is today `Unsigned | Valid | Invalid`. It gains:

- **`Untrusted`** — signature is cryptographically fine, but the signing key is not authorized. This is
  neither valid nor invalid, and **must never render green**.
- **`ChainBroken`** — dangling `prev_hash` or a sequence gap at this record.

### Behaviour on a detected gap

**Warn loudly, keep working.** A persistent, non-dismissable banner names the gap; affected runs are
flagged in the table; the log remains usable. Blocking a scientist's daily workflow behind a modal
teaches people to click through modals.

**A detected gap is itself appended to the chain** as a sealed, signed integrity-event record. The log
therefore carries its own tamper history: an auditor sees not just that a record vanished, but that Janus
detected it, when, and under whose key. It also makes delete-then-restore visible.

## Honest limits

State these in the docs rather than papering over them:

1. **A determined attacker with disk write access AND the signing private key can still truncate the
   tail** — delete the newest records, delete `head.json`, and re-sign a shorter head. Without the
   private key, the forged head is rejected by the `TrustStore`. Closing this fully requires an external
   witness (remote append-only log or countersignature), which is a later decision and sits awkwardly
   with an open-source, no-infrastructure goal.
2. **Accidental deletion is fully covered, including the tail**, because Johnny will not also delete and
   re-sign `head.json`. This is the realistic case and the one auditors care about most.
3. **The system can prove removal, never explain it.** A gap is a gap; whether it was malice, `rm -rf`,
   or a flaky sync client is for a human to establish. Making removal *evident* is the whole job.

## Why this unblocks open source

The license server is currently the *only* thing binding a signing key to an identity — and it does that
job badly (checked at startup, discarded at verify). Removing licensing without replacing it would strip
out the last vestige of what was supposed to make a signature mean something.

The `TrustStore` interface is the replacement. Once verification asks *"is this fingerprint authorized?"*
instead of *"did this record bring its own key?"*, the key source becomes a configuration detail.
Open-sourcing then means swapping `LicenseTrustStore` for `KeyringTrustStore` — **not** rewriting the
security model. Done in this order, going open source does not weaken the compliance story; it forces it
to finally become true.

## Success criteria

1. Deleting a sealed record mid-chain produces a report naming the exact missing sequence, while every
   surviving record still verifies as valid.
2. Deleting the tail is detected via `head.json` tip mismatch.
3. A record signed by an unauthorized key reports `Untrusted` and never renders green.
4. `AddRun` → `UpdateRun` → seal produces a record that **verifies**, closing the live bug.
5. `UpdateRun` on a sealed record is refused.
6. A detected gap appends a sealed integrity-event record to the chain.
