#!/bin/bash
#
# generate-license.sh - Generate a Janus license token
#
# Usage:
#   ./scripts/generate-license.sh [options]
#
# Options:
#   -h, --host              License server host (default: http://localhost:8443)
#   -a, --agreement-id      Agreement ID (default: 1)
#   -e, --email             User email (default: darrell.breeden@pharmalytica.io)
#   -d, --days              Token validity in days (default: 365)
#   --signing-key           Path to client's RSA public key PEM file (for run log signing)
#   --output                Output file for the generated token (default: stdout)
#   --auth-token            Bearer token for authentication
#
# Environment Variables:
#   LICENSE_SERVER_URL    - Override default server URL
#   LICENSE_AUTH_TOKEN    - Bearer token for authentication
#
# Examples:
#   # Basic license generation
#   ./scripts/generate-license.sh
#
#   # With signing public key for CFR 21 Part 11 compliance
#   ./scripts/generate-license.sh --signing-key ~/.config/janus/signing-key.pub
#
#   # Save token to file
#   ./scripts/generate-license.sh --output ~/.config/janus/license.jwt

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Defaults
HOST="${LICENSE_SERVER_URL:-http://localhost:8443}"
AGREEMENT_ID=1
USER_EMAIL="darrell.breeden@pharmalytica.io"
DAYS=365
SIGNING_KEY_FILE=""
OUTPUT=""
AUTH_TOKEN="${LICENSE_AUTH_TOKEN:-}"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--host)
            HOST="$2"
            shift 2
            ;;
        -a|--agreement-id)
            AGREEMENT_ID="$2"
            shift 2
            ;;
        -e|--email)
            USER_EMAIL="$2"
            shift 2
            ;;
        -d|--days)
            DAYS="$2"
            shift 2
            ;;
        --signing-key)
            SIGNING_KEY_FILE="$2"
            shift 2
            ;;
        --output)
            OUTPUT="$2"
            shift 2
            ;;
        --auth-token)
            AUTH_TOKEN="$2"
            shift 2
            ;;
        --help)
            head -30 "$0" | tail -25
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            exit 1
            ;;
    esac
done

# Calculate duration in seconds
DURATION_SECONDS=$((DAYS * 24 * 60 * 60))

echo -e "${BLUE}Janus License Generator${NC}"
echo "================================"
echo -e "Server:       ${YELLOW}$HOST${NC}"
echo -e "Agreement ID: ${YELLOW}$AGREEMENT_ID${NC}"
echo -e "Email:        ${YELLOW}$USER_EMAIL${NC}"
echo -e "Validity:     ${YELLOW}$DAYS days${NC}"
if [[ -n "$SIGNING_KEY_FILE" ]]; then
    echo -e "Signing Key:  ${YELLOW}$SIGNING_KEY_FILE${NC}"
fi
echo ""

# Check server health
echo -e "${BLUE}Checking server health...${NC}"
HEALTH=$(curl -s -w "%{http_code}" -o /tmp/health_response.json "$HOST/health" 2>/dev/null || echo "000")
if [[ "$HEALTH" != "200" ]]; then
    echo -e "${RED}Server not responding at $HOST${NC}"
    echo "Make sure the license server is running"
    exit 1
fi
echo -e "${GREEN}Server is healthy${NC}"
echo ""

# Build the request JSON
REQUEST_JSON="{
    \"agreement_id\": $AGREEMENT_ID,
    \"user_email\": \"$USER_EMAIL\",
    \"duration_seconds\": $DURATION_SECONDS"

# Add signing public key if provided
if [[ -n "$SIGNING_KEY_FILE" ]]; then
    if [[ ! -f "$SIGNING_KEY_FILE" ]]; then
        echo -e "${RED}Signing public key file not found: $SIGNING_KEY_FILE${NC}"
        exit 1
    fi
    # Read and escape the PEM file for JSON
    SIGNING_KEY_PEM=$(cat "$SIGNING_KEY_FILE")
    # Escape newlines and quotes for JSON
    SIGNING_KEY_ESCAPED=$(echo "$SIGNING_KEY_PEM" | sed ':a;N;$!ba;s/\n/\\n/g' | sed 's/"/\\"/g')
    REQUEST_JSON="$REQUEST_JSON,
    \"signing_public_key\": \"$SIGNING_KEY_ESCAPED\""
    echo -e "${GREEN}Including signing public key from: $SIGNING_KEY_FILE${NC}"
fi

REQUEST_JSON="$REQUEST_JSON
}"

# Generate token
echo -e "${BLUE}Generating license token...${NC}"

if [[ -n "$AUTH_TOKEN" ]]; then
    TOKEN_RESPONSE=$(curl -s -X POST "$HOST/api/v1/tokens" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $AUTH_TOKEN" \
        -d "$REQUEST_JSON")
else
    TOKEN_RESPONSE=$(curl -s -X POST "$HOST/api/v1/tokens" \
        -H "Content-Type: application/json" \
        -d "$REQUEST_JSON")
fi

TOKEN=$(echo "$TOKEN_RESPONSE" | jq -r '.token // empty')
EXPIRES_AT=$(echo "$TOKEN_RESPONSE" | jq -r '.expires_at // empty')

if [[ -z "$TOKEN" ]]; then
    echo -e "${RED}Failed to generate token${NC}"
    echo "Response: $TOKEN_RESPONSE"
    exit 1
fi

echo -e "${GREEN}Token generated successfully!${NC}"
echo -e "Expires: ${YELLOW}$EXPIRES_AT${NC}"
echo ""

# Output token
if [[ -n "$OUTPUT" ]]; then
    echo "$TOKEN" > "$OUTPUT"
    echo -e "${GREEN}Token saved to: $OUTPUT${NC}"
else
    echo -e "${BLUE}License Token:${NC}"
    echo "----------------------------------------"
    echo "$TOKEN"
    echo "----------------------------------------"
fi
echo ""

# Decode and display token claims
echo -e "${BLUE}Token Claims:${NC}"
echo "$TOKEN" | cut -d'.' -f2 | base64 -d 2>/dev/null | jq . 2>/dev/null || echo "(Could not decode claims)"
echo ""

echo -e "${GREEN}Done!${NC}"
