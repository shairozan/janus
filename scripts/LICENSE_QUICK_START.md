# License Server Quick Start Guide

## Prerequisites

1. **PostgreSQL running** (via Docker Compose):
   ```bash
   docker compose --profile dev up -d postgres
   ```

2. **Environment variables set**:
   ```bash
   export LICENSING_DATABASE_URL="postgres://janus:janus_dev_password@localhost:5432/janus_license?sslmode=disable"
   export LICENSING_ENCRYPTION_KEY="12345678901234567890123456789012"  # 32 characters for dev
   ```

3. **jq installed** (for JSON parsing):
   ```bash
   sudo apt-get install jq
   ```

## Option 1: Complete Automated Workflow

Run everything in one command:

```bash
./scripts/license-workflow.sh workflow
```

This will:
1. ✓ Check license server health
2. ✓ Generate master signing key
3. ✓ Create test organization
4. ✓ Create license agreement with full features
5. ✓ Generate JWT token and save to `~/.config/janus/license.jwt`
6. ✓ Validate the token
7. ✓ Build Janus with embedded public key
8. ✓ Ready to run `./janus`

## Option 2: Interactive Menu

Run the script without arguments for an interactive menu:

```bash
./scripts/license-workflow.sh
```

## Option 3: Individual Commands

### Start License Server

```bash
cd cmd/license-server
go run . serve
```

Or if already built:
```bash
cd cmd/license-server
./license-server serve
```

### Check Server Health

```bash
curl http://localhost:8443/health | jq
```

### Generate Master Signing Key

```bash
curl -X POST http://localhost:8443/api/v1/keys/rotate \
  -H "Content-Type: application/json" \
  -d '{"expires_in_days": 365}' | jq
```

### List All Signing Keys

```bash
curl http://localhost:8443/api/v1/keys | jq
```

### Get Master Public Key

```bash
curl http://localhost:8443/api/v1/keys | jq -r '.[] | select(.organization_id == null and .active == true) | .public_key_pem' | head -1
```

### Create Organization

The `customer_id` is a unique identifier for the organization (e.g., company slug).

```bash
curl -X POST http://localhost:8443/api/v1/organizations \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Test Organization",
    "customer_id": "test-org"
  }' | jq
```

### List Organizations

```bash
curl http://localhost:8443/api/v1/organizations/list | jq
```

### Create License Agreement

Replace `ORG_ID` with the actual organization ID. `start_date` is required (YYYY-MM-DD format):

```bash
curl -X POST http://localhost:8443/api/v1/agreements \
  -H "Content-Type: application/json" \
  -d '{
    "organization_id": 1,
    "tier": "enterprise",
    "max_seats": 50,
    "features": ["audit", "grid", "validation"],
    "start_date": "2025-01-01",
    "end_date": "2025-12-31",
    "cost_per_month": 5000.00
  }' | jq
```

Note: `end_date` is optional. If not provided, the license has no expiration.

### List Agreements

```bash
curl http://localhost:8443/api/v1/agreements/list | jq
```

### Generate JWT Token

Replace `AGREEMENT_ID` with the actual agreement ID:

```bash
curl -X POST http://localhost:8443/api/v1/tokens \
  -H "Content-Type: application/json" \
  -d '{
    "agreement_id": 1,
    "user_email": "user@example.com",
    "duration_seconds": 31536000
  }' | jq -r '.token' > ~/.config/janus/license.jwt
```

### Validate Token

```bash
curl -X POST http://localhost:8443/api/v1/tokens/validate \
  -H "Content-Type: application/json" \
  -d "{\"token\": \"$(cat ~/.config/janus/license.jwt)\"}" | jq
```

### Build Janus with Embedded Public Key

```bash
# Get the master public key
PUBLIC_KEY=$(curl -s http://localhost:8443/api/v1/keys | \
  jq -r '.[] | select(.organization_id == null and .active == true) | .public_key_pem' | head -1)

# Build Janus with the embedded key
go build -ldflags "-X 'github.com/pharmalytica/janus/internal/license/publickey.EmbeddedPublicKey=${PUBLIC_KEY}'" \
  -o ./janus .
```

## Testing the License

After running the workflow or manual steps:

```bash
# Run Janus (will validate license automatically and launch GUI)
./janus
```

If license is invalid or missing, you'll see a GUI dialog with the error.

## License Tiers and Features

### Enterprise
- **Features**: `["audit", "grid", "validation", "reporting"]`
- **Max Seats**: 50+
- **Cost**: $5000+/month

### Professional
- **Features**: `["audit", "grid"]`
- **Max Seats**: 10-50
- **Cost**: $2000-5000/month

### Starter
- **Features**: `["audit"]`
- **Max Seats**: 1-10
- **Cost**: $500-2000/month

## Feature Mapping

| Feature | Description | GUI Impact |
|---------|-------------|------------|
| `audit` | Audit trail logging | Enables audit logging for all operations |
| `grid` | Grid scheduler support | Shows Grid Details tab, enables SLURM monitoring |
| `validation` | IQ/OQ validation | Enables validation test suite |
| `reporting` | Advanced reporting | Enables report generation features |

## Troubleshooting

### License Server Not Running

```bash
# Check if server is running
curl http://localhost:8443/health

# Start the server
cd cmd/license-server
export LICENSING_DATABASE_URL="postgres://janus:janus_dev_password@localhost:5432/janus_license?sslmode=disable"
export LICENSING_ENCRYPTION_KEY="12345678901234567890123456789012"
go run . serve
```

### PostgreSQL Not Running

```bash
# Start PostgreSQL
docker compose --profile dev up -d postgres

# Check PostgreSQL status
docker compose ps
```

### No Active Master Key

```bash
# Generate a new master key
curl -X POST http://localhost:8443/api/v1/keys/rotate \
  -H "Content-Type: application/json" \
  -d '{"expires_in_days": 365}' | jq
```

### License File Not Found

```bash
# Check if license file exists
cat ~/.config/janus/license.jwt

# Regenerate license
./scripts/license-workflow.sh workflow
```

### Build Without Embedded Key (Dev Mode)

If you just want to test without embedding a key:

```bash
go build -o ./janus .
```

This will build with `EmbeddedPublicKey = "dev"` but license validation will fail.

## Production Deployment

For production, you should:

1. Generate a production master key with long expiration
2. Store the public key securely
3. Build Janus with the embedded public key using CI/CD
4. Distribute the signed binary
5. Generate customer-specific tokens from agreements

Example production build:

```bash
PUBLIC_KEY=$(curl -s https://license.yourdomain.com/api/v1/keys/public | jq -r '.public_key_pem')

go build -ldflags "\
  -X 'github.com/pharmalytica/janus/internal/license/publickey.EmbeddedPublicKey=${PUBLIC_KEY}' \
  -X github.com/pharmalytica/janus/internal/version.Version=v1.0.0 \
  -X github.com/pharmalytica/janus/internal/version.Commit=$(git rev-parse HEAD)" \
  -o janus-v1.0.0 .
```
