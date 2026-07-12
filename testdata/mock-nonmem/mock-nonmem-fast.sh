#!/bin/bash
# Mock NONMEM Executor for Integration Testing (FAST VERSION)
#
# This is an optimized version for CI/CD pipelines that skips output streaming
# and generates files immediately. Use this when you don't need to test streaming
# behavior and want fast test execution.
#
# Usage: mock-nonmem-fast.sh <model.mod> <output.lst> [additional args...]
#
# Speed comparison:
# - Regular version: ~14 seconds (realistic streaming)
# - Fast version: <0.5 seconds (instant output)

set -e

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STDOUT_FILE="$SCRIPT_DIR/STDOUT"

# Parse arguments (mimicking NONMEM command line)
MODEL_FILE="$1"
OUTPUT_FILE="$2"
shift 2  # Remaining args are additional options (e.g., -licfile, -PARAFILE)

# Validate required arguments
if [ -z "$MODEL_FILE" ] || [ -z "$OUTPUT_FILE" ]; then
    echo "Usage: $0 <model.mod> <output.lst> [options...]" >&2
    echo "Example: $0 acop.mod acop.lst -licfile=\${WORKSPACE}/nonmem.lic" >&2
    exit 1
fi

# Extract base name (without extension) and directory from output file path
OUTPUT_DIR="$(cd "$(dirname "$OUTPUT_FILE")" && pwd)"
BASE_NAME="$(basename "${OUTPUT_FILE%.lst}")"

# Output STDOUT immediately (no streaming delay)
cat "$STDOUT_FILE"

# Copy all output files to the output directory with proper naming
cp "$SCRIPT_DIR/acop.lst" "$OUTPUT_DIR/${BASE_NAME}.lst"
cp "$SCRIPT_DIR/acop.ext" "$OUTPUT_DIR/${BASE_NAME}.ext"
cp "$SCRIPT_DIR/acop.xml" "$OUTPUT_DIR/${BASE_NAME}.xml"
cp "$SCRIPT_DIR/acop.phi" "$OUTPUT_DIR/${BASE_NAME}.phi"

# Copy the model file if it doesn't exist
if [ ! -f "$OUTPUT_DIR/${BASE_NAME}.mod" ]; then
    cp "$MODEL_FILE" "$OUTPUT_DIR/${BASE_NAME}.mod"
fi

# Exit with success code
exit 0
