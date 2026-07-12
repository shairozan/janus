# Janus License Server - Docker Deployment

This directory contains Docker deployment files for the Janus License Server.

## Quick Start

### Using Docker Compose (Recommended for local development)

```bash
# Set encryption key (required)
export LICENSING_ENCRYPTION_KEY="your-32-byte-encryption-key-here"

# Start the services
docker-compose up -d

# View logs
docker-compose logs -f license-server

# Stop the services
docker-compose down
```

The license server will be available at `http://localhost:8080`.

### Using Docker Only

```bash
# Build the image
docker build -f license/Dockerfile -t janus-license-server:latest .

# Run with external PostgreSQL
docker run -d \
  -p 8080:8080 \
  -e LICENSING_DATABASE_URL="postgres://user:pass@host:5432/dbname?sslmode=disable" \
  -e LICENSING_ENCRYPTION_KEY="your-32-byte-encryption-key-here" \
  janus-license-server:latest serve --auto-migrate
```

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `LICENSING_DATABASE_URL` | Yes | PostgreSQL connection string |
| `LICENSING_ENCRYPTION_KEY` | Yes | 32-byte AES-256 encryption key for private keys |
| `LICENSING_PORT` | No | HTTP server port (default: 8080) |

## Pre-built Images

Pre-built images are available from GitHub Container Registry:

```bash
docker pull ghcr.io/pharmalytica/janus/license-server:latest
docker pull ghcr.io/pharmalytica/janus/license-server:v1.0.0
```

### Available Tags

- `latest` - Latest build from main branch
- `v*.*.*` - Semantic version releases
- `main` - Latest commit on main branch
- `sha-<commit>` - Specific commit builds

## Image Details

- **Base Image**: `scratch` (minimal, ~15MB)
- **Platforms**: `linux/amd64`, `linux/arm64`
- **User**: Runs as non-root (UID 65534)
- **Exposed Port**: 8080

## Database Migrations

Migrations are automatically applied when using the `--auto-migrate` flag:

```bash
docker run janus-license-server:latest serve --auto-migrate
```

Or manually:

```bash
docker run janus-license-server:latest migrate
```

## Security Considerations

1. **Always use a strong encryption key**: Generate with `openssl rand -base64 32`
2. **Use TLS in production**: Place behind a reverse proxy (nginx, Traefik, etc.)
3. **Secure database connection**: Use SSL/TLS for PostgreSQL connections
4. **Keep images updated**: Regularly pull latest images for security patches

## Health Checks

The license server exposes a health check endpoint:

```bash
curl http://localhost:8080/health
```

## Building Multi-Platform Images

```bash
docker buildx create --use
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -f license/Dockerfile \
  -t janus-license-server:latest \
  --push .
```

## Troubleshooting

### Container won't start
- Check database connectivity: `docker logs <container-id>`
- Verify `LICENSING_DATABASE_URL` is correct
- Ensure PostgreSQL is running and accessible

### Migration errors
- Manually inspect database: `psql $LICENSING_DATABASE_URL`
- Check migrations table: `SELECT * FROM goose_db_version;`
- Rollback if needed: `docker run janus-license-server:latest migrate down`

### Permission denied errors
- The container runs as UID 65534 (nobody)
- Ensure mounted volumes have correct permissions

## Production Deployment

Example Kubernetes deployment coming soon. For now, use docker-compose or your orchestration tool of choice.

Example nginx reverse proxy configuration:

```nginx
server {
    listen 443 ssl http2;
    server_name license.example.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```
