#!/bin/bash
# Mock NONMEM Executor for Integration Testing
#
# This script emulates a NONMEM run by:
# 1. Streaming STDOUT output slowly (like a real NONMEM run)
# 2. Producing all output files (xml, lst, ext, phi, etc.)
# 3. Exiting with success code (0)
#
# Usage: mock-nonmem.sh <model.mod> <output.lst> [additional args...]
#
# This allows integration testing without requiring an actual NONMEM license.

set -e

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STDOUT_FILE="$SCRIPT_DIR/STDOUT"
LINES_PER_SECOND=5  # Simulate realistic NONMEM execution speed
SLEEP_INTERVAL=$(echo "scale=3; 1.0 / $LINES_PER_SECOND" | bc)

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
# NONMEM writes outputs to the same directory as the output file, not the model file
OUTPUT_DIR="$(cd "$(dirname "$OUTPUT_FILE")" && pwd)"
BASE_NAME="$(basename "${OUTPUT_FILE%.lst}")"

# Stream STDOUT line by line to simulate NONMEM execution
echo "Mock NONMEM starting (streaming output at $LINES_PER_SECOND lines/sec)..." >&2

while IFS= read -r line; do
    echo "$line"
    sleep "$SLEEP_INTERVAL"
done < "$STDOUT_FILE"

echo "Mock NONMEM output complete, generating files..." >&2

# Copy all output files to the output directory with proper naming
# These are the typical NONMEM output files
cp "$SCRIPT_DIR/acop.lst" "$OUTPUT_DIR/${BASE_NAME}.lst"
cp "$SCRIPT_DIR/acop.ext" "$OUTPUT_DIR/${BASE_NAME}.ext"
cp "$SCRIPT_DIR/acop.xml" "$OUTPUT_DIR/${BASE_NAME}.xml"
cp "$SCRIPT_DIR/acop.phi" "$OUTPUT_DIR/${BASE_NAME}.phi"

# Copy the model file if it doesn't exist (NONMEM does this)
if [ ! -f "$OUTPUT_DIR/${BASE_NAME}.mod" ]; then
    cp "$MODEL_FILE" "$OUTPUT_DIR/${BASE_NAME}.mod"
fi

echo "Mock NONMEM complete: generated ${BASE_NAME}.{lst,ext,xml,phi}" >&2

# Exit with success code (mimicking successful NONMEM execution)
exit 0
