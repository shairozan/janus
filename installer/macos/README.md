# macOS PKG Installer

This directory contains the build script for creating signed macOS PKG installers for Janus.

## Prerequisites

### Required Software

1. **macOS** (for building and signing)
   - macOS 10.15+ recommended
   - Xcode Command Line Tools installed

2. **pkgbuild and productbuild** (standard on macOS)
   ```bash
   # Verify installation
   which pkgbuild
   which productbuild
   ```

3. **Apple Developer ID Certificate** (for signing)
   - Developer ID Installer certificate from Apple Developer Portal
   - Certificate must be installed in macOS Keychain
   - Required for distribution outside the Mac App Store

4. **Go 1.23+** (for building binaries, if needed)
   - Download from https://golang.org/dl/

## Building the Installer

### Quick Build

```bash
cd installer/macos
./build-pkg.sh 0.2.0 arm64
```

This will:
1. Locate the pre-built Janus and Executor binaries
2. Create PKG directory structure with `/usr/local/bin` installation target
3. Generate installation scripts (postinstall)
4. Create component package with pkgbuild
5. Build product package with productbuild
6. Sign the package with your Apple Developer ID (if certificate is available)
7. Generate SHA256 checksum
8. Output to `../../dist/janus-0.2.0-macos-arm64.pkg`

**Note**: The build script expects binaries to already be built. In CI, this is handled by the build-matrix job. For local builds:

```bash
cd /path/to/janus

# Build Janus (GUI) - for Apple Silicon
GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build -o janus-macos-arm64 \
  -ldflags "-X github.com/pharmalytica/janus/internal/version.Version=0.2.0" .

# Build Executor (CLI) - for Apple Silicon
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o executor-darwin-arm64 \
  -ldflags "-X github.com/pharmalytica/janus/internal/version.Version=0.2.0" ./cmd/executor
```

### Building for Different Architectures

**Apple Silicon (M1/M2/M3)**:
```bash
./build-pkg.sh 0.2.0 arm64
```

**Intel (x86_64)**:
```bash
./build-pkg.sh 0.2.0 amd64
```

## Code Signing

### Local Development Signing

For local builds with your Apple Developer ID certificate:

1. **Install your certificate** in Keychain Access
2. **Find the certificate name**:
   ```bash
   security find-identity -v
   ```
   Look for "Developer ID Installer: Your Name (TEAM_ID)"

3. **Set environment variable** (optional):
   ```bash
   export APPLE_CERTIFICATE_NAME="Developer ID Installer: Your Name (TEAM_ID)"
   ```

4. **Build with signing**:
   ```bash
   ./build-pkg.sh 0.2.0 arm64
   ```

The script will automatically detect and use your certificate.

### CI/CD Signing (GitHub Actions)

For CI builds, the certificate is provided as base64-encoded secrets:

1. **Export certificate from Keychain**:
   ```bash
   # Find your Developer ID Installer certificate
   security find-identity -v
   # Look for "Developer ID Installer: Your Name (TEAM_ID)"
   # Note the certificate hash (40-character hex string), then:
   security export -k ~/Library/Keychains/login.keychain-db \
     -t identities -f pkcs12 -o certificate.p12 -P "your-password" <certificate-hash>
   ```

2. **Encode certificate to base64**:
   ```bash
   base64 -i certificate.p12 | pbcopy
   # Now paste into GitHub Secrets as APPLE_CERTIFICATE_BASE64
   ```

3. **Set GitHub Secrets**:
   - `APPLE_CERTIFICATE_BASE64` - Base64-encoded .p12 certificate
   - `APPLE_CERTIFICATE_PASSWORD` - Password for the .p12 file
   - `APPLE_CERTIFICATE_NAME` - Certificate name (e.g., "Developer ID Installer")
   - `KEYCHAIN_PASSWORD` (optional) - Temporary keychain password for CI

The GitHub Actions workflow will:
- Decode the certificate
- Create a temporary keychain
- Import the certificate
- Sign the package
- Clean up the keychain

## Files in This Directory

- **`build-pkg.sh`** - Main build script
  - Locates binaries (janus + executor)
  - Creates package root structure
  - Generates installation scripts
  - Builds component and product packages
  - Signs package (if certificate available)
  - Generates checksums

- **`README.md`** - This file

## Package Features

### Installation Location
- Binaries: `/usr/local/bin/janus` and `/usr/local/bin/executor`
- Both executables are made executable (chmod +x)
- `/usr/local/bin` is typically already in PATH on macOS

### System Integration
- **Command Line**: `janus` and `executor` commands available system-wide
- **GUI Launch**: Users can run `janus` from Terminal to launch GUI
- **Standard PATH**: No manual PATH configuration needed

### Post-Installation Scripts

The installer includes a `postinstall` script that:
- Ensures binaries are executable
- Verifies installation to `/usr/local/bin`
- Displays success message with usage instructions

### Installer GUI

The PKG installer provides a standard macOS installation experience with:
- **Welcome screen** - Introduction to Janus and features
- **License agreement** - Project license (from repository root)
- **Installation type** - Standard install (customization disabled for simplicity)
- **Installation progress** - Standard macOS progress indicator
- **Conclusion screen** - Success message with usage instructions

## Testing the Installer

### Local Testing

1. **Build the installer** (see above)

2. **Install on test system**:
   ```bash
   # Double-click the PKG file, or:
   sudo installer -pkg dist/janus-0.2.0-macos-arm64.pkg -target /
   ```

3. **Verify installation**:
   - Check binaries exist:
     ```bash
     ls -la /usr/local/bin/janus
     ls -la /usr/local/bin/executor
     ```
   - Test commands:
     ```bash
     janus --version
     executor --version
     ```
   - Launch GUI:
     ```bash
     janus
     ```

4. **Verify signature** (if signed):
   ```bash
   pkgutil --check-signature dist/janus-0.2.0-macos-arm64.pkg
   ```

   Expected output (if properly signed):
   ```
   Package "janus-0.2.0-macos-arm64.pkg":
      Status: signed by a developer certificate issued by Apple
      Certificate Chain:
       1. Developer ID Installer: Your Name (TEAM_ID)
          SHA256 Fingerprint: ...
       2. Developer ID Certification Authority
          SHA256 Fingerprint: ...
       3. Apple Root CA
          SHA256 Fingerprint: ...
   ```

5. **Test installation workflow**:
   - Right-click PKG → "Open" (bypasses Gatekeeper warning if unsigned)
   - Follow installation wizard
   - Verify "Installation was successful" message
   - Open Terminal and run `janus --version`

### Uninstallation

macOS PKG installers don't include automatic uninstall. To remove Janus:

```bash
# Remove binaries
sudo rm -f /usr/local/bin/janus
sudo rm -f /usr/local/bin/executor

# Optionally remove user config (preserved by default)
rm -rf ~/.config/janus/
```

Or use a third-party uninstaller like [AppCleaner](https://freemacsoft.net/appcleaner/).

### Reinstallation / Upgrade

To upgrade to a newer version:

1. Simply install the new PKG over the old installation
2. The installer will replace the binaries in `/usr/local/bin`
3. User configuration in `~/.config/janus/` is preserved

No manual uninstall needed for upgrades.

## Architecture Support

The build script supports both Apple Silicon and Intel architectures:

- **arm64** - Apple Silicon (M1/M2/M3 chips)
  - Uses `janus-macos-arm64` and `executor-darwin-arm64` binaries
  - Targets `arm64` architecture in PKG metadata

- **amd64** (x86_64) - Intel Macs
  - Uses `janus-macos-amd64` and `executor-darwin-amd64` binaries
  - Targets `x86_64` architecture in PKG metadata

Universal binaries (combining both architectures) are not currently supported but could be added using `lipo`.

## Troubleshooting

### Build Fails: "Binary not found"

The build script expects binaries at:
- `../../janus-macos-{arch}` (repository root)
- `../../dist/macos/janus-macos-{arch}` (dist directory)
- `../../dist/janus-macos-{arch}` (dist directory root)

Make sure you've built the binaries first or are running in CI where they're pre-built.

### "pkgbuild: command not found"

Xcode Command Line Tools are not installed:
```bash
xcode-select --install
```

### Signature Verification Fails

If the package is unsigned:
- This is expected for local development builds without a certificate
- The package will install but show Gatekeeper warnings
- Users must right-click → "Open" to bypass Gatekeeper

To properly sign, you need:
1. Apple Developer account ($99/year)
2. Developer ID Installer certificate installed in Keychain
3. Certificate name set in `APPLE_CERTIFICATE_NAME` environment variable

### "Package is damaged" Error

This typically means:
1. The package wasn't signed, or
2. The signature is invalid, or
3. The certificate expired

For development, users can bypass by:
1. Right-click PKG → "Open" (instead of double-clicking)
2. Or: System Preferences → Security & Privacy → "Open Anyway"

### Installer Hangs or Fails

Check the installer log:
```bash
cat /var/log/install.log
```

Common issues:
- Insufficient permissions (use `sudo installer`)
- Disk space (PKG is ~50-100MB)
- Conflicting files in `/usr/local/bin` (manually remove old versions)

## CI/CD Integration

This installer is designed to be built in GitHub Actions. See `.github/workflows/release.yml` for the automated build pipeline.

Key workflow steps:
1. **Build binaries** - Previous job builds macOS binaries for both architectures
2. **Download artifacts** - Downloads janus and executor binaries
3. **Import certificate** - Decodes and imports signing certificate to temporary keychain
4. **Build PKG** - Runs `build-pkg.sh` with version and architecture
5. **Sign PKG** - Automatically signed if certificate is available
6. **Verify signature** - Validates the signature with `pkgutil`
7. **Cleanup keychain** - Removes temporary keychain
8. **Upload artifacts** - Uploads PKG and checksum to GitHub Releases

### Required GitHub Secrets

- `APPLE_CERTIFICATE_BASE64` - Base64-encoded .p12 certificate file
- `APPLE_CERTIFICATE_PASSWORD` - Password for the .p12 file
- `APPLE_CERTIFICATE_NAME` (optional) - Defaults to "Developer ID Installer"
- `KEYCHAIN_PASSWORD` (optional) - Defaults to temporary password

## Security Considerations

### Notarization

For distribution outside the Mac App Store, Apple requires **notarization** in addition to signing:

1. Sign the package (done by build script)
2. Submit to Apple for notarization:
   ```bash
   xcrun notarytool submit janus-0.2.0-macos-arm64.pkg \
     --apple-id "your-email@example.com" \
     --password "app-specific-password" \
     --team-id "TEAM_ID" \
     --wait
   ```
3. Staple the notarization ticket:
   ```bash
   xcrun stapler staple janus-0.2.0-macos-arm64.pkg
   ```

**Note**: Notarization is not currently automated in the build script but can be added to CI.

### Gatekeeper

macOS Gatekeeper will verify:
- Package signature (Developer ID)
- Notarization ticket (if notarized)

Unsigned packages will trigger warnings but can be opened via right-click → "Open".

## References

- [Creating macOS Installer Packages](https://developer.apple.com/library/archive/documentation/DeveloperTools/Reference/DistributionDefinitionRef/Chapters/Introduction.html)
- [pkgbuild man page](https://ss64.com/osx/pkgbuild.html)
- [productbuild man page](https://ss64.com/osx/productbuild.html)
- [Code Signing Guide](https://developer.apple.com/library/archive/documentation/Security/Conceptual/CodeSigningGuide/Introduction/Introduction.html)
- [Notarizing macOS Software](https://developer.apple.com/documentation/security/notarizing_macos_software_before_distribution)
