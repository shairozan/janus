#!/bin/bash
# Cleanup Script for Model Directory
#
# This script removes all files from a model directory EXCEPT:
# - The model file (.mod, .ctl, .nmctl)
# - The Hermes configuration file (.janus.config.json)
# - The data file referenced in the model
# - Script files (.sh) - these are infrastructure
#
# Usage: cleanup-model-dir.sh [directory]
#
# If no directory is specified, operates on current directory.
# This is useful for cleaning up after test runs while preserving
# the essential files needed for re-execution.
#
# Typical files removed (NONMEM output):
# - *.lst, *.ext, *.xml, *.phi, *.cov, *.cor, *.coi
# - *.shk, *.shm, *.grd, *.cpu, *.tab
# - INTER, LINK, FCON, FDATA, FSTREAM, FREPORT
# - fort.* files, temporary files, logs

set -e

# Determine target directory
TARGET_DIR="${1:-.}"

if [ ! -d "$TARGET_DIR" ]; then
    echo "Error: Directory '$TARGET_DIR' does not exist" >&2
    exit 1
fi

cd "$TARGET_DIR"

echo "Cleaning up model directory: $(pwd)"

# Find the model file (first .mod, .ctl, or .nmctl file)
MODEL_FILE=$(find . -maxdepth 1 -type f \( -name "*.mod" -o -name "*.ctl" -o -name "*.nmctl" \) | head -n 1 | sed 's|^\./||')

if [ -z "$MODEL_FILE" ]; then
    echo "Warning: No model file (.mod, .ctl, .nmctl) found in directory" >&2
    echo "Will only preserve .janus.config.json" >&2
fi

# Try to extract data file from model if it exists
DATA_FILE=""
if [ -n "$MODEL_FILE" ] && [ -f "$MODEL_FILE" ]; then
    # Extract data file from $DATA directive (case insensitive)
    DATA_FILE=$(grep -i '^\$DATA' "$MODEL_FILE" | head -n 1 | awk '{print $2}' | tr -d '\r')

    if [ -n "$DATA_FILE" ]; then
        # Handle relative paths - get just the filename if it's in current dir
        DATA_FILE=$(basename "$DATA_FILE")
    fi
fi

echo "Files to preserve:"
echo "  - .janus.config.json (Hermes config)"
[ -n "$MODEL_FILE" ] && echo "  - $MODEL_FILE (model file)"
[ -n "$DATA_FILE" ] && [ -f "$DATA_FILE" ] && echo "  - $DATA_FILE (data file)"

# Count files before cleanup
BEFORE_COUNT=$(find . -maxdepth 1 -type f | wc -l)

# Remove all files except the ones we want to keep
find . -maxdepth 1 -type f | while read -r file; do
    # Remove leading ./
    file=$(echo "$file" | sed 's|^\./||')

    # Skip if it's the config file
    [ "$file" = ".janus.config.json" ] && continue

    # Skip if it's the model file
    [ -n "$MODEL_FILE" ] && [ "$file" = "$MODEL_FILE" ] && continue

    # Skip if it's the data file
    [ -n "$DATA_FILE" ] && [ "$file" = "$DATA_FILE" ] && continue

    # Skip shell scripts (infrastructure)
    case "$file" in
        *.sh)
            continue
            ;;
    esac

    # Remove the file (NONMEM output, logs, etc.)
    echo "Removing: $file"
    rm -f "$file"
done

# Count files after cleanup
AFTER_COUNT=$(find . -maxdepth 1 -type f | wc -l)
REMOVED=$((BEFORE_COUNT - AFTER_COUNT))

echo ""
echo "Cleanup complete!"
echo "  Files before: $BEFORE_COUNT"
echo "  Files after:  $AFTER_COUNT"
echo "  Files removed: $REMOVED"
