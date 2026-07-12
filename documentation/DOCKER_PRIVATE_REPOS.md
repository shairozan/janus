# Docker Support for Private GitHub Repositories

## Overview

Janus now supports building and testing in Docker with private GitHub repository dependencies (specifically `github.com/pharmalytica/hermes`). This document explains how the authentication works and how to use it.

## Problem

The `pharmalytica/hermes` repository is private, which means:
- Docker builds fail when trying to `go mod download`
- CI/CD pipelines can't access the dependency
- Developers on different machines need consistent authentication

## Solution

We've implemented GitHub token-based authentication for Docker builds that works both locally and in CI/CD.

## How It Works

### 1. Build-Time Authentication (docker/Dockerfile.dev)

The development Dockerfile accepts GitHub credentials as build arguments:

```dockerfile
ARG GITHUB_USER
ARG GITHUB_TOKEN

ENV GOPRIVATE=github.com/pharmalytica/hermes

RUN if [ -n "$GITHUB_USER" ] && [ -n "$GITHUB_TOKEN" ]; then \
        git config --global url."https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com/".insteadOf "https://github.com/"; \
    fi
```

This configures Git to use the token for HTTPS authentication when Go tries to fetch private modules.

### 2. Runtime Authentication (mage docker commands)

The mage Docker commands automatically pass through GitHub credentials:

```go
// In runInDevContainer()
if githubUser := os.Getenv("GITHUB_USER"); githubUser != "" {
    dockerArgs = append(dockerArgs, "-e", "GITHUB_USER="+githubUser)
}
if githubToken := os.Getenv("GITHUB_TOKEN"); githubToken != "" {
    dockerArgs = append(dockerArgs, "-e", "GITHUB_TOKEN="+githubToken)
}
dockerArgs = append(dockerArgs, "-e", "GOPRIVATE=github.com/pharmalytica/hermes")
```

## Usage

### Local Development

1. **Create GitHub PAT** (if you don't have one):
   - Go to https://github.com/settings/tokens
   - Generate new token (classic)
   - Select `repo` scope
   - Copy the token

2. **Set environment variables**:

   **Windows (PowerShell)**:
   ```powershell
   $env:GITHUB_USER="your-username"
   $env:GITHUB_TOKEN="ghp_xxxxxxxxxxxx"
   ```

   **Windows (CMD)**:
   ```cmd
   set GITHUB_USER=your-username
   set GITHUB_TOKEN=ghp_xxxxxxxxxxxx
   ```

   **Linux/macOS (bash/zsh)**:
   ```bash
   export GITHUB_USER="your-username"
   export GITHUB_TOKEN="ghp_xxxxxxxxxxxx"
   ```

   **Permanent (add to shell profile)**:
   ```bash
   # ~/.bashrc or ~/.zshrc
   export GITHUB_USER="your-username"
   export GITHUB_TOKEN="ghp_xxxxxxxxxxxx"
   ```

3. **Build development image**:
   ```bash
   mage docker:buildDev
   ```

4. **Use Docker commands**:
   ```bash
   mage docker:build        # Build binary
   mage docker:unit         # Run unit tests
   mage docker:gui          # Run GUI tests
   mage docker:checkAll     # Full validation
   mage docker:shell        # Interactive shell
   ```

### CI/CD (GitHub Actions)

```yaml
name: CI

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Docker
        run: docker pull dukeofubuntu/janus-ci:latest-ubuntu24

      - name: Build dev image
        env:
          GITHUB_USER: ${{ secrets.GH_USER }}
          GITHUB_TOKEN: ${{ secrets.GH_TOKEN }}
        run: mage docker:buildDev

      - name: Run tests
        env:
          GITHUB_USER: ${{ secrets.GH_USER }}
          GITHUB_TOKEN: ${{ secrets.GH_TOKEN }}
        run: mage docker:checkAll
```

**Required Secrets**:
- `GH_USER`: GitHub username
- `GH_TOKEN`: Personal access token with `repo` scope

### CI/CD (GitLab CI)

```yaml
variables:
  DOCKER_IMAGE: dukeofubuntu/janus-ci:latest-ubuntu24

stages:
  - test

test:
  stage: test
  image: docker:latest
  services:
    - docker:dind
  before_script:
    - apk add --no-cache go
  script:
    - export GITHUB_USER=$GH_USER
    - export GITHUB_TOKEN=$GH_TOKEN
    - mage docker:buildDev
    - mage docker:checkAll
```

**Required Variables** (Settings → CI/CD → Variables):
- `GH_USER`: GitHub username (can be visible)
- `GH_TOKEN`: Personal access token (must be masked/protected)

## Security Best Practices

### Token Scope

Only grant the minimum required scope:
- ✅ `repo` - Read access to private repositories
- ❌ Don't grant `admin`, `delete`, or other unnecessary scopes

### Token Storage

- **Local Development**: Store in environment variables, NOT in code
- **CI/CD**: Use platform secrets/variables features
- **Never commit**: Add `.env` files to `.gitignore`

### Token Rotation

- Rotate tokens regularly (every 90 days recommended)
- Revoke old tokens immediately after rotation
- Update all CI/CD environments when rotating

### Token Protection

```bash
# BAD - Don't do this
echo "GITHUB_TOKEN=ghp_xxxxx" >> ~/.bashrc

# GOOD - Use a secrets manager
# AWS Secrets Manager, 1Password, etc.

# ACCEPTABLE - Environment variable (cleared on logout)
export GITHUB_TOKEN="ghp_xxxxx"
```

## Troubleshooting

### Build fails with "unknown revision v0.0.1"

**Cause**: Git can't authenticate to private repo.

**Fix**: Ensure `GITHUB_USER` and `GITHUB_TOKEN` are set:
```bash
echo $GITHUB_USER  # Should print your username
echo $GITHUB_TOKEN # Should print ghp_xxxxx (don't share output)
```

### "Development image not found"

**Cause**: Haven't built the dev image yet.

**Fix**:
```bash
mage docker:buildDev
```

### "This may be a private repository"

**Cause**: Token doesn't have `repo` scope or is expired.

**Fix**: Generate new token with correct scope.

### Docker build succeeds but runtime fails

**Cause**: Credentials were used at build time but not passed at runtime.

**Fix**: Mage commands handle this automatically. If using `docker run` manually:
```bash
docker run --rm \
  -e GITHUB_USER=$GITHUB_USER \
  -e GITHUB_TOKEN=$GITHUB_TOKEN \
  -e GOPRIVATE=github.com/pharmalytica/hermes \
  -v $(pwd):/workspace \
  janus-dev:latest \
  go build
```

## Files Modified

1. **magefiles/docker.go**
   - Added GitHub credential pass-through in `runInDevContainer()`
   - Added build args in `BuildDev()`

2. **docker/Dockerfile.dev**
   - Added `ARG GITHUB_USER` and `ARG GITHUB_TOKEN`
   - Added `ENV GOPRIVATE=github.com/pharmalytica/hermes`
   - Added Git configuration for token authentication

3. **go.mod**
   - Removed local `replace` directive for hermes
   - Using proper version: `github.com/pharmalytica/hermes v0.0.1`

4. **README.md**
   - Added "Docker Development with Private Repositories" section
   - Documented setup steps and CI/CD configuration

## Benefits

1. **Consistent Builds**: Same authentication mechanism for local and CI/CD
2. **No Local Clones**: Don't need `../../hermes` directory locally
3. **Security**: Tokens can be rotated without changing code
4. **CI/CD Ready**: Works in any CI/CD platform with secret storage
5. **Fast Development**: Cached dependencies in Docker image

## Alternative: Making Repository Public

If the `pharmalytica/hermes` repository becomes public:
1. Remove `GITHUB_USER`/`GITHUB_TOKEN` environment variables
2. Keep `GOPRIVATE` setting (harmless for public repos)
3. Docker builds will work without authentication
4. This documentation remains valid for future private dependencies
