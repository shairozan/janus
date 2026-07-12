# Docker Hub Setup for Janus CI

This guide walks through setting up Docker Hub integration for the Janus CI/CD pipeline.

## 1. Create Docker Hub Account and Repository

1. **Create/Login to Docker Hub**: Go to [hub.docker.com](https://hub.docker.com)
2. **Create Repository**: Create a new public repository named `janus-ci`
3. **Create Access Token**: Go to Account Settings → Security → Access Tokens
   - Name: `github-actions-janus`
   - Permissions: `Read, Write, Delete`
   - **Save the token securely** - you'll need it for GitHub secrets

## 2. Configure GitHub Secrets

In your GitHub repository settings, add these secrets:

### Repository Secrets

Go to: **Settings** → **Secrets and Variables** → **Actions** → **New repository secret**

Add these two secrets:

| Secret Name | Value | Description |
|-------------|-------|-------------|
| `DOCKERHUB_USERNAME` | Your Docker Hub username | Used for docker login |
| `DOCKERHUB_TOKEN` | The access token from step 1 | Authentication token |

### Example:
```
DOCKERHUB_USERNAME: shairozan
DOCKERHUB_TOKEN: dckr_pat_abc123xyz789... (your token)
```

## 3. Test the Setup

1. **Push to main branch** with Docker file changes to trigger build
2. **Check Actions tab** for the "Build Docker Images" workflow
3. **Verify images** are pushed to Docker Hub at:
   - `shairozan/janus-ci:ubuntu20`
   - `shairozan/janus-ci:ubuntu22`
   - `shairozan/janus-ci:ubuntu24`

## 4. Using Images in CI

Once images are built and pushed, update your workflows:

```yaml
jobs:
  test:
    runs-on: ubuntu-latest
    container: shairozan/janus-ci:ubuntu22
    steps:
      - uses: actions/checkout@v4
      - run: mage unit
```

## 5. Manual Image Building

If you need to build/push manually:

```bash
# Login to Docker Hub
docker login -u shairozan

# Build and push all images
docker build -f docker/Dockerfile.ubuntu20 -t shairozan/janus-ci:ubuntu20 .
docker push shairozan/janus-ci:ubuntu20

docker build -f docker/Dockerfile.ubuntu22 -t shairozan/janus-ci:ubuntu22 .
docker push shairozan/janus-ci:ubuntu22

docker build -f docker/Dockerfile.ubuntu24 -t shairozan/janus-ci:ubuntu24 .
docker push shairozan/janus-ci:ubuntu24
```

## 6. Image Maintenance

Images are automatically maintained:

- **On Dockerfile changes**: Rebuild triggered by PR/push
- **Weekly rebuilds**: Every Sunday at 2 AM UTC for security updates
- **Manual trigger**: Use "Run workflow" in GitHub Actions

## Benefits

After setup, you'll get:

✅ **Faster CI builds** - No dependency installation time
✅ **Consistent environments** - Same setup across all CI runs
✅ **Headless GUI testing** - Xvfb pre-configured for Fyne tests
✅ **Multi-platform support** - Ubuntu 20.04, 22.04, 24.04
✅ **Reduced failures** - Pre-tested, stable environments

## Troubleshooting

### Docker Hub Push Fails
- Verify `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` secrets are set
- Check token has `Read, Write, Delete` permissions
- Ensure repository `janus-ci` exists and is public

### Image Pull Fails in CI
- Wait for images to build and push (check Actions tab)
- Verify image names match exactly: `shairozan/janus-ci:ubuntu22`
- Check Docker Hub repository is public

### Build Errors
- Check Dockerfile syntax in `docker/` directory
- Ensure base images (ubuntu:20.04, etc.) are available
- Review build logs in GitHub Actions

## Security Notes

- Docker Hub tokens are scoped to your account only
- Repository is public but tokens remain private in GitHub secrets
- Images contain no secrets or sensitive data
- Regular rebuilds ensure latest security patches