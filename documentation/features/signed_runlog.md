# Signed Run Log

## Overview

Janus implements cryptographic signing of run log entries to ensure audit trail integrity for CFR 21 Part 11 compliance. Each execution record is signed with an RSA-SHA256 signature, providing:

- **Tamper Evidence**: Any modification to a signed record is detectable
- **Non-Repudiation**: Signatures are tied to specific users via their license
- **Multi-User Support**: Records can be verified by any team member, regardless of who signed them
- **Merge Conflict Prevention**: UUID-based directory storage eliminates Git conflicts

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
        └── index.json                                   # Auto-generated index (gitignored)
```

### Why Directory-Based Storage?

The previous single-file approach (`model.janus_history.json`) caused merge conflicts when:
- Two team members ran the model on different branches
- Both branches added new entries to the same JSON array
- Git couldn't automatically merge the array modifications

**Solution**: Each run is stored as an individual file with a UUID filename:
- UUIDv7 ensures time-ordered, globally unique identifiers
- No two branches can create the same filename
- Git can merge branches without conflicts
- Index file is gitignored and auto-regenerates from individual files

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
│  Load Record    │────▶│  Extract Signer  │────▶│  Verify Against │
│                 │     │  Public Key      │     │  Embedded Key   │
└─────────────────┘     └──────────────────┘     └─────────────────┘
                                                          │
                                                          ▼
                                               ┌─────────────────────┐
                                               │  ✓ Valid            │
                                               │  ✗ Invalid          │
                                               │  ? Unverifiable     │
                                               └─────────────────────┘
```

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

When verifying:
1. Extract the embedded public key from the record
2. Verify the signature against that key
3. If valid, the record hasn't been tampered with

This means **any team member can verify any record**, regardless of who signed it.

## Verification Status

The run history table displays verification status for each record:

| Icon | Status | Meaning |
|------|--------|---------|
| ✓ | Valid | Signature verified successfully |
| ✗ | Invalid | Signature verification failed - record may have been modified |
| ? | Unsigned | Record has no signature |
| ? | Unverifiable | Signed but no public key available (legacy records) |

### Status Details

**Valid (Green ✓)**
- Signature present
- Embedded public key available
- Verification succeeded
- Display: "Signed by alice@company.com"

**Invalid (Red ✗)**
- Signature present
- Verification failed
- Record has been modified after signing
- Display: "Signature invalid - record may have been modified"

**Unsigned (Gray ?)**
- No signature field
- Signing was not configured when record was created
- Display: "Unsigned"

**Unverifiable (Gray ?)**
- Signature present
- No embedded public key (legacy record)
- No fallback key available
- Display: "Signed by Unknown (cannot verify - no public key)"

## CFR 21 Part 11 Compliance

This implementation addresses key CFR 21 Part 11 requirements:

| Requirement | Implementation |
|-------------|----------------|
| **11.10(a)** Validation | Signature verification validates record integrity |
| **11.10(c)** Protection of records | Cryptographic signatures detect unauthorized modifications |
| **11.10(e)** Audit trail | Complete execution metadata is signed and tamper-evident |
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

### Records showing "?" but were signed

Legacy records signed before signer provenance was added don't have embedded public keys.

**Solution:** No action needed. New records will include full signer provenance.

## Security Considerations

1. **Protect your private key** - Anyone with your private key can sign records as you
2. **Key rotation** - If a key is compromised, request a new license with a new public key
3. **Backup keys securely** - Lost private keys cannot be recovered
4. **Verify before trusting** - Always check the verification status in the UI

## API Reference

### SignRecordWithInfo

```go
func SignRecordWithInfo(record *RunRecord, signer *signing.Signer, signerEmail string) error
```

Signs a run record with full signer provenance.

### VerifyRecordStatus

```go
func VerifyRecordStatus(record *RunRecord, fallbackPublicKeyPEM string) VerificationResult
```

Returns detailed verification status for UI display.

### VerificationResult

```go
type VerificationResult struct {
    Status  VerificationStatus  // Valid, Invalid, Unsigned, Unverifiable
    Message string              // Human-readable description
    Signer  string              // Email of signer (if known)
}
```
