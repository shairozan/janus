#!/bin/bash
set -e

VERSION="${1:-0.2.0}"
TARGET_NAME="${2:-ubuntu2404}"
OUTPUT_DIR="${3:-../../dist}"
ARCH="amd64"
PKG_NAME="janus_${VERSION}_${TARGET_NAME}_${ARCH}"
BUILD_DIR="${OUTPUT_DIR}/linux/${PKG_NAME}"

echo "Building Janus DEB package v${VERSION} for ${TARGET_NAME} (${ARCH})"

# Clean previous build
rm -rf "${BUILD_DIR}"

# Create directory structure
mkdir -p "${BUILD_DIR}/DEBIAN"
mkdir -p "${BUILD_DIR}/usr/bin"
mkdir -p "${BUILD_DIR}/usr/share/applications"
mkdir -p "${BUILD_DIR}/usr/share/icons/hicolor/48x48/apps"
mkdir -p "${BUILD_DIR}/usr/share/icons/hicolor/128x128/apps"
mkdir -p "${BUILD_DIR}/usr/share/icons/hicolor/256x256/apps"
mkdir -p "${BUILD_DIR}/usr/share/mime/packages"
mkdir -p "${BUILD_DIR}/usr/share/doc/janus"

# Check for pre-built janus binary (from CI) or build it
if [ -f "../../janus-${TARGET_NAME}" ]; then
    echo "✅ Using pre-built binary: janus-${TARGET_NAME}"
    cp "../../janus-${TARGET_NAME}" "${BUILD_DIR}/usr/bin/janus"
elif [ -f "../../dist/linux/janus-${TARGET_NAME}" ]; then
    echo "✅ Using pre-built binary from dist: dist/linux/janus-${TARGET_NAME}"
    cp "../../dist/linux/janus-${TARGET_NAME}" "${BUILD_DIR}/usr/bin/janus"
else
    echo "Janus binary not found, building for Linux amd64..."

    # Build the binary
    cd ../..
    GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build \
        -o "janus-${TARGET_NAME}" \
        -ldflags "-X github.com/shairozan/janus/internal/version.Version=${VERSION}" \
        .

    if [ $? -ne 0 ]; then
        echo "❌ ERROR: Go build failed"
        exit 1
    fi

    cd installer/linux

    # Copy the newly built binary
    cp "../../janus-${TARGET_NAME}" "${BUILD_DIR}/usr/bin/janus"
    echo "✅ Binary built and copied: janus-${TARGET_NAME}"
fi

chmod 755 "${BUILD_DIR}/usr/bin/janus"

# Check for pre-built executor binary (from CI) or build it
EXECUTOR_BINARY="executor-linux-amd64"
if [ -f "../../${EXECUTOR_BINARY}" ]; then
    echo "✅ Using pre-built executor binary: ${EXECUTOR_BINARY}"
    cp "../../${EXECUTOR_BINARY}" "${BUILD_DIR}/usr/bin/executor"
elif [ -f "../../dist/linux/${EXECUTOR_BINARY}" ]; then
    echo "✅ Using pre-built executor binary from dist: dist/linux/${EXECUTOR_BINARY}"
    cp "../../dist/linux/${EXECUTOR_BINARY}" "${BUILD_DIR}/usr/bin/executor"
else
    echo "Executor binary not found, building for Linux amd64..."

    # Build the executor binary
    cd ../..
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build \
        -o "${EXECUTOR_BINARY}" \
        -ldflags "-X github.com/shairozan/janus/internal/version.Version=${VERSION}" \
        ./cmd/executor

    if [ $? -ne 0 ]; then
        echo "❌ ERROR: Executor build failed"
        exit 1
    fi

    cd installer/linux

    # Copy the newly built binary
    cp "../../${EXECUTOR_BINARY}" "${BUILD_DIR}/usr/bin/executor"
    echo "✅ Executor built and copied: ${EXECUTOR_BINARY}"
fi

chmod 755 "${BUILD_DIR}/usr/bin/executor"

# Generate control file from template
sed "s/{{VERSION}}/${VERSION}/g" control.template > "${BUILD_DIR}/DEBIAN/control"

# Copy maintenance scripts
cp postinst "${BUILD_DIR}/DEBIAN/"
cp postrm "${BUILD_DIR}/DEBIAN/"
chmod 755 "${BUILD_DIR}/DEBIAN/postinst"
chmod 755 "${BUILD_DIR}/DEBIAN/postrm"

# Copy desktop integration files
cp janus.desktop "${BUILD_DIR}/usr/share/applications/"
cp janus.xml "${BUILD_DIR}/usr/share/mime/packages/"

# Generate PNG icons from logo.png if they don't exist
if [ -f "../../assets/logo.png" ]; then
    echo "Generating PNG icons from logo.png..."

    # Check if ImageMagick is available
    if command -v magick >/dev/null 2>&1 || command -v convert >/dev/null 2>&1; then
        # Use 'magick convert' if available (ImageMagick 7+), otherwise 'convert' (ImageMagick 6)
        CONVERT_CMD="convert"
        if command -v magick >/dev/null 2>&1; then
            CONVERT_CMD="magick convert"
        fi

        $CONVERT_CMD "../../assets/logo.png" -resize 48x48 "../../assets/icons/janus-48.png"
        $CONVERT_CMD "../../assets/logo.png" -resize 128x128 "../../assets/icons/janus-128.png"
        $CONVERT_CMD "../../assets/logo.png" -resize 256x256 "../../assets/icons/janus-256.png"

        echo "✅ Icons generated"
    else
        echo "⚠️  WARNING: ImageMagick not found, skipping icon generation"
        echo "   Icons can be added manually to assets/icons/"
    fi
fi

# Copy icons (if they exist)
if [ -f "../../assets/icons/janus-48.png" ]; then
    cp "../../assets/icons/janus-48.png" "${BUILD_DIR}/usr/share/icons/hicolor/48x48/apps/janus.png"
    echo "✅ Copied 48x48 icon"
fi

if [ -f "../../assets/icons/janus-128.png" ]; then
    cp "../../assets/icons/janus-128.png" "${BUILD_DIR}/usr/share/icons/hicolor/128x128/apps/janus.png"
    echo "✅ Copied 128x128 icon"
fi

if [ -f "../../assets/icons/janus-256.png" ]; then
    cp "../../assets/icons/janus-256.png" "${BUILD_DIR}/usr/share/icons/hicolor/256x256/apps/janus.png"
    echo "✅ Copied 256x256 icon"
fi

# Copy documentation
cp ../../LICENSE "${BUILD_DIR}/usr/share/doc/janus/copyright"
if [ -f "../../CHANGELOG.md" ]; then
    gzip -9 -c ../../CHANGELOG.md > "${BUILD_DIR}/usr/share/doc/janus/changelog.gz"
fi

# Set proper permissions
find "${BUILD_DIR}" -type d -exec chmod 755 {} \;
find "${BUILD_DIR}/usr/share" -type f -exec chmod 644 {} \;

# Build DEB package
echo "Building DEB package..."
dpkg-deb --build "${BUILD_DIR}"

# Move to output directory
OUTPUT_FILE="janus_${VERSION}_${TARGET_NAME}_amd64.deb"
mv "${BUILD_DIR}.deb" "${OUTPUT_DIR}/${OUTPUT_FILE}"

# Generate checksum
cd "${OUTPUT_DIR}"
sha256sum "${OUTPUT_FILE}" > "${OUTPUT_FILE}.sha256"

echo "✅ DEB package built successfully!"
echo "  Location: ${OUTPUT_DIR}/${OUTPUT_FILE}"
echo "  SHA256: $(cat ${OUTPUT_FILE}.sha256)"
echo "  Includes: janus, executor"
