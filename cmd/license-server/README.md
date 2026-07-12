# License Server

JWT-based licensing server for Janus with PostgreSQL backend and AES-GCM encrypted key storage.

## Building

### Development Build (default version: "dev")
```bash
go build -o license-server ./cmd/license-server
```

### Production Build with Version Information
```bash
go build -ldflags "\
  -X github.com/pharmalytica/janus/internal/license/version.Version=v1.0.0 \
  -X github.com/pharmalytica/janus/internal/license/version.Commit=$(git rev-parse HEAD) \
  -X github.com/pharmalytica/janus/internal/license/version.Date=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
  -X github.com/pharmalytica/janus/internal/license/version.BuiltBy=github-actions" \
  -o license-server ./cmd/license-server
```

### GitHub Actions Build (automated)
The CI/CD pipeline automatically injects version information during tagged builds:
```yaml
- name: Build
  run: |
    go build -ldflags "\
      -X github.com/pharmalytica/janus/internal/license/version.Version=${{ github.ref_name }} \
      -X github.com/pharmalytica/janus/internal/license/version.Commit=${{ github.sha }} \
      -X github.com/pharmalytica/janus/internal/license/version.Date=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
      -X github.com/pharmalytica/janus/internal/license/version.BuiltBy=github-actions" \
      -o license-server ./cmd/license-server
```

## Version Information

Version information is exposed in the health endpoint:

```bash
curl http://localhost:8443/health | jq .
```

Response:
```json
{
  "status": "healthy",
  "timestamp": "2025-09-30T15:56:42Z",
  "version": {
    "version": "v1.0.0",
    "commit": "abc123def456",
    "date": "2025-09-30T15:56:23Z",
    "builtBy": "github-actions",
    "goVersion": "go1.25.1"
  }
}
```

## Configuration

All configuration via environment variables with `LICENSING_` prefix:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `LICENSING_DATABASE_URL` | Yes | - | PostgreSQL connection string |
| `LICENSING_ENCRYPTION_KEY` | Yes | - | 32-byte encryption key |
| `LICENSING_PORT` | No | `8443` | Server listen port |
| `LICENSING_HOST` | No | `0.0.0.0` | Server bind host |

## Development Setup

1. **Start PostgreSQL**:
   ```bash
   docker compose --profile dev up -d postgres
   ```

2. **Copy environment template**:
   ```bash
   cp .env.example .env
   ```

3. **Run the server**:
   ```bash
   source .env
   go run ./cmd/license-server serve
   ```

   Or in VS Code, press **F5** and select "License Server"

## API Endpoints

- `GET  /health` - Health check with version info
- `POST /api/v1/tokens` - Generate JWT token
- `POST /api/v1/tokens/validate` - Validate JWT token
- `POST /api/v1/keys/rotate` - Rotate signing keys

## Database Migrations

Migrations run automatically on server startup using [goose](https://github.com/pressly/goose).

Migration files: `internal/license/db/migrations/*.sql`

## Security

- Private keys encrypted with AES-256-GCM
- JWT signatures using RS256 (RSA 4096-bit keys)
- Environment-based encryption key management
- Database credentials via environment variables

## Testing

```bash
# Health check
curl http://localhost:8443/health

# Generate token (requires test data)
curl -X POST http://localhost:8443/api/v1/tokens \
  -H "Content-Type: application/json" \
  -d '{"agreement_id": 1, "user_email": "user@example.com"}'

# Validate token
curl -X POST http://localhost:8443/api/v1/tokens/validate \
  -H "Content-Type: application/json" \
  -d '{"token": "eyJhbGc..."}'
```