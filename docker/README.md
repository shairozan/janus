# Janus CI Docker Images

This directory contains Docker images for the Janus CI/CD pipeline. These images include all necessary dependencies for building and testing Janus, including GUI libraries and headless display support.

## Available Images

All images are published to GitHub Container Registry under `ghcr.io/shairozan/janus-ci`:

- `ghcr.io/shairozan/janus-ci:ubuntu20` - Ubuntu 20.04 LTS
- `ghcr.io/shairozan/janus-ci:ubuntu22` - Ubuntu 22.04 LTS
- `ghcr.io/shairozan/janus-ci:ubuntu24` - Ubuntu 24.04 LTS

## What's Included

Each image contains:

- **Go 1.25.1** - Latest Go compiler
- **Build Tools** - gcc, g++, make, pkg-config, multilib support
- **GUI Dependencies** - OpenGL, X11, Fyne dependencies
- **Headless Display** - Xvfb for GUI testing without physical display
- **Development Tools** - git, curl, mage build system
- **Optimized Environment** - Pre-configured for Janus development

## Usage

### In GitHub Actions

```yaml
jobs:
  test:
    container: ghcr.io/shairozan/janus-ci:ubuntu22
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Run tests
        run: |
          go mod download
          mage unit
```

### Local Development

```bash
# Run interactive shell
docker run -it --rm \
  -v $(pwd):/workspace \
  -w /workspace \
  ghcr.io/shairozan/janus-ci:ubuntu22

# Run specific command
docker run --rm \
  -v $(pwd):/workspace \
  -w /workspace \
  ghcr.io/shairozan/janus-ci:ubuntu22 \
  bash -c "go mod download && mage lint"
```

### GUI Testing

The images include Xvfb (virtual framebuffer) for headless GUI testing:

```bash
# GUI tests will automatically use virtual display
docker run --rm \
  -v $(pwd):/workspace \
  -w /workspace \
  ghcr.io/shairozan/janus-ci:ubuntu22 \
  bash -c "mage unit"  # Includes GUI tests
```

## Environment Variables

Each image sets these environment variables:

- `DISPLAY=:99` - Virtual display for GUI applications
- `CGO_ENABLED=1` - Enable CGO for Fyne compilation
- `QT_QPA_PLATFORM=offscreen` - Qt headless mode
- `FYNE_THEME=light` - Default Fyne theme
- `GOPATH=/go` - Go workspace
- `PATH` - Includes Go binaries and mage

## Building Images Locally

```bash
# Build specific version
docker build -f docker/Dockerfile.ubuntu22 -t janus-ci:ubuntu22 .

# Build all versions
for version in 20 22 24; do
  docker build -f docker/Dockerfile.ubuntu${version} -t janus-ci:ubuntu${version} .
done
```

## Image Maintenance

Images are automatically rebuilt:

- **On code changes** - When Dockerfiles are modified
- **Weekly** - Every Sunday at 2 AM UTC for security updates
- **Manual trigger** - Via GitHub Actions workflow dispatch

## Performance Benefits

Using these pre-built images in CI provides:

- **Faster builds** - No dependency installation time
- **Consistency** - Same environment across all CI runs
- **Reliability** - Pre-tested, known-good configurations
- **Multi-platform** - Support for different Ubuntu LTS versions

## Size Optimization

Images are optimized for CI use:

- Multi-stage builds to minimize final size
- Cleaned package caches
- Only essential dependencies included
- Shared layer caching for faster pulls

## Troubleshooting

### GUI Tests Failing

If GUI tests fail, ensure the image has proper virtual display:

```bash
# Check display is available
echo $DISPLAY  # Should show :99

# Manually start Xvfb if needed
Xvfb :99 -screen 0 1024x768x24 -ac +extension GLX +render -noreset &
```

### Build Issues

If builds fail, check CGO is enabled:

```bash
# Verify CGO
echo $CGO_ENABLED  # Should be 1

# Check compiler
gcc --version
pkg-config --list-all | grep -E "(gl|x11)"
```

### Performance Issues

For faster local development, use volume mounts for Go cache:

```bash
docker run --rm \
  -v $(pwd):/workspace \
  -v ~/.cache/go-build:/root/.cache/go-build \
  -v ~/go/pkg/mod:/go/pkg/mod \
  -w /workspace \
  ghcr.io/shairozan/janus-ci:ubuntu22 \
  mage unit
```