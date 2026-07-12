# License Key Management & JWT Validation Workflow

This document describes the complete workflow for managing RSA signing keys in the license-server and embedding public keys into Janus for JWT validation.

## Architecture Overview

```
┌──────────────────────┐         ┌──────────────────────┐
│  License Server      │         │  Janus GUI           │
│                      │         │                      │
│  • Generate RSA keys │         │  • Embedded public   │
│  • Sign JWTs         │◄────────┤    key (build time)  │
│  • Issue tokens      │  Token  │  • Validate JWTs     │
│  • Manage agreements │         │  • Extract claims    │
└──────────────────────┘         └──────────────────────┘
```

## Security Model

### Private Key Storage
- **Generated**: 4096-bit RSA keys via `crypto/rand` (cryptographically secure)
- **Encrypted**: AES-256-GCM with 32-byte key (from `LICENSING_ENCRYPTION_KEY` env var)
- **Stored**: Database column `private_key_encrypted` (base64-encoded ciphertext)
- **Usage**: Decrypted in-memory only when signing JWTs (never exposed via API)

### Public Key Distribution
- **Method**: Embedded directly into Janus binary at build time via ldflags
- **No Network**: Eliminates man-in-the-middle attacks
- **Immutable**: Cannot be changed after build without recompiling
- **Fallback**: Binary builds without key (dev mode), but license validation fails

## Complete Workflow

### Phase 1: License Server Setup

#### 1.1. Start License Server
```bash
# Make sure PostgreSQL is running
docker compose --profile dev up -d postgres

# Build and start license-server (auto-applies migrations)
mage license:build
./license-server serve
```

The server:
- Applies database migrations automatically
- Creates `signing_keys` table
- Listens on port 8443 (configurable via `LICENSING_PORT`)

#### 1.2. Generate Master Signing Key
```bash
# Option A: Using mage helper
mage license:generateMasterKey

# Option B: Direct API call
curl -X POST http://localhost:8443/api/v1/keys/rotate \
  -H "Content-Type: application/json" \
  -d '{
    "expires_in_days": 365
  }'
```

This generates:
- RSA-4096 keypair
- Key ID: `key-<timestamp>`
- Expires: 365 days from now
- Master key (not organization-specific)

**Database record created**:
```sql
INSERT INTO signing_keys (
  key_id,
  organization_id,  -- NULL for master keys
  public_key_pem,   -- Plaintext PEM
  private_key_encrypted,  -- AES-GCM encrypted
  algorithm,        -- RS256
  expires_at
)
```

#### 1.3. Export Public Key
```bash
mage license:exportPublicKey
```

This:
1. Queries `/api/v1/keys` for master keys (`organization_id = null`)
2. Fetches public key PEM via `/api/v1/keys/public?key_id=<key_id>`
3. Saves to `.license_public_key.pem` (git-ignored for security)

**File created**: `.license_public_key.pem`
```
-----BEGIN RSA PUBLIC KEY-----
MIICCgKCAgEA...
-----END RSA PUBLIC KEY-----
```

### Phase 2: Janus Build with Embedded Key

#### 2.1. Build Janus with Public Key
```bash
# Build automatically embeds .license_public_key.pem if present
mage build
```

Build process:
1. Checks for `.license_public_key.pem`
2. If found: Reads PEM, escapes newlines, injects via ldflags
3. If not found: Builds without key (dev mode warning)

**ldflags injection**:
```bash
-X "github.com/pharmalytica/janus/internal/license/publickey.EmbeddedPublicKey=-----BEGIN RSA PUBLIC KEY-----\nMIICC..."
```

The public key is now **compiled into the binary** at:
- `internal/license/publickey.EmbeddedPublicKey` (string variable)

#### 2.2. Verify Key Embedding
```bash
# Check if key is embedded (without exposing it)
./janus --check-license-key  # TODO: Add this flag

# Or check programmatically
go run -ldflags "..." . <<EOF
package main
import "github.com/pharmalytica/janus/internal/license/publickey"
func main() {
    println(publickey.GetKeyInfo())
}
EOF
```

### Phase 3: JWT Token Generation & Validation

#### 3.1. Create Organization & Agreement
```bash
# Create organization
curl -X POST http://localhost:8443/api/v1/organizations \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Acme Pharmaceuticals",
    "contact_name": "John Smith",
    "email": "john@acme.com"
  }'
# Returns: {"id": 1, ...}

# Create agreement with features
curl -X POST http://localhost:8443/api/v1/agreements \
  -H "Content-Type: application/json" \
  -d '{
    "organization_id": 1,
    "tier": "enterprise",
    "max_seats": 50,
    "features": ["audit", "grid", "validation"],
    "cost_per_month": 5000.00,
    "start_date": "2025-01-01",
    "end_date": "2025-12-31"
  }'
# Returns: {"id": 1, ...}
```

#### 3.2. Generate JWT Token
```bash
curl -X POST http://localhost:8443/api/v1/tokens \
  -H "Content-Type: application/json" \
  -d '{
    "agreement_id": 1,
    "user_email": "scientist@acme.com",
    "duration_seconds": 2592000
  }'
```

**Response**:
```json
{
  "token": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_at": "2025-11-01T00:00:00Z"
}
```

**JWT Claims (signed with private key)**:
```json
{
  "agreement_id": 1,
  "organization_id": 1,
  "tier": "enterprise",
  "features": ["audit", "grid", "validation"],
  "max_seats": 50,
  "user_email": "scientist@acme.com",
  "exp": 1730419200,
  "iat": 1727827200,
  "jti": "uuid-here"
}
```

#### 3.3. Validate JWT in Janus
```go
import (
    "github.com/pharmalytica/janus/internal/license/validator"
)

// Initialize validator (reads embedded public key)
v, err := validator.NewValidator()
if err != nil {
    // No public key embedded - dev mode
}

// Validate token from user
claims, err := v.ValidateToken(tokenString)
if err != nil {
    // Invalid signature, expired, etc.
}

// Check features
if claims.HasFeature("audit") {
    // Enable audit logging
}
if claims.HasFeature("grid") {
    // Enable SLURM integration
}
```

## Key Rotation

### Rotating Keys
```bash
# Generate new key (old key remains valid until expiration)
curl -X POST http://localhost:8443/api/v1/keys/rotate \
  -H "Content-Type: application/json" \
  -d '{"expires_in_days": 365}'

# Export new public key
mage license:exportPublicKey

# Rebuild Janus with new key
mage build

# Distribute new binary to users
```

**Grace Period**: Old tokens remain valid until their expiration. New tokens signed with new key.

### Emergency Revocation
```sql
-- Revoke a compromised key immediately
UPDATE signing_keys
SET revoked_at = NOW()
WHERE key_id = 'key-12345';
```

All tokens signed with that key become immediately invalid.

## Mage Targets Reference

### License Server
- `mage license:build` - Build license-server binary
- `mage license:serve` - Start license-server
- `mage license:seed` - Seed test data (organizations + agreements)
- `mage license:generateMasterKey` - Generate master signing key
- `mage license:exportPublicKey` - Export public key to `.license_public_key.pem`

### Janus
- `mage build` - Build Janus (auto-embeds public key if present)
- `mage release v1.0.0` - Build release (requires public key)

## File Structure

```
janus/
├── .license_public_key.pem          # Git-ignored, generated by exportPublicKey
├── .env                             # License server config (encryption key)
├── cmd/
│   └── license-server/
│       ├── seed_data.json           # Test organizations/agreements
│       └── README.md                # License server docs
├── internal/
│   └── license/
│       ├── publickey/
│       │   └── embedded.go          # Public key embedding logic
│       ├── validator/
│       │   └── validator.go         # JWT validation
│       ├── keys/
│       │   ├── manager.go           # Key generation & storage
│       │   └── encryptor.go         # AES-GCM encryption
│       └── jwt/
│           └── generator.go         # JWT token generation
└── magefiles/
    ├── license.go                   # License server mage targets
    └── license_key.go               # Key management mage targets
```

## Security Considerations

### ✅ What This Protects Against
- **MITM Attacks**: No network fetch of public key
- **Key Tampering**: Embedded at compile time, immutable
- **Private Key Exposure**: Encrypted at rest, decrypted only in memory
- **Unauthorized Tokens**: Only license-server can sign valid tokens

### ⚠️ What You Must Protect
- **LICENSING_ENCRYPTION_KEY**: 32-byte AES key (never commit to git)
- **License Server Database**: Contains encrypted private keys
- **License Server Access**: Restrict who can generate tokens
- **.license_public_key.pem**: Git-ignored, but not sensitive (public key)

### 🚫 Don't Do This
- ❌ Commit encryption keys to version control
- ❌ Expose license-server to public internet without authentication
- ❌ Reuse the same encryption key across environments
- ❌ Build production Janus without embedded public key

## GitHub Actions / CI/CD Setup

### Setting Up GitHub Secrets

For CI/CD builds to include license validation, you need to add the public key as a GitHub secret:

1. **Export the public key locally:**
   ```bash
   mage license:exportPublicKey
   # This creates .license_public_key.pem
   ```

2. **Add as GitHub Secret:**
   - Go to repository Settings → Secrets and variables → Actions
   - Click "New repository secret"
   - Name: `LICENSE_PUBLIC_KEY`
   - Value: Paste the **entire contents** of `.license_public_key.pem` (including the `-----BEGIN PUBLIC KEY-----` and `-----END PUBLIC KEY-----` lines)

3. **Workflow Integration:**
   The CI/CD workflows automatically detect this secret and embed it during builds:

   ```yaml
   - name: Setup license public key
     if: ${{ secrets.LICENSE_PUBLIC_KEY != '' }}
     shell: bash
     run: |
       echo "${{ secrets.LICENSE_PUBLIC_KEY }}" > .license_public_key.pem
   ```

### How It Works in CI/CD

- **CI Workflow (`.github/workflows/ci.yml`)**: Tests run with or without the key (optional)
- **Release Workflow (`.github/workflows/release.yml`)**: Automatically embeds public key if `LICENSE_PUBLIC_KEY` secret exists
- **Build Process**: The release build script checks for `.license_public_key.pem` and includes it in ldflags
- **Graceful Degradation**: If no secret is set, builds succeed but without license validation

### When to Update the Secret

Update the `LICENSE_PUBLIC_KEY` GitHub secret whenever you:
- Rotate the master key on license-server
- Switch to a new license-server instance
- Upgrade/change the key algorithm

**Important**: After updating the secret, all new CI builds will automatically use the new public key. Existing released binaries continue using their embedded key until rebuilt.

## Troubleshooting

### "No public key embedded" Error
```bash
# Generate and export key
mage license:generateMasterKey
mage license:exportPublicKey

# Rebuild Janus
mage build
```

### "Failed to validate token" Error
- Check token hasn't expired: `jwt.io` decoder
- Verify public key matches private key used to sign
- Ensure Janus was built with correct public key

### Key Rotation Not Working
- Export new public key: `mage license:exportPublicKey`
- Rebuild Janus: `mage build`
- Old tokens remain valid until expiration
- Check key ID in JWT header matches active key

## Future Enhancements

- [ ] JWKS endpoint for dynamic key discovery (optional)
- [ ] Multiple active keys for smooth rotation
- [ ] Key versioning in JWT header (`kid` claim)
- [ ] Automated key rotation schedule
- [ ] Hardware security module (HSM) integration
