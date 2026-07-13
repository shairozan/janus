# Signed Run Log

## Overview

Janus implements cryptographic signing and hash-chaining of run log entries to ensure audit trail integrity for CFR 21 Part 11 compliance. Each execution record is signed with an RSA-SHA256 signature and linked into a hash chain anchored by a signed checkpoint, providing:

- **Tamper Evidence**: Modification of a sealed record, and removal, insertion, duplication, reordering, or tail truncation of records from the log **as a set**, are all detectable and named. This is not unconditional — see [Integrity Guarantees](#integrity-guarantees) for exactly what is covered and what is not.
- **Non-Repudiation**: Signatures are tied to specific users via their license, and are only ever verified against a trust anchor — a record's own embedded key can never vouch for itself
- **Multi-User Support** (*design capability, not yet shipping*): Verification is anchored in a `TrustStore`, so a record CAN be verified by any team member whose trust store holds the signer's key. **The shipping commercial build holds exactly one key — your own** — and provides no mechanism to add another. Records signed by a colleague, or by your own key before you rotated it, therefore report **Untrusted**/**head untrusted** today. See [Known Limits](#known-limits).
- **Merge Conflict Prevention** (*partially retracted*): UUID-based directory storage eliminates Git conflicts **between the individual run record files**. It does **not** eliminate them for the chain: `head.json` is a single shared mutable file, and two branches that each seal a run will conflict on it *and* take the same sequence number. **Read [Known Limits](#known-limits) before branching a model directory.**

## Storage Architecture

Run logs use a **directory-based storage system** designed to prevent Git merge conflicts when multiple team members work on the same model across different branches.

### Directory Structure

```
model-directory/
├── model.mod                      # Your model file
└── .janus/
    └── runlog/
        ├── 019377a8-4e2c-7f1a-8b3d-2c4e5f6a7b8c.json  # Individual run
        ├── 019377b2-1d3e-7a2b-9c4d-3e5f6a7b8c9d.json  # Individual run
        ├── 019377c5-8f4a-7b3c-0d1e-4f6a7b8c9d0e.json  # Individual run
        ├── head.json                                    # Signed chain-tip checkpoint (SHARED, MUTABLE — see Known Limits)
        └── index.json                                   # Auto-generated, rebuildable index
```

### Why Directory-Based Storage?

The previous single-file approach (`model.janus_history.json`) caused merge conflicts when:
- Two team members ran the model on different branches
- Both branches added new entries to the same JSON array
- Git couldn't automatically merge the array modifications

**Solution**: Each run is stored as an individual file with a UUID filename:
- UUIDv7 ensures time-ordered, globally unique identifiers
- No two branches can create the same **record** filename
- Git can merge branches without conflicts **in the record files themselves**
- `index.json` is a rebuildable cache and auto-regenerates from the individual record files. (Janus does **not** write a `.gitignore` into `.janus/`; if you do not want the index tracked, add `.janus/runlog/index.json` to your repository's ignore rules yourself.)

**This does not extend to the chain.** Hash-chaining (added after the directory
layout was designed) reintroduces a single shared mutable file — `head.json` —
and a single shared counter, the chain `sequence`. Two branches that each seal a
run will conflict on `head.json` and will both take the same sequence number, and
a merged log then carries a permanent duplicate-sequence break. See
[Known Limits](#known-limits). This is a real architectural consequence, not a
rough edge.

## How It Works

### Signing Flow

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│  Run Completes  │────▶│  Create Record   │────▶│  Sign Record    │
└─────────────────┘     └──────────────────┘     └─────────────────┘
                                                          │
                                                          ▼
                                               ┌─────────────────────┐
                                               │  Store with:        │
                                               │  - Signature        │
                                               │  - Public Key       │
                                               │  - Signer Email     │
                                               │  - Fingerprint      │
                                               │  - Timestamp        │
                                               └─────────────────────┘
```

### Verification Flow

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│  Load Record    │────▶│  Look Up Signer  │────▶│  Verify Against │
│                 │     │  Fingerprint in  │     │  Trust Store's  │
│                 │     │  the Trust Store │     │  Copy of the Key│
└─────────────────┘     └──────────────────┘     └─────────────────┘
                                                          │
                                                          ▼
                                               ┌─────────────────────┐
                                               │  ✓ Valid            │
                                               │  ✗ Invalid          │
                                               │  ✗ Untrusted        │
                                               │  ? Unsigned         │
                                               │  ? Unverifiable     │
                                               └─────────────────────┘
```

The record still carries a `signer_public_key` field, but verification never trusts it: a record's own embedded key is exactly what an attacker controls, so letting a record vouch for itself would let a forged record pass. The signer's `signer_fingerprint` is looked up in a `TrustStore` instead, and the signature is checked against *that* store's copy of the key — never the one on the record.

## What Gets Signed

The signature covers the **entire run record** at the time of signing, except for signature metadata fields.

### Signed Fields (tampering detected)

| Field | Description |
|-------|-------------|
| `id` | UUID string (e.g., `019377a8-4e2c-7f1a-8b3d-2c4e5f6a7b8c`) |
| `timestamp` | When the run started |
| `model_file` | Path to the model file |
| `command` | Full command executed |
| `exit_code` | Process exit code |
| `status` | Run status (completed/failed) |
| `is_parallel` | Whether parallel execution was used |
| `cores` | Number of cores used |
| `is_grid` | Whether grid execution was used |
| `nonmem_options` | Additional NONMEM options |
| `container` | Container provenance (image, digest, resources) |
| `stdout_compressed` | Captured standard output |
| `stderr_compressed` | Captured standard error |
| `description_compressed` | User-provided description |
| `embedded_files` | Output files (lst, ext, phi, xml, etc.) |
| `sequence` | This record's position in the hash chain |
| `prev_hash` | Hash of the previous sealed record — the chain link |
| `sealed` | Marks the record immutable; a sealed record can never be re-signed in place |

Because `sequence` and `prev_hash` are set *before* signing and are covered by the same signature as everything else, a record cannot be renumbered or re-linked without invalidating its own signature — see [Integrity Guarantees](#integrity-guarantees) for what this buys at the level of the whole log.

### Excluded from Signature

| Field | Reason |
|-------|--------|
| `signature` | The signature itself |
| `signer_public_key` | Added after signing |
| `signer_fingerprint` | Added after signing |
| `signer_email` | Added after signing |
| `signed_at` | Added after signing |

## Key Generation

Janus uses RSA-2048 keys for signing. Keys must be generated with OpenSSL (not ssh-keygen).

### Generate a Key Pair

```bash
# Generate private key (keep this secure!)
openssl genrsa -out ~/.config/janus/signing-key.pem 2048

# Extract public key (submit this with license request)
openssl rsa -in ~/.config/janus/signing-key.pem -pubout -out ~/.config/janus/signing-key.pub
```

### Key Storage

| File | Location | Purpose |
|------|----------|---------|
| Private Key | `~/.config/janus/signing-key.pem` | Signs run records (never shared) |
| Public Key | Embedded in JWT license | Verifies signatures |

## Configuration

### janus.yaml

```yaml
runlog:
  enabled: true
  signing:
    private_key: ~/.config/janus/signing-key.pem
```

### License Integration

When requesting a license, include your public key:

```bash
./scripts/generate-license.sh \
  --signing-key ~/.config/janus/signing-key.pub \
  --email user@company.com
```

The public key is embedded in the JWT license token and validated at startup.

## Multi-User Verification

### The Challenge

In team environments, multiple users may work on the same model over time. Each user has their own signing key pair. How can User B verify records signed by User A?

### The Solution

Each signed record includes the signer's public key:

```json
{
  "id": "019377a8-4e2c-7f1a-8b3d-2c4e5f6a7b8c",
  "timestamp": "2025-01-15T10:30:00Z",
  "model_file": "model.mod",
  "exit_code": 0,
  "signature": "base64-encoded-signature...",
  "signer_public_key": "-----BEGIN PUBLIC KEY-----\n...",
  "signer_fingerprint": "a1b2c3d4...",
  "signer_email": "alice@company.com",
  "signed_at": "2025-01-15T10:35:00Z"
}
```

The embedded `signer_public_key` is informational — it is what lets the UI display "Signed by alice@company.com" without a network call — but it is **not** what verification trusts. A record vouching for its own authenticity with its own embedded key is exactly what a forged record would also do. Verification instead:

1. Reads the record's `signer_fingerprint`
2. Looks that fingerprint up in a `TrustStore` (the commercial build anchors this in the license's signing key claim; the open-source build anchors it in a user- or org-maintained keyring — see `internal/runlog/trust.go`)
3. If the fingerprint is not found, the record is reported **Untrusted** — never Valid, no matter how cryptographically sound the signature is
4. If the fingerprint is found, the signature is verified against the trust store's own copy of the key, never the record's

This means **any team member can verify any record signed by a key their trust store recognizes**, regardless of who signed it — and a record signed by a key nobody authorized is flagged as such, not silently accepted.

### Current Limitation — read this before relying on the above

The trust store is the whole mechanism, and **the shipping commercial build populates it with exactly one key: the `SigningPublicKey` claim from the local license** (`appsetup.BuildTrustStore` → `runlog.NewLicenseTrust`). There is, today, **no supported way to add a second key**. `runlog.NewKeyringTrust` — the multi-key path — exists in the code but is wired to no configuration setting and is never called by the application.

Consequences you will actually hit:

- Bob opening a model directory Alice ran in sees the head reported as **not signed by a recognised key**, and Alice's records as **Untrusted**. Nothing is wrong with the log.
- The same happens to a **single user who rotates their signing key** — which [Security Considerations](#security-considerations) tells you to do after a compromise. Records sealed under the old key stop verifying.

Janus deliberately distinguishes this from tampering. An unrecognised key means *your trust store is incomplete*, not *the log is broken*: it produces a "RUN LOG NOT FULLY VERIFIED" warning, it does **not** claim the log is damaged, and — critically — it does **not** write a `KindIntegrityEvent` into the audit trail. Sealing an immutable record asserting "the log was found broken" because Janus does not happen to hold a key would be fabricating audit evidence. See `ChainReport.HasIntegrityBreak` in `internal/runlog/verify.go`.

Multi-key trust (a keyring the user or organization maintains) is the intended resolution and the code seam for it is in place; until it is wired up, treat multi-user and post-rotation verification as **not supported**.

## Verification Status

The run history table displays verification status for each record:

| Icon | Status | Meaning |
|------|--------|---------|
| ✓ | Valid | Signature verified against a key the trust store recognizes — record intact |
| ✗ | Invalid | Signer is trusted, but the signature does not check out — record may have been modified |
| ✗ | Untrusted | Signature is cryptographically sound, but the signing key's fingerprint is not in the trust store. Never rendered green — this is how a forged or unauthorized-key record announces itself |
| ? | Unsigned | Record has no signature |
| ? | Unverifiable | No trust anchor (license or keyring) is configured for this store at all — nothing can be checked yet |

### Status Details

**Valid (Green ✓)**
- Signature present
- Signer's fingerprint found in the trust store
- Signature verified against the trust store's copy of the key
- Display: "Signed by alice@company.com"

**Invalid (Red ✗)**
- Signature present, signer's fingerprint IS in the trust store
- Verification against the trust store's key failed
- Record has been modified after signing
- Display: "Signature invalid - record may have been modified"

**Untrusted (Red ✗)**
- Signature present and internally well-formed
- Signer's fingerprint is NOT in the trust store — covers both a forged/unauthorized key and a legacy record whose key was never registered
- Never displayed as green: a signature nobody authorized proves nothing about the record's integrity
- Display: "Signed by an UNAUTHORIZED key (...) — not trusted"

**Unsigned (Gray ?)**
- No signature field
- Signing was not configured when record was created
- Display: "Unsigned"

**Unverifiable (Gray ?)**
- Signature present
- No trust anchor configured for this store (no license, no keyring) — there is nothing to check the signer's fingerprint against yet
- This is a store-wide condition, not a per-record one: every record shows this way until a trust anchor is configured
- Display: "Signed (cannot verify — no trust anchor configured)"

## Integrity Guarantees

A signature on an individual record only proves that record was not altered. It says nothing about whether the *set* of records is complete — a signed record simply vanishing is invisible to a check that only ever looks at one record at a time. Chain verification (`VerifyChain` / `RunLogStore.VerifyIntegrity`, `internal/runlog/verify.go` and `chain.go`) is the piece that closes that gap: sealed records carry a monotonic `sequence` and a `prev_hash` linking each one to its predecessor, both covered by the record's own signature, and the chain's declared tip is itself signed in a separate `head.json` checkpoint.

### Detected

- **Modification** of a sealed record — detected twice over. Per record, its RSA-SHA256 signature no longer verifies. At the level of the *set*, the record's hash no longer matches the `prev_hash` its successor committed to. The **newest** sealed record has no successor, so nothing in the chain itself commits to it: it is covered instead by the `tip_hash` in the signed `head.json`, which `VerifyChain` compares against the actual hash of the highest sealed record. Without that comparison, editing the newest record would break only its own signature while the chain still reported itself intact.
- **Removal** of a sealed record from anywhere but the very end of the chain — a hole in the `sequence`/`prev_hash` linkage. The report names the missing sequence (e.g. "gap at sequence 17") and every surviving record still correctly reports Valid; a neighbour vanishing does not make the records next to it forgeries.
- **Duplication or insertion** — two records claiming the same `sequence` are detected and named, distinctly from a gap.
- **Reordering or renumbering** — the chain commits to order via `prev_hash`, so records cannot be relabeled or shuffled without breaking the link.
- **Forgery with an unauthorized key** — verification looks the signer's fingerprint up in a `TrustStore`; a record's own embedded key is never treated as authority for itself. A signature from a key nobody authorized reports **Untrusted**, which never renders green.
- **Truncation of the newest records** — a signed `head.json` checkpoint declares the expected chain length and tip hash. A pure hash chain by itself CANNOT detect this: a truncated chain is still perfectly self-consistent, which is exactly why the head checkpoint exists.
- **Corrupt (unreadable) record files** — reported as corrupt/unreadable, distinctly from a missing file, so an auditor is not told a file was deleted when it was merely damaged.
- **Its own tamper history**: a detected break is itself appended to the chain as a sealed, signed `KindIntegrityEvent` record (`RunLogStore.AppendIntegrityEvent`), so the log carries evidence that Janus detected the break, when, and under whose signing key — not merely that a gap currently exists.

Accidental deletion — the case auditors most often actually encounter, as opposed to a deliberate attack — is fully covered, including deletion of the newest record(s): an accidental `rm` does not also re-sign a shorter `head.json`, so it is caught by the tip check exactly like a deliberate truncation.

### Known Limits

- **Branching a model directory breaks the chain, permanently and unhealably. Know this before you branch.** The chain is a single linear structure with a single shared mutable checkpoint (`head.json`) and a single shared counter (`sequence`). Git's per-record-file merge safety does not extend to either:
  - Two branches that each seal a run both rewrite `head.json` → a **guaranteed merge conflict** on that file, every time.
  - Worse, both branches take the **same sequence number** (each computed it from the same pre-branch head). After the merge, the log permanently contains two records claiming one sequence — a duplicate-sequence break, which `VerifyIntegrity` correctly reports and which **cannot be repaired**: sealed records are immutable by design, so renumbering one of them is precisely the operation the audit trail exists to forbid.

  There is no automatic resolution and Janus will not invent one. Until the chain is made branch-aware, the supported model is **one linear run log per model directory**: do not seal runs on two branches of the same model directory and then merge them. Running on a branch is fine; *sealing runs on both sides of a fork and merging* is not.
- **Multi-user and post-key-rotation verification are not supported in the shipping build.** The trust store holds exactly one key (your license's), with no mechanism to add another, so a colleague's records — and your own records from before a key rotation — report as Untrusted rather than Valid. This is a *trust-scope* limitation, not an integrity finding: Janus says so in those words, and never writes an integrity event over it. See [Current Limitation](#current-limitation--read-this-before-relying-on-the-above).
- **An attacker holding both the signing private key and write access to the run-log directory can delete the newest records and re-sign a shorter head.** Nothing in a purely local audit log can prevent this — the head checkpoint is only as trustworthy as the key that signs it, and a compromised key can re-certify any history. Detecting this requires an external witness (a remote append-only log or an independent countersignature), which Janus does not currently implement.
- **The system can prove that a record was removed; it cannot explain why.** Malicious deletion, an operator's `rm -rf`, and a flaky sync client all produce the identical, indistinguishable signal: a gap or a tip mismatch. Making removal *evident* is the whole job of this feature; establishing the cause is necessarily a human, out-of-band task.
- Index.json (`internal/runlog/store.go`) is a rebuildable performance cache used only to answer *display* queries (the run list, pagination) quickly — it carries no trust of its own. `VerifyIntegrity` never reads it: `SealedRecords` enumerates the run-log **directory** itself and loads every non-reserved `*.json` record straight from disk. Editing, truncating, or deleting entries from the index therefore hides nothing — a removed record still shows up as a gap in the survivors' own signed `sequence`s, and a record file *added* to the directory but absent from the index is still enumerated, and reveals itself as a duplicate sequence or a tip beyond the one the signed head declares. Chain verification is anchored in each record's own signed `sequence`/`prev_hash` and in the signed head, not in the index.

## CFR 21 Part 11 Compliance

This implementation addresses key CFR 21 Part 11 requirements:

| Requirement | Implementation |
|-------------|----------------|
| **11.10(a)** Validation | Signature verification validates record integrity; chain verification validates that the record set is complete |
| **11.10(c)** Protection of records | Cryptographic signatures detect modification of a sealed record; hash chaining plus a signed head checkpoint detect deletion, insertion, duplication, reordering, and tail truncation of records against a trust anchor — see [Integrity Guarantees](#integrity-guarantees) for what is, and is not, covered |
| **11.10(e)** Audit trail | Execution metadata is signed and chained. Modification of a sealed record, and removal, insertion, duplication, reordering, or truncation of records from the log, are detected and named — the missing or duplicated sequence is reported, not just "something is wrong." This does **not** hold against an attacker who possesses both the signing private key and write access to the run-log directory; see [Known Limits](#known-limits) |
| **11.50** Signature manifestations | Signer identity (email) stored with signature |
| **11.70** Signature linking | Signature cryptographically bound to record content |
| **11.100** General requirements | RSA-2048 provides adequate security |

## Troubleshooting

### "Signing key pair validation failed"

The private key doesn't match the public key in your license.

**Solutions:**
1. Regenerate your key pair
2. Request a new license with the correct public key
3. Ensure you're using the same key pair submitted with your license

### "unsupported PEM block type"

You generated keys with `ssh-keygen` instead of OpenSSL.

**Solution:** Regenerate keys using OpenSSL commands above.

### Records showing "?" (Unverifiable)

No trust anchor (license or keyring) is configured for this store at all. This affects every record uniformly — it is not specific to legacy records.

**Solution:** Configure a license (commercial build) or a keyring (open-source build) as the trust anchor.

### Records showing "✗" (Untrusted) that you believe were legitimately signed

Once a trust anchor IS configured, a record whose signer fingerprint is not in it renders **Untrusted**, not a soft "?" — including old records signed before signer fingerprints were tracked, or by a key that has since been rotated out. This is deliberate: verification never falls back to a record's own embedded key, because that is exactly what a forged record would also supply.

**There is currently no action you can take.** The honest state of the software: the commercial build derives its trust store from the single `SigningPublicKey` claim in your license, and exposes **no** way to add the historical signer's key. `runlog.NewKeyringTrust` (multi-key) exists in the code but is not wired to any configuration setting. Advice to "add the key to your keyring" would be advice you cannot follow — see [Known Limits](#known-limits).

What Janus does instead is refuse to lie about it: an unrecognised key is reported as *"the key that signed this run log is not one this installation recognises"* — explicitly **not** as tampering — and no integrity event is written into the audit trail over it. If you do not recognize the signer *and* the chain also reports a genuine break (a gap, a duplicate, a modified record), treat that as a real finding.

## Security Considerations

1. **Protect your private key** - Anyone with your private key can sign records as you
2. **Key rotation** - If a key is compromised, request a new license with a new public key. Be aware of the cost: records sealed under the old key will report as Untrusted afterwards, because the trust store holds only the current license key. Janus reports this as a trust-scope warning, never as tampering, and writes nothing into the audit trail over it — but it will not verify green again until multi-key trust ships. See [Known Limits](#known-limits).
3. **Backup keys securely** - Lost private keys cannot be recovered
4. **Verify before trusting** - Always check the verification status in the UI
5. **A compromised key defeats the chain, not just individual records** - Someone holding your private key AND write access to the run-log directory can delete the newest sealed records and re-sign a shorter `head.json` that is internally consistent. This is a fundamental limit of any locally-verified log, not a bug — see [Known Limits](#known-limits). Protecting the private key is what protects the whole audit trail, not merely one signature.

## API Reference

### SignRecordWithInfo

```go
func SignRecordWithInfo(record *RunRecord, signer *signing.Signer, signerEmail string) error
```

Signs a run record with full signer provenance.

### VerifyRecordStatus

```go
func VerifyRecordStatus(record *RunRecord, trust TrustStore) VerificationResult
```

Returns detailed verification status for UI display. Verifies against the trust store's copy of the signer's key, looked up by fingerprint — never against the record's own embedded `signer_public_key`.

### VerifyChain

```go
func VerifyChain(records []*RunRecord, head *Head, trust TrustStore) ChainReport
```

Verifies the sealed records **as a set**: sequence continuity, no sequence claimed twice, intact `prev_hash` links, and a tip that matches the signed head. `records` must be sealed records sorted ascending by `Sequence`. See `RunLogStore.VerifyIntegrity`, which loads the sealed records and head from a store and calls this.

### VerificationResult

```go
type VerificationResult struct {
    Status  VerificationStatus  // Valid, Invalid, Untrusted, Unsigned, Unverifiable, ChainBroken
    Message string              // Human-readable description
    Signer  string              // Email of signer (if known)
}
```
