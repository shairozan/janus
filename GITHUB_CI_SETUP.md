# GitHub CI/CD Setup for Private Repository Access

This guide explains how to configure GitHub Actions to access the private `pharmalytica/hermes` repository.

## Quick Summary

**What you need to add**: One GitHub secret named `GH_PRIVATE_REPO_TOKEN`

**Where to add it**: Repository Settings → Secrets and variables → Actions

**What it contains**: A GitHub Personal Access Token (PAT) with `repo` scope

---

## Step-by-Step Setup

### 1. Create a GitHub Personal Access Token (PAT)

1. Go to GitHub Settings → Developer settings → Personal access tokens → **Tokens (classic)**
   - Direct link: https://github.com/settings/tokens

2. Click **"Generate new token"** → **"Generate new token (classic)"**

3. Configure the token:
   - **Note**: `Janus CI/CD - Private Repo Access`
   - **Expiration**: Choose your preference (recommended: 90 days with calendar reminder to rotate)
   - **Scopes**: Select **ONLY** `repo` (Full control of private repositories)
     - ✅ `repo` (includes repo:status, repo_deployment, public_repo, repo:invite, security_events)
     - ❌ Don't select any other scopes (minimize permissions)

4. Click **"Generate token"**

5. **IMPORTANT**: Copy the token immediately (it starts with `ghp_`)
   - You won't be able to see it again!
   - Store it temporarily in a secure location (password manager recommended)

### 2. Add Secret to Janus Repository

1. Go to the **pharmalytica/janus** repository on GitHub

2. Navigate to **Settings** → **Secrets and variables** → **Actions**

3. Click **"New repository secret"**

4. Configure the secret:
   - **Name**: `GH_PRIVATE_REPO_TOKEN` (must match exactly)
   - **Secret**: Paste the token you created (the `ghp_...` value)

5. Click **"Add secret"**

### 3. Verify the Setup

After adding the secret, the next push or pull request will automatically use it.

**Test with a simple push**:
```bash
git add .
git commit -m "test: verify CI private repo access"
git push
```

**Check the workflow run**:
1. Go to the **Actions** tab in GitHub
2. Find your workflow run
3. Check the "Download dependencies" step
4. Should see: "all modules verified" without any authentication errors

---

## What This Enables

With `GH_PRIVATE_REPO_TOKEN` configured, all workflows can now:

### ✅ CI Workflow (`ci.yml`)
- Download `github.com/pharmalytica/hermes` dependency
- Run unit tests with Hermes integration
- Run integration tests
- Run linting

### ✅ Release Workflow (`release.yml`)
- Build binaries for all platforms (Linux, Windows, macOS)
- Run validation tests
- Create release artifacts with Hermes support

### ✅ Docker Build Workflow (`docker-build.yml`)
- Build CI images with cached Hermes dependency
- Push to Docker Hub with all dependencies pre-installed

---

## How It Works

### In CI Containers (Ubuntu)

The workflows configure Git to use the token for HTTPS authentication:

```yaml
- name: Configure Git for private repositories
  run: |
    git config --global url."https://${{ secrets.GITHUB_TOKEN }}@github.com/".insteadOf "https://github.com/"
  env:
    GITHUB_TOKEN: ${{ secrets.GH_PRIVATE_REPO_TOKEN }}

- name: Download dependencies
  run: go mod download
  env:
    GOPRIVATE: github.com/pharmalytica/hermes
```

**What this does**:
1. Tells Git to use the token for all `github.com` URLs
2. Sets `GOPRIVATE` so Go doesn't try to use the public proxy
3. When `go mod download` runs, it uses Git with authentication

### In Docker Builds

The Docker build workflow passes credentials as build arguments:

```yaml
- name: Build and push Docker image
  uses: docker/build-push-action@v5
  with:
    build-args: |
      GITHUB_USER=${{ github.actor }}
      GITHUB_TOKEN=${{ secrets.GH_PRIVATE_REPO_TOKEN }}
```

**What this does**:
1. Passes the token to the Dockerfile at build time
2. Dockerfile configures Git authentication during image build
3. Dependencies are cached in the image for faster subsequent builds

---

## Security Best Practices

### ✅ DO

- **Use token with minimal scope** (`repo` only)
- **Set expiration** (90 days recommended)
- **Rotate regularly** (set calendar reminder)
- **Use Classic PAT** (fine-grained tokens not yet supported everywhere)
- **Revoke immediately** if compromised
- **Document expiration date** so others know when rotation is needed

### ❌ DON'T

- **Don't grant `admin` scope** (not needed)
- **Don't grant `write:packages` scope** (not needed)
- **Don't set "No expiration"** (security risk)
- **Don't share the token** (GitHub audits who created it)
- **Don't commit the token** (already protected by GitHub Secrets)
- **Don't use in local development** (use your own credentials locally)

---

## Token Rotation

When the token expires, you'll see authentication failures in CI:

**Symptoms**:
```
go: github.com/pharmalytica/hermes@v0.0.1: reading github.com/pharmalytica/hermes/go.mod at revision v0.0.1:
git ls-remote -q https://github.com/pharmalytica/hermes in /go/pkg/mod/cache/vcs/xxx: exit status 128:
fatal: could not read Username for 'https://github.com': terminal prompts disabled
```

**Fix**:
1. Create new PAT (same process as above)
2. Update `GH_PRIVATE_REPO_TOKEN` secret with new value
3. No code changes needed!

---

## Troubleshooting

### Issue: "Permission denied" during `go mod download`

**Cause**: Token doesn't have `repo` scope or is expired.

**Fix**:
1. Check token scopes at https://github.com/settings/tokens
2. Ensure `repo` scope is checked
3. If expired, create new token and update secret

### Issue: "Could not read Username for 'https://github.com'"

**Cause**: Secret `GH_PRIVATE_REPO_TOKEN` not set or named incorrectly.

**Fix**:
1. Go to repo Settings → Secrets and variables → Actions
2. Verify secret exists and is named **exactly** `GH_PRIVATE_REPO_TOKEN`
3. If missing, add it (see Step 2 above)

### Issue: "rate limit exceeded"

**Cause**: Using GitHub's automatic `GITHUB_TOKEN` instead of personal PAT.

**Fix**: Ensure `GH_PRIVATE_REPO_TOKEN` is set - the automatic token can't access other private repos.

### Issue: Docker build fails with authentication error

**Cause**: `docker-build.yml` workflow doesn't have the build-args.

**Fix**: Verify the workflow file has:
```yaml
build-args: |
  GITHUB_USER=${{ github.actor }}
  GITHUB_TOKEN=${{ secrets.GH_PRIVATE_REPO_TOKEN }}
```

---

## Alternative: Make Hermes Repository Public

If you decide to make `pharmalytica/hermes` public:

1. No changes needed to workflows (they'll continue to work)
2. Can remove `GH_PRIVATE_REPO_TOKEN` secret (optional)
3. `GOPRIVATE` setting is harmless for public repos
4. Local development becomes easier (no credentials needed)

**Trade-off**: Repository code becomes visible to everyone.

---

## Files Modified

The following workflow files were updated to support private repository access:

1. **`.github/workflows/ci.yml`**
   - Added Git configuration step
   - Added `GOPRIVATE` environment variable

2. **`.github/workflows/release.yml`**
   - Updated `test` job with Git configuration
   - Updated `validation` job with Git configuration
   - Updated `build-matrix` job for all platforms (Linux, Windows, macOS)

3. **`.github/workflows/docker-build.yml`**
   - Added `build-args` with GitHub credentials
   - Passes credentials to Dockerfile at build time

4. **`docker/Dockerfile.dev`**
   - Added `ARG GITHUB_USER` and `ARG GITHUB_TOKEN`
   - Added Git configuration for private repos
   - Set `GOPRIVATE` environment variable

All changes are backward compatible - if the secret isn't set, regular public dependencies still work.

---

## Summary Checklist

- [ ] Create GitHub Personal Access Token (classic) with `repo` scope
- [ ] Add `GH_PRIVATE_REPO_TOKEN` secret to pharmalytica/janus repository
- [ ] Push a test commit to verify CI works
- [ ] Set calendar reminder for token rotation (90 days)
- [ ] Document token creation date and expiration date

Once complete, all CI/CD workflows will automatically authenticate to the private `pharmalytica/hermes` repository!
