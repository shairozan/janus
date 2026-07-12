# Janus Licensing Architecture

## Overview

Janus uses a JWT-based token licensing system with tiered pricing. Licenses are validated using embedded public keys with optional online validation for renewal and revocation.

## Organizational storage
Customers will be stored as organizations in a postgres database with a big int ID, a name, created, and nullable deactivated dates. Licenses will be stored in a separate table listing bigint IDs, names, dates similarly as well as MSRP. A join table called "agreements" will track organization binding to licenses. Each entry in this table having a nullable "deactivated date". 

## Pricing Tiers

| Tier | Price/Seat/Month | Features |
|------|------------------|----------|
| Basic | $20 | Local execution only |
| Professional | $30 | Local execution + Grid support (SLURM/SGE) |
| Enterprise | $100 | Full feature set including Audit engine |

## Token Structure

Licenses are distributed as JWT tokens with the following claims structure:

```json
{
  "iss": "https://license.janus.io",
  "sub": "user@company.com",
  "aud": "customer-12345",
  "tier": "enterprise",
  "features": ["local", "grid", "audit"],
  "seats": 50,
  "exp": 1735689600,
  "iat": 1704067200,
  "jti": "unique-token-id",
  "org_name": "Acme Biotech"
}
```

### Claim Definitions

- **iss** (issuer): Janus licensing server URL
- **sub** (subject): Licensed user email
- **aud** (audience): Customer identifier (immutable, used for validation)
- **tier**: Pricing tier (`basic`, `professional`, `enterprise`)
- **features**: Array of enabled features (`local`, `grid`, `audit`)
- **seats**: Number of licensed seats for the organization
- **exp** (expiration): Token expiration timestamp
- **iat** (issued at): Token issuance timestamp
- **jti** (JWT ID): Unique token identifier for tracking/revocation
- **org_name**: Human-readable organization name for display

## Validation Strategy

### Hybrid Approach

Janus implements a tiered validation strategy combining offline and online validation:

1. **Primary: Offline validation** with embedded public key
   - Fast, no network dependency
   - Public key embedded at compile time using Go's `embed` directive
   - Allows operation in air-gapped environments

2. **Secondary: Online validation** via JWKS endpoint
   - Used for key rotation and real-time revocation
   - Non-blocking, cached validation
   - Falls back gracefully if offline

### Validation Flow

```go
func validateLicense(tokenString string) (*Claims, error) {
    // 1. Quick offline validation with embedded key
    claims, err := validateOffline(tokenString)
    if err != nil {
        return nil, err
    }
    
    // 2. Verify audience matches customer ID
    if claims.Audience != expectedCustomerID {
        return nil, errors.New("license not valid for this customer")
    }
    
    // 3. Check if token is near expiration (< 7 days)
    if isNearExpiration(claims.ExpiresAt) {
        go renewLicenseAsync() // Background renewal
    }
    
    // 4. Periodic online validation (cached, daily)
    if shouldCheckOnline() {
        go validateOnlineAsync(tokenString) // Non-blocking
    }
    
    return claims, nil
}
```

## License File Locations

Janus searches for license files in the following order:

1. Environment variable: `JANUS_LICENSE` (path to license file or token string)
2. User directory: `~/.janus/license.jwt`
3. System directory: `/etc/janus/license.jwt`

## Feature Gating

Features are enabled based on the `features` claim in the token:

```go
func isFeatureEnabled(feature string) bool {
    claims := getCurrentLicense()
    for _, f := range claims.Features {
        if f == feature {
            return true
        }
    }
    return false
}

// Usage
if !isFeatureEnabled("grid") {
    return errors.New("Grid execution requires Professional tier or higher. Visit https://janus.io/upgrade")
}
```

## JWKS Endpoint

The licensing server exposes a JSON Web Key Set for online validation:

```
GET https://license.janus.io/.well-known/jwks.json
```

Response format:
```json
{
  "keys": [
    {
      "kid": "2024-01-key",
      "kty": "RSA",
      "alg": "RS256",
      "use": "sig",
      "n": "...",
      "e": "AQAB"
    }
  ]
}
```

JWT tokens include a `kid` header to identify which key was used for signing.

## License Renewal

### Automatic Renewal Flow

1. Janus detects token expiring within 7 days
2. Makes authenticated request to renewal endpoint:
   ```
   POST /api/v1/license/renew
   Authorization: Bearer <current-token>
   ```
3. Server validates:
   - Token signature is valid
   - Customer subscription is active
   - Payment method is current
   - Extracts customer ID from `aud` claim
4. If valid: generates new token with extended expiration
5. Janus writes new token to license file
6. User notified of successful renewal

### Manual Renewal

```bash
$ janus license renew
Checking license status...
License renewed successfully. Valid until: 2025-10-29
```

## Installation and Setup

### Initial License Installation

```bash
# Install license token
$ janus license install --token=<jwt>

# Janus automatically extracts customer ID from aud claim
# Validates token signature and expiration
# Writes to ~/.janus/license.jwt
```

### Verification

```bash
$ janus license status
Organization: Acme Biotech
Customer ID: customer-12345
Tier: Enterprise
Licensed Seats: 50
Features: local, grid, audit
Expires: 2025-10-29
Status: Active
```

## Key Rotation Strategy

- Each Janus binary embeds 2-3 public keys (current + previous versions)
- Allows smooth transition during key rotation
- Falls back to JWKS endpoint if embedded keys fail to validate
- Recommended rotation schedule: annually or on security events

## Licensing Models

Janus supports two licensing models to accommodate different organizational needs:

### Per-Seat Model (Cloud-Connected)

Best for cloud-native organizations, smaller teams, and those wanting detailed usage analytics.

**Characteristics:**
- Each user has an individual token with their email in `sub` claim
- Janus phones home on execution to track active users
- Billing based on unique active users per billing period
- Requires internet connectivity
- Self-service portal for user token generation

**Token Example:**
```json
{
  "iss": "https://license.janus.io",
  "sub": "jane.doe@acme.com",
  "aud": "customer-12345",
  "tier": "enterprise",
  "features": ["local", "grid", "audit"],
  "license_model": "per-seat",
  "seats": 50,
  "exp": 1735689600,
  "iat": 1704067200,
  "jti": "unique-token-id",
  "org_name": "Acme Biotech"
}
```

### Concurrent User Model (Air-Gapped)

Best for air-gapped environments, large organizations with many occasional users, and compliance-sensitive deployments.

**Characteristics:**
- Organization-wide token(s)
- Customer runs local license server (provided by Janus)
- License server enforces concurrent user limits
- Janus reports usage to local license server
- Customer exports usage data for billing reconciliation
- Works completely offline
- **Local token generation/refresh capability** using customer-specific signing keys

**Token Example:**
```json
{
  "iss": "https://license.janus.io",
  "sub": "license-server@acme.com",
  "aud": "customer-12345",
  "tier": "enterprise",
  "features": ["local", "grid", "audit"],
  "license_model": "concurrent",
  "concurrent_limit": 50,
  "license_server_url": "https://license.acme.internal",
  "exp": 1735689600,
  "iat": 1704067200,
  "jti": "unique-token-id",
  "org_name": "Acme Biotech",
  "key_id": "2025-q1"
}
```

## Air-Gapped Token Management

For customers operating in air-gapped or GxP-compliant environments, Janus provides a sophisticated key management system that enables local token generation while maintaining payment enforcement.

### Initial Setup

When a customer purchases a concurrent license:

1. **Janus generates customer-specific key pair**:
   - RSA 4096-bit signing key pair
   - Key ID based on quarter (e.g., "2025-q1")
   - Private key for customer's license server
   - Public key embedded in Janus binaries

2. **Customer receives secure package**:
   - Initial organization token (90-day validity)
   - Private signing key (PEM format, encrypted)
   - License server binary or Docker image
   - Setup documentation

3. **Customer deploys license server**:
   ```bash
   $ janus-license-server init \
     --private-key=/secure/path/signing-key.pem \
     --customer-id=customer-12345 \
     --concurrent-limit=50
   ```

### Local Token Generation

The customer's license server can generate and refresh tokens locally without internet connectivity:

```bash
# Generate token for new user
$ janus-license-server token generate \
  --user jane.doe@acme.com \
  --duration 90d \
  --output /tokens/jane.jwt

# Refresh expiring token
$ janus-license-server token refresh \
  --token-file /tokens/jane.jwt \
  --extend 90d

# Batch refresh for all active users
$ janus-license-server token refresh-all \
  --duration 90d
```

**Server-Side Token Generation:**
```go
func (ls *LicenseServer) GenerateUserToken(email string) (string, error) {
    claims := jwt.MapClaims{
        "iss": "https://license.janus.io",
        "sub": email,
        "aud": ls.CustomerID,
        "tier": ls.Tier,
        "features": ls.Features,
        "license_model": "concurrent",
        "concurrent_limit": ls.ConcurrentLimit,
        "license_server_url": ls.ServerURL,
        "exp": time.Now().Add(90 * 24 * time.Hour).Unix(),
        "iat": time.Now().Unix(),
        "jti": uuid.New().String(),
        "org_name": ls.OrgName,
        "key_id": ls.KeyID, // e.g., "2025-q1"
    }
    
    token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
    token.Header["kid"] = ls.KeyID
    
    // Sign with customer's private key
    return token.SignedString(ls.privateKey)
}
```

### Key Rotation and Payment Enforcement

Annual key rotation provides the payment enforcement mechanism while maintaining operational continuity:

**Normal Operation (Customer Current on Payments):**

```
Q4 2024: Customer operates with "2024-q4" keys
    ↓
Q1 2025: Janus sends new "2025-q1" key pair
    ↓
Customer updates license server with new keys
    ↓
Q2 2025: Janus v2.5 ships with embedded public keys:
         - "2025-q1" (current)
         - "2024-q4" (grace period)
    ↓
Q3 2025: Tokens signed with either key work
    ↓
Q4 2025: Janus v2.8 drops "2024-q4" key
         - Only accepts "2025-q1" tokens
```

**Non-Payment Scenario (Customer Stops Paying):**

```
Q4 2024: Customer operating with "2024-q4" keys
    ↓
Q1 2025: Customer doesn't renew - NO new keys sent
    ↓
Q2 2025: Customer can still generate tokens locally
         - License server signs with "2024-q4" key
         - Janus v2.4 and earlier accept these tokens
    ↓
Q3 2025: Janus v2.6 released - still accepts "2024-q4" (grace period)
    ↓
Q4 2025: Janus v2.8 released - only accepts "2025-q1" keys
         - Customer's locally-generated tokens rejected
         - Customer must either:
           * Stay on old Janus version (no updates/security patches)
           * Pay subscription to receive new keys
    ↓
Result: Max 90-day token validity + 6-9 month grace period before hard stop
```

### Janus Multi-Key Validation

Janus embeds multiple public keys to support smooth rotation:

```go
// Embed multiple keys for rotation grace period
//go:embed keys/2025-q1.pem
var publicKey2025Q1 string

//go:embed keys/2024-q4.pem
var publicKey2024Q4 string

var publicKeys = map[string]*rsa.PublicKey{
    "2025-q1": parseKey(publicKey2025Q1),
    "2024-q4": parseKey(publicKey2024Q4),
}

func validateLicense(tokenString string) (*Claims, error) {
    // Parse token to extract key ID from header
    token, _ := jwt.Parse(tokenString, nil)
    kid, ok := token.Header["kid"].(string)
    if !ok {
        return nil, errors.New("missing key ID in token header")
    }
    
    // Lookup appropriate public key
    publicKey, exists := publicKeys[kid]
    if !exists {
        return nil, fmt.Errorf("unknown key version '%s' - please update Janus or renew license", kid)
    }
    
    // Validate token signature
    claims := &Claims{}
    token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
        if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
        }
        return publicKey, nil
    })
    
    if err != nil {
        return nil, fmt.Errorf("token validation failed: %w", err)
    }
    
    if !token.Valid {
        return nil, errors.New("invalid token")
    }
    
    return claims, nil
}
```

### Key Distribution Security

**Initial Key Delivery:**
- Private keys delivered via secure channel (encrypted email, secure file transfer)
- Keys encrypted with customer-provided password/passphrase
- Key fingerprint sent separately for verification

**Key Storage:**
- License server stores private key in secure location
- File permissions restricted to service account only (400)
- Optional: HSM integration for high-security environments
- Key encrypted at rest with customer-managed passphrase

**Key Rotation Delivery:**
- New keys sent 60 days before required upgrade
- Multiple delivery attempts via different channels
- Customer confirms receipt and deployment
- Old keys continue working during transition period

### GxP Compliance Benefits

This approach provides ideal characteristics for validated environments:

✅ **No Internet Dependency**: All operations local after initial setup  
✅ **Continuous Operation**: No phone-home required for day-to-day use  
✅ **Audit Trail Integrity**: All usage data stays within validated environment  
✅ **Change Control**: Key updates are planned, validated deployments  
✅ **Payment Enforcement**: Non-payment results in eventual operational cessation  
✅ **Graceful Degradation**: 6-12 month runway from non-payment to hard stop  
✅ **Version Control**: Customers control Janus upgrade timeline  
✅ **Data Sovereignty**: No customer data leaves the facility

### License Server Key Management Commands

```bash
# Check current key status
$ janus-license-server key status
Key ID: 2025-q1
Valid Until: 2025-12-31
Tokens Generated: 47
Active Tokens: 42

# Import new key rotation
$ janus-license-server key rotate \
  --new-key=/secure/2025-q2-key.pem \
  --effective-date=2025-04-01

# Re-sign all active tokens with new key
$ janus-license-server token resign-all \
  --key-id=2025-q2 \
  --preserve-expiration

# Export key fingerprint for verification
$ janus-license-server key fingerprint
SHA256: a3:4f:8b:2c:...
```

## Usage Tracking

### Per-Seat Model: Cloud Tracking

Janus tracks usage by sending events to the licensing server on key actions:

```go
func recordUsage(action string) error {
    claims := getCurrentLicense()
    
    payload := UsageEvent{
        CustomerID: claims.Audience,
        UserEmail: claims.Subject,
        Action: action,  // "model_load", "execution_start", "execution_complete"
        Timestamp: time.Now(),
        JTI: claims.JTI,
        Version: janusVersion,
        ExecutionID: generateExecutionID(), // For idempotency
    }
    
    // Non-blocking, fire-and-forget with retry
    go sendUsageEvent(payload)
    
    return nil
}
```

**API Endpoint:**
```
POST /api/v1/usage/track
Authorization: Bearer <license-token>
Content-Type: application/json

{
  "action": "execution_start",
  "timestamp": "2025-09-29T14:30:00Z",
  "execution_id": "exec-abc123",
  "metadata": {
    "model": "protein-folding-v2",
    "grid_backend": "slurm"
  }
}
```

**Server-Side Validation:**
```go
func handleUsageTracking(w http.ResponseWriter, r *http.Request) {
    token := extractBearerToken(r)
    
    // 1. Validate token signature and claims
    claims, err := validateToken(token)
    if err != nil {
        return unauthorized(w, "Invalid token")
    }
    
    // 2. Check token hasn't been revoked
    if isRevoked(claims.JTI) {
        return unauthorized(w, "Token revoked")
    }
    
    // 3. Rate limit per JTI (token instance)
    // Allow reasonable burst: 1000 requests per hour per token
    if !rateLimiter.Allow(claims.JTI, 1000, time.Hour) {
        return tooManyRequests(w, "Rate limit exceeded")
    }
    
    // 4. Validate request payload matches token claims
    var event UsageEvent
    json.NewDecoder(r.Body).Decode(&event)
    
    if event.UserEmail != claims.Subject {
        return forbidden(w, "User email mismatch")
    }
    
    if event.CustomerID != claims.Audience {
        return forbidden(w, "Customer ID mismatch")
    }
    
    // 5. Validate timestamp (reject events too far in past/future)
    if !isTimestampValid(event.Timestamp) {
        return badRequest(w, "Invalid timestamp")
    }
    
    // 6. Record usage (idempotent based on execution_id)
    recordUsage(claims.Audience, claims.Subject, event)
    
    return ok(w)
}
```

**Backend Tracks:**
- Unique active users per billing period
- Usage patterns per customer and user
- Feature utilization rates
- Peak usage times
- Execution counts and durations

### Concurrent Model: Local License Server

Customer runs a local license server (Docker image or binary provided by Janus):

```bash
$ janus-license-server start \
  --master-token=<org-token-file> \
  --port=8443 \
  --max-concurrent=50 \
  --data-dir=/var/lib/janus-license
```

**Janus Configuration:**
```bash
$ janus config set license-server https://license.acme.internal:8443
```

**License Lease Flow:**

```go
// On execution start - acquire lease
func acquireLicense() (*Lease, error) {
    resp, err := http.Post(
        licenseServerURL + "/api/lease/acquire",
        "application/json",
        json.Marshal(LeaseRequest{
            UserEmail: getUserEmail(),      // From system or config
            MachineID: getMachineID(),
            HostName: getHostName(),
            Action: "execution_start",
        }),
    )
    
    if err != nil {
        return nil, fmt.Errorf("failed to acquire license: %w", err)
    }
    
    // Server returns lease with expiration
    // { "lease_id": "lease-abc123", "expires_at": "2025-09-29T15:30:00Z" }
    var lease Lease
    json.NewDecoder(resp.Body).Decode(&lease)
    
    // Start heartbeat to keep lease alive
    go maintainLease(lease.ID)
    
    return &lease, nil
}

// On execution complete or timeout - release lease
func releaseLicense(leaseID string) error {
    http.Post(
        licenseServerURL + "/api/lease/release",
        "application/json",
        json.Marshal(LeaseRelease{
            LeaseID: leaseID,
            Timestamp: time.Now(),
        }),
    )
}
```

**License Server Responsibilities:**
- Enforce concurrent user limits
- Track active leases (who, what, when)
- Queue requests when limit reached
- Automatic lease expiration and cleanup
- Usage log generation for billing

### Billing Reconciliation (Concurrent Model)

Monthly usage export from customer's license server:

```bash
$ janus-license-server export --month=2025-09 --output=usage-2025-09.json
```

**Export Format:**
```json
{
  "customer_id": "customer-12345",
  "period": "2025-09",
  "license_model": "concurrent",
  "concurrent_limit": 50,
  "usage_summary": {
    "unique_users": 47,
    "total_executions": 1284,
    "peak_concurrent": 48,
    "average_concurrent": 32.5,
    "total_hours": 15680
  },
  "detailed_logs": [
    {
      "timestamp": "2025-09-01T08:15:00Z",
      "user": "jane.doe@acme.com",
      "machine": "workstation-042",
      "action": "lease_acquired",
      "lease_id": "lease-001"
    },
    {
      "timestamp": "2025-09-01T10:30:00Z",
      "user": "jane.doe@acme.com",
      "machine": "workstation-042",
      "action": "lease_released",
      "lease_id": "lease-001",
      "duration_seconds": 8100
    }
  ],
  "signature": "base64-encoded-hmac-sha256",
  "export_timestamp": "2025-10-01T00:00:00Z"
}
```

Customer transmits this signed file via:
- Email to billing@janus.io
- Upload to customer portal
- Automated API submission (if internet available)

Janus validates signature and bills accordingly.

## API Authentication

### Usage Tracking Authentication

The usage tracking API uses the license token itself for authentication, providing a simple yet secure approach:

**Why Use License Token:**
- ✅ Cryptographic proof via signature validation
- ✅ Identity embedded in claims (sub, aud)
- ✅ Revocation capability via JTI blacklist
- ✅ Built-in expiration
- ✅ Simple to implement and debug
- ✅ No additional credential management needed

**Security Measures:**

1. **Token Signature Validation**: Server validates JWT signature using public key
2. **Revocation Check**: JTI claim checked against revocation blacklist
3. **Rate Limiting**: 1000 requests per hour per JTI prevents abuse
4. **Claim Validation**: User email and customer ID in payload must match token claims
5. **Timestamp Validation**: Reject events with timestamps too far in past/future (±15 minutes)
6. **Idempotency**: Execution IDs deduplicate repeated submissions
7. **Payload Limits**: Maximum 10KB metadata to prevent abuse

**Note on Alternative Approaches:**

We considered x509 client certificates (mTLS) but determined this would be over-engineering:
- Requires CA infrastructure
- Complex certificate distribution and rotation
- Difficult troubleshooting
- Provides same fundamental protection as signed JWTs

The JWT approach provides equivalent security with significantly less operational complexity.

## Security Considerations

1. **Token Storage**: License files should have restricted permissions (600)
2. **Transport**: All API calls use HTTPS
3. **Token Lifetime**: 90-day expiration with 7-day renewal window
4. **Revocation**: JTI-based blacklist for immediate revocation
5. **Audience Validation**: Always validate `aud` claim matches expected customer ID
6. **Grace Period**: 7-day grace period after expiration with warnings before hard cutoff
7. **Rate Limiting**: Per-token rate limits prevent API abuse
8. **Monitoring**: Alert on unusual usage patterns (sudden spikes, geographic anomalies)
9. **Signature Verification**: Export files from license servers include HMAC signatures to prevent tampering

## Implementation Phases

### Phase 1: MVP
- Embedded public key validation
- Basic feature gating
- Manual license installation

### Phase 2: Enhanced
- Automatic renewal
- JWKS endpoint integration
- License status dashboard

### Phase 3: Enterprise
- Floating license server for concurrent user licensing
- Advanced seat management
- Usage analytics and telemetry
- Real-time revocation support