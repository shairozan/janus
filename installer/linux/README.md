# Linux DEB Package

This directory contains the configuration files and build script for creating Janus Debian (.deb) packages for Ubuntu/Debian systems.

## Prerequisites

### Required Software

1. **dpkg-deb** (standard on Debian/Ubuntu)
   ```bash
   # Usually pre-installed, but if needed:
   sudo apt-get install dpkg-dev
   ```

2. **ImageMagick** (for icon generation)
   ```bash
   sudo apt-get install imagemagick
   ```

3. **Go 1.23+** (for building the binary)
   - Download from https://golang.org/dl/

## Building the Package

### Quick Build

```bash
cd installer/linux
./build-deb.sh 0.2.0 ubuntu2404
```

This will:
1. Use the pre-built binary from the repository root (e.g., `janus-ubuntu2404`)
2. Generate PNG icons from `assets/logo.png` (if not already present)
3. Create the DEB package structure with desktop integration
4. Install MIME type definitions for `.mod`, `.ctl`, and `.nmctl` files
5. Set up post-install and removal scripts
6. Build the DEB package: `janus_0.2.0_ubuntu2404_amd64.deb`
7. Generate SHA256 checksum

**Note**: The script expects the binary to already be built. In CI, this is handled by the build-matrix job. For local builds, build the binary first:

```bash
cd /path/to/janus
GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -o janus-ubuntu2404 -ldflags "-X github.com/pharmalytica/janus/internal/version.Version=0.2.0" .
```

### Manual Build Steps

If you need more control:

```bash
# 1. Build the Go binary
cd /path/to/janus
GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build \\
  -o janus-ubuntu2404 \\
  -ldflags "-X github.com/pharmalytica/janus/internal/version.Version=0.2.0" .

# 2. Generate icons (if needed)
convert assets/logo.png -resize 48x48 assets/icons/janus-48.png
convert assets/logo.png -resize 128x128 assets/icons/janus-128.png
convert assets/logo.png -resize 256x256 assets/icons/janus-256.png

# 3. Build the DEB package
cd installer/linux
./build-deb.sh 0.2.0 ubuntu2404

# Output: ../../janus_0.2.0_ubuntu2404_amd64.deb
```

## Files in This Directory

- **`control.template`** - Debian package metadata template
  - Package name, version, dependencies
  - Description and maintainer info
  - Templated with `{{VERSION}}` placeholder

- **`postinst`** - Post-installation script
  - Updates MIME database (`update-mime-database`)
  - Updates desktop database (`update-desktop-database`)
  - Updates icon cache (`gtk-update-icon-cache`)
  - Runs automatically after `dpkg -i`

- **`postrm`** - Post-removal script
  - Cleans up MIME/desktop databases after uninstall
  - Runs automatically after `dpkg -r` or `dpkg --purge`

- **`janus.desktop`** - FreeDesktop Desktop Entry
  - Registers Janus in application menus (GNOME/KDE)
  - Associates with MIME types for double-click support
  - Defines icon, categories, and launch command

- **`janus.xml`** - MIME type definitions
  - Registers `text/x-nonmem-model` for `.mod` files
  - Registers `text/x-nonmem-control` for `.ctl` and `.nmctl` files
  - Enables file associations in file managers

- **`build-deb.sh`** - Build automation script
  - Creates DEB directory structure
  - Generates icons from logo.png
  - Populates control file from template
  - Builds the package with `dpkg-deb`
  - Generates checksums

## Package Features

### Installation Location
- Binary: `/usr/bin/janus`
- Desktop file: `/usr/share/applications/janus.desktop`
- Icons: `/usr/share/icons/hicolor/{48x48,128x128,256x256}/apps/janus.png`
- MIME types: `/usr/share/mime/packages/janus.xml`
- Documentation: `/usr/share/doc/janus/`

### System Integration
- **Application Menu**: Janus appears in GNOME Activities / KDE Launcher under "Science" category
- **File Associations**: Double-click `.mod`, `.ctl`, or `.nmctl` files to open in Janus
- **Right-Click Menu**: "Open with Janus" in file manager context menu
- **Command Line**: `janus` command available system-wide

### Dependencies

The package depends on:
- `libc6` (>= 2.31)
- `libgl1` - OpenGL support
- `libx11-6`, `libxcursor1`, `libxrandr2`, `libxinerama1` - X11 windowing
- `libxi6`, `libxxf86vm1` - Additional X11 features

These are standard on most Ubuntu/Debian desktop systems.

### User Configuration

User config is stored in `~/.config/janus/` and is **preserved** during uninstall unless you use `dpkg --purge`.

## Testing the Package

### Local Installation

```bash
# Build the package
./build-deb.sh 0.2.0 ubuntu2404

# Install
sudo dpkg -i ../../janus_0.2.0_ubuntu2404_amd64.deb

# If you get dependency errors:
sudo apt-get install -f
```

### Verification Checklist

- [ ] `/usr/bin/janus` exists and is executable
- [ ] `janus --version` shows correct version
- [ ] Janus appears in application menu (GNOME Activities / KDE Launcher)
- [ ] Application icon displays correctly in menu
- [ ] Create a test `.mod` file → double-click → Janus opens
- [ ] Right-click `.mod` file → "Open with Janus" appears
- [ ] `~/.config/janus/config.yml` is created on first run

### Uninstallation

```bash
# Remove package (keeps user config)
sudo apt-get remove janus

# Purge completely (removes everything including /etc configs)
sudo apt-get purge janus
```

**Note**: User config in `~/.config/janus/` is preserved even with `purge`.

### Clean Reinstall

```bash
# Remove old version
sudo apt-get remove janus

# Install new version
sudo dpkg -i janus_0.2.1_ubuntu2404_amd64.deb
```

## Supported Ubuntu Versions

The build process supports multiple Ubuntu target names:
- **ubuntu2004** - Ubuntu 20.04 LTS (Focal Fossa)
- **ubuntu2204** - Ubuntu 22.04 LTS (Jammy Jellyfish)
- **ubuntu2404** - Ubuntu 24.04 LTS (Noble Numbat)

Build for different versions:
```bash
./build-deb.sh 0.2.0 ubuntu2004
./build-deb.sh 0.2.0 ubuntu2204
./build-deb.sh 0.2.0 ubuntu2404
```

## Troubleshooting

### "Binary not found" Error

The build script expects the binary at:
- `../../janus-{target-name}` (repository root)
- `../../dist/linux/janus-{target-name}` (dist directory)

Make sure you've built the binary first or are running in CI where it's pre-built.

### "ImageMagick not found" Warning

Icon generation will be skipped. You can:
1. Install ImageMagick: `sudo apt-get install imagemagick`
2. Manually create icons in `assets/icons/` (janus-48.png, janus-128.png, janus-256.png)
3. Use pre-generated icons from a previous build

### File Associations Don't Work

After installing, you may need to:
1. Log out and log back in (refreshes desktop environment)
2. Right-click a `.mod` file → "Properties" → "Open With" → Select Janus
3. Check MIME database was updated: `update-mime-database ~/.local/share/mime`

### Icons Don't Appear

```bash
# Manually update icon cache
sudo gtk-update-icon-cache -f /usr/share/icons/hicolor

# Or logout/login to refresh
```

## CI/CD Integration

This package is automatically built in GitHub Actions. See `.github/workflows/release.yml`:

```yaml
- name: Create .deb package
  if: matrix.goos == 'linux'
  shell: bash
  run: |
    VERSION="${{ github.ref_name }}"
    DEB_VERSION="${VERSION#v}"
    cd installer/linux
    ./build-deb.sh "${DEB_VERSION}" "${{ matrix.target-name }}"
```

The workflow:
1. Builds binaries for ubuntu2004, ubuntu2204, ubuntu2404
2. Installs ImageMagick for icon generation
3. Runs build-deb.sh for each target
4. Uploads DEBs and checksums to GitHub Releases

## References

- [Debian Policy Manual](https://www.debian.org/doc/debian-policy/)
- [FreeDesktop Desktop Entry Specification](https://specifications.freedesktop.org/desktop-entry-spec/)
- [FreeDesktop Shared MIME Info Specification](https://specifications.freedesktop.org/shared-mime-info-spec/)
- [Ubuntu Packaging Guide](https://packaging.ubuntu.com/html/)
