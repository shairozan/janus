#!/bin/bash
set -e

# Build script for macOS PKG installer
# Usage: ./build-pkg.sh <version> <arch>
# Example: ./build-pkg.sh 0.2.0 arm64

VERSION=${1:-"0.0.0"}
ARCH=${2:-"arm64"}  # arm64 or amd64
INSTALLER_CERT_NAME="${APPLE_CERTIFICATE_NAME:-Developer ID Installer}"
APPLICATION_CERT_NAME="${APPLE_APPLICATION_CERT_NAME:-Developer ID Application}"

echo "=========================================="
echo "Building Janus macOS PKG Installer"
echo "=========================================="
echo "Version: $VERSION"
echo "Architecture: $ARCH"
echo "Application Certificate: $APPLICATION_CERT_NAME"
echo "Installer Certificate: $INSTALLER_CERT_NAME"
echo ""

# Determine binary names
if [ "$ARCH" = "arm64" ]; then
    JANUS_BINARY="janus-macos-arm64"
    EXECUTOR_BINARY="executor-darwin-arm64"
    PKG_ARCH="arm64"
elif [ "$ARCH" = "amd64" ]; then
    JANUS_BINARY="janus-macos-amd64"
    EXECUTOR_BINARY="executor-darwin-amd64"
    PKG_ARCH="x86_64"
else
    echo "ERROR: Unknown architecture: $ARCH"
    echo "Must be 'arm64' or 'amd64'"
    exit 1
fi

# Paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$SCRIPT_DIR/../.."
DIST_DIR="$REPO_ROOT/dist"
BUILD_DIR="$DIST_DIR/macos-pkg-build-$ARCH"
PKG_ROOT="$BUILD_DIR/package-root"
SCRIPTS_DIR="$BUILD_DIR/scripts"

# Output paths
PKG_NAME="janus-$VERSION-macos-$ARCH.pkg"
PKG_PATH="$DIST_DIR/$PKG_NAME"
CHECKSUM_PATH="$PKG_PATH.sha256"

echo "🔍 Locating binaries..."

# Find Janus binary
JANUS_SOURCE=""
if [ -f "$REPO_ROOT/$JANUS_BINARY" ]; then
    JANUS_SOURCE="$REPO_ROOT/$JANUS_BINARY"
elif [ -f "$DIST_DIR/macos/$JANUS_BINARY" ]; then
    JANUS_SOURCE="$DIST_DIR/macos/$JANUS_BINARY"
elif [ -f "$DIST_DIR/$JANUS_BINARY" ]; then
    JANUS_SOURCE="$DIST_DIR/$JANUS_BINARY"
else
    echo "❌ ERROR: Janus binary not found: $JANUS_BINARY"
    echo "Searched locations:"
    echo "  - $REPO_ROOT/$JANUS_BINARY"
    echo "  - $DIST_DIR/macos/$JANUS_BINARY"
    echo "  - $DIST_DIR/$JANUS_BINARY"
    exit 1
fi

# Find executor binary
EXECUTOR_SOURCE=""
if [ -f "$REPO_ROOT/$EXECUTOR_BINARY" ]; then
    EXECUTOR_SOURCE="$REPO_ROOT/$EXECUTOR_BINARY"
elif [ -f "$DIST_DIR/macos/$EXECUTOR_BINARY" ]; then
    EXECUTOR_SOURCE="$DIST_DIR/macos/$EXECUTOR_BINARY"
elif [ -f "$DIST_DIR/$EXECUTOR_BINARY" ]; then
    EXECUTOR_SOURCE="$DIST_DIR/$EXECUTOR_BINARY"
else
    echo "❌ ERROR: Executor binary not found: $EXECUTOR_BINARY"
    echo "Searched locations:"
    echo "  - $REPO_ROOT/$EXECUTOR_BINARY"
    echo "  - $DIST_DIR/macos/$EXECUTOR_BINARY"
    echo "  - $DIST_DIR/$EXECUTOR_BINARY"
    exit 1
fi

echo "✅ Found Janus: $JANUS_SOURCE"
echo "✅ Found Executor: $EXECUTOR_SOURCE"
echo ""

# Clean and create build directories
echo "📁 Creating package structure..."
rm -rf "$BUILD_DIR"
mkdir -p "$PKG_ROOT/usr/local/bin"
mkdir -p "$PKG_ROOT/Applications"
mkdir -p "$SCRIPTS_DIR"
mkdir -p "$DIST_DIR"

# Copy binaries
echo "📦 Copying binaries..."
cp "$JANUS_SOURCE" "$PKG_ROOT/usr/local/bin/janus"
cp "$EXECUTOR_SOURCE" "$PKG_ROOT/usr/local/bin/executor"
chmod +x "$PKG_ROOT/usr/local/bin/janus"
chmod +x "$PKG_ROOT/usr/local/bin/executor"

# Code sign binaries if certificate is available
if [ -n "$APPLE_CERTIFICATE_BASE64" ]; then
    echo "🔐 Code signing binaries with Apple Developer ID..."

    # Check if the Application signing identity is available
    if security find-identity -v | grep -q "$APPLICATION_CERT_NAME"; then
        echo "✅ Found Application certificate: $APPLICATION_CERT_NAME"

        # Sign janus binary
        echo "  Signing janus binary..."
        codesign \
            --sign "$APPLICATION_CERT_NAME" \
            --options runtime \
            --timestamp \
            --force \
            "$PKG_ROOT/usr/local/bin/janus"

        # Sign executor binary
        echo "  Signing executor binary..."
        codesign \
            --sign "$APPLICATION_CERT_NAME" \
            --options runtime \
            --timestamp \
            --force \
            "$PKG_ROOT/usr/local/bin/executor"

        # Verify signatures
        echo "🔍 Verifying binary signatures..."
        codesign --verify --verbose "$PKG_ROOT/usr/local/bin/janus"
        codesign --verify --verbose "$PKG_ROOT/usr/local/bin/executor"

        echo "✅ Binaries signed successfully"
    else
        echo "⚠️  WARNING: Application certificate not found in keychain"
        echo "Available identities:"
        security find-identity -v || echo "  (none)"
        echo ""
        echo "Binaries will not be signed. Notarization may fail."
    fi
else
    echo "ℹ️  No certificate provided (APPLE_CERTIFICATE_BASE64 not set)"
    echo "Binaries will not be signed."
fi
echo ""

# Create postinstall script
echo "📝 Creating installation scripts..."
cat > "$SCRIPTS_DIR/postinstall" <<'EOF'
#!/bin/bash

# Ensure binaries are executable
chmod +x /usr/local/bin/janus
chmod +x /usr/local/bin/executor

# Create symlinks in /usr/local/bin if they don't exist
if [ ! -L "/usr/local/bin/janus" ] && [ -f "/usr/local/bin/janus" ]; then
    echo "Janus installed to /usr/local/bin/janus"
fi

if [ ! -L "/usr/local/bin/executor" ] && [ -f "/usr/local/bin/executor" ]; then
    echo "Executor installed to /usr/local/bin/executor"
fi

# Ensure /usr/local/bin is in PATH (add to shell profiles if needed)
echo "Installation complete!"
echo ""
echo "Janus has been installed to /usr/local/bin/janus"
echo "Executor has been installed to /usr/local/bin/executor"
echo ""
echo "You can now run 'janus' from the command line."

exit 0
EOF

chmod +x "$SCRIPTS_DIR/postinstall"

# Build the component package
echo "🔨 Building component package..."
COMPONENT_PKG="$BUILD_DIR/janus-component.pkg"

pkgbuild \
    --root "$PKG_ROOT" \
    --scripts "$SCRIPTS_DIR" \
    --identifier "io.github.shairozan.janus" \
    --version "$VERSION" \
    --install-location "/" \
    "$COMPONENT_PKG"

if [ ! -f "$COMPONENT_PKG" ]; then
    echo "❌ ERROR: Component package build failed"
    exit 1
fi

echo "✅ Component package created"

# Create distribution XML for productbuild
echo "📝 Creating distribution definition..."
DISTRIBUTION_XML="$BUILD_DIR/distribution.xml"

cat > "$DISTRIBUTION_XML" <<EOF
<?xml version="1.0" encoding="utf-8"?>
<installer-gui-script minSpecVersion="1">
    <title>Janus</title>
    <organization>io.github.shairozan</organization>
    <domains enable_localSystem="true"/>
    <options customize="never" require-scripts="false" hostArchitectures="$PKG_ARCH"/>

    <!-- Define documents displayed at various steps -->
    <welcome file="welcome.html" mime-type="text/html"/>
    <license file="license.txt"/>
    <conclusion file="conclusion.html" mime-type="text/html"/>

    <!-- Define the installer choices -->
    <choices-outline>
        <line choice="default">
            <line choice="io.github.shairozan.janus"/>
        </line>
    </choices-outline>

    <choice id="default"/>
    <choice id="io.github.shairozan.janus" visible="false">
        <pkg-ref id="io.github.shairozan.janus"/>
    </choice>

    <pkg-ref id="io.github.shairozan.janus" version="$VERSION" onConclusion="none">janus-component.pkg</pkg-ref>

</installer-gui-script>
EOF

# Create welcome message
cat > "$BUILD_DIR/welcome.html" <<EOF
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8"/>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif; }
        h1 { color: #1d1d1f; }
        p { color: #424245; line-height: 1.5; }
    </style>
</head>
<body>
    <h1>Welcome to Janus $VERSION</h1>
    <p>This installer will install Janus, a GUI replacement for Certara Pirana built with Go and the Fyne framework.</p>
    <p>Janus provides comprehensive support for:</p>
    <ul>
        <li>Loading and managing pharmacometric models</li>
        <li>Local and remote execution (SLURM/grid schedulers)</li>
        <li>Execution tracking and audit logs</li>
        <li>Configuration management</li>
    </ul>
    <p>Click Continue to begin the installation.</p>
</body>
</html>
EOF

# Create license file
if [ -f "$REPO_ROOT/LICENSE" ]; then
    cp "$REPO_ROOT/LICENSE" "$BUILD_DIR/license.txt"
elif [ -f "$REPO_ROOT/LICENSE.md" ]; then
    cp "$REPO_ROOT/LICENSE.md" "$BUILD_DIR/license.txt"
else
    echo "See https://github.com/shairozan/janus for license terms." > "$BUILD_DIR/license.txt"
fi

# Create conclusion message
cat > "$BUILD_DIR/conclusion.html" <<EOF
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8"/>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif; }
        h1 { color: #1d1d1f; }
        p { color: #424245; line-height: 1.5; }
        code { background-color: #f5f5f7; padding: 2px 6px; border-radius: 3px; font-family: 'SF Mono', Monaco, monospace; }
    </style>
</head>
<body>
    <h1>Installation Complete</h1>
    <p>Janus has been successfully installed!</p>
    <p>The following binaries are now available in <code>/usr/local/bin</code>:</p>
    <ul>
        <li><code>janus</code> - Main GUI application</li>
        <li><code>executor</code> - Command-line executor for grid systems</li>
    </ul>
    <p>You can launch Janus by running <code>janus</code> from the Terminal.</p>
    <p>For more information, visit the <a href="https://github.com/shairozan/janus">Janus GitHub repository</a>.</p>
</body>
</html>
EOF

# Build the product package (unsigned)
echo "🔨 Building product package..."
UNSIGNED_PKG="$BUILD_DIR/janus-unsigned.pkg"

productbuild \
    --distribution "$DISTRIBUTION_XML" \
    --package-path "$BUILD_DIR" \
    --resources "$BUILD_DIR" \
    "$UNSIGNED_PKG"

if [ ! -f "$UNSIGNED_PKG" ]; then
    echo "❌ ERROR: Product package build failed"
    exit 1
fi

echo "✅ Product package created"

# Sign the package if certificate is available
if [ -n "$APPLE_CERTIFICATE_BASE64" ]; then
    echo "🔐 Attempting to sign package with Apple Developer ID..."
    echo "Looking for identity: $INSTALLER_CERT_NAME"

    # Check if the signing identity is available in the keychain
    # Note: Don't use -p codesigning filter as Developer ID Installer certs may not show up
    if security find-identity -v | grep -q "$INSTALLER_CERT_NAME"; then
        echo "✅ Found signing identity in keychain"

        productsign \
            --sign "$INSTALLER_CERT_NAME" \
            "$UNSIGNED_PKG" \
            "$PKG_PATH"

        if [ $? -eq 0 ]; then
            echo "✅ Package signed successfully"

            # Verify the signature
            echo "🔍 Verifying signature..."
            pkgutil --check-signature "$PKG_PATH"
        else
            echo "⚠️  WARNING: Package signing failed, using unsigned package"
            cp "$UNSIGNED_PKG" "$PKG_PATH"
        fi
    else
        echo "❌ ERROR: Signing identity '$INSTALLER_CERT_NAME' not found in keychain"
        echo ""
        echo "Available identities:"
        security find-identity -v || echo "  (none)"
        echo ""
        echo "Certificate import may have failed. Check APPLE_CERTIFICATE_BASE64 and APPLE_CERTIFICATE_PASSWORD."
        exit 1
    fi
else
    echo "ℹ️  No certificate provided (APPLE_CERTIFICATE_BASE64 not set), creating unsigned package"
    cp "$UNSIGNED_PKG" "$PKG_PATH"
fi

# Generate checksum
echo "🔐 Generating SHA256 checksum..."
cd "$DIST_DIR"
shasum -a 256 "$PKG_NAME" | awk '{print $1}' > "$CHECKSUM_PATH"
cd - > /dev/null

# Cleanup build directory
echo "🧹 Cleaning up build directory..."
rm -rf "$BUILD_DIR"

# Final output
echo ""
echo "=========================================="
echo "✅ Build Complete!"
echo "=========================================="
echo "Package: $PKG_PATH"
echo "Checksum: $CHECKSUM_PATH"
echo ""
ls -lh "$PKG_PATH"
echo ""
echo "SHA256: $(cat "$CHECKSUM_PATH")"
echo "=========================================="
