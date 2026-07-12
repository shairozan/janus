#!/bin/bash
# License Server Workflow Script
# Complete workflow to generate signing keys, create test data, and build Janus with embedded public key

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
BASE_URL="${LICENSE_SERVER_URL:-http://localhost:8443}"
LICENSE_OUTPUT="${HOME}/.config/janus/license.jwt"

# Helper functions
print_header() {
    echo -e "\n${BLUE}==== $1 ====${NC}\n"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

print_info() {
    echo -e "${YELLOW}ℹ $1${NC}"
}

# Check if jq is installed
if ! command -v jq &> /dev/null; then
    print_error "jq is required but not installed. Install with: sudo apt-get install jq"
    exit 1
fi

# Function to check server health
check_server() {
    print_header "Checking License Server Health"

    HEALTH=$(curl -s "${BASE_URL}/health" || echo '{"status":"error"}')
    STATUS=$(echo "$HEALTH" | jq -r '.status')

    if [ "$STATUS" = "healthy" ]; then
        print_success "License server is healthy"
        echo "$HEALTH" | jq '.'
        return 0
    else
        print_error "License server is not healthy or not running"
        print_info "Start the server with: cd cmd/license-server && ./license-server serve"
        return 1
    fi
}

# Function to generate/rotate master signing key
generate_master_key() {
    print_header "Generating Master Signing Key"

    RESPONSE=$(curl -s -X POST "${BASE_URL}/api/v1/keys/rotate" \
        -H "Content-Type: application/json" \
        -d '{
            "expires_in_days": 365
        }')

    NEW_KEY_ID=$(echo "$RESPONSE" | jq -r '.new_key_id')

    if [ "$NEW_KEY_ID" != "null" ] && [ -n "$NEW_KEY_ID" ]; then
        print_success "Master signing key generated: $NEW_KEY_ID"
        echo "$RESPONSE" | jq '.'
        return 0
    else
        print_error "Failed to generate master key"
        echo "$RESPONSE" | jq '.'
        return 1
    fi
}

# Function to list all signing keys
list_keys() {
    print_header "Listing All Signing Keys"

    RESPONSE=$(curl -s "${BASE_URL}/api/v1/keys")

    echo "$RESPONSE" | jq '.'
}

# Function to get the active master public key
get_master_public_key() {
    print_header "Fetching Master Public Key"

    # Get all keys and find the active master key (organization_id = null)
    KEYS=$(curl -s "${BASE_URL}/api/v1/keys")

    # Find active master key (no organization_id and active = true)
    MASTER_KEY=$(echo "$KEYS" | jq -r '.[] | select(.organization_id == null and .active == true) | .public_key_pem' | head -1)

    if [ -n "$MASTER_KEY" ]; then
        print_success "Retrieved master public key"
        echo "$MASTER_KEY"
        return 0
    else
        print_error "No active master key found"
        return 1
    fi
}

# Function to create an organization
create_organization() {
    local ORG_NAME="${1:-Test Organization}"
    local CUSTOMER_ID="${2:-$(echo "$ORG_NAME" | tr '[:upper:]' '[:lower:]' | tr ' ' '-')}"

    print_header "Creating Organization: $ORG_NAME"

    RESPONSE=$(curl -s -X POST "${BASE_URL}/api/v1/organizations" \
        -H "Content-Type: application/json" \
        -d "{
            \"name\": \"$ORG_NAME\",
            \"customer_id\": \"$CUSTOMER_ID\"
        }")

    ORG_ID=$(echo "$RESPONSE" | jq -r '.id')

    if [ "$ORG_ID" != "null" ] && [ -n "$ORG_ID" ]; then
        print_success "Organization created with ID: $ORG_ID"
        echo "$RESPONSE" | jq '.'
        echo "$ORG_ID"
        return 0
    else
        print_error "Failed to create organization"
        echo "$RESPONSE" | jq '.'
        return 1
    fi
}

# Function to list organizations
list_organizations() {
    print_header "Listing Organizations"

    RESPONSE=$(curl -s "${BASE_URL}/api/v1/organizations/list")
    echo "$RESPONSE" | jq '.'
}

# Function to create a license agreement
create_agreement() {
    local ORG_ID="$1"
    local TIER="${2:-enterprise}"
    local MAX_SEATS="${3:-10}"
    local FEATURES="${4:-[\"audit\", \"grid\"]}"
    local START_DATE="${5:-$(date +%Y-%m-%d)}"  # Default to today
    local END_DATE="${6:-}"  # Optional

    print_header "Creating License Agreement"

    # Build JSON payload
    local JSON_PAYLOAD="{
        \"organization_id\": $ORG_ID,
        \"tier\": \"$TIER\",
        \"max_seats\": $MAX_SEATS,
        \"features\": $FEATURES,
        \"start_date\": \"$START_DATE\""

    # Add end_date if provided
    if [ -n "$END_DATE" ]; then
        JSON_PAYLOAD="${JSON_PAYLOAD},
        \"end_date\": \"$END_DATE\""
    fi

    JSON_PAYLOAD="${JSON_PAYLOAD},
        \"cost_per_month\": 1000.00
    }"

    RESPONSE=$(curl -s -X POST "${BASE_URL}/api/v1/agreements" \
        -H "Content-Type: application/json" \
        -d "$JSON_PAYLOAD")

    AGREEMENT_ID=$(echo "$RESPONSE" | jq -r '.id')

    if [ "$AGREEMENT_ID" != "null" ] && [ -n "$AGREEMENT_ID" ]; then
        print_success "Agreement created with ID: $AGREEMENT_ID"
        echo "$RESPONSE" | jq '.'
        echo "$AGREEMENT_ID"
        return 0
    else
        print_error "Failed to create agreement"
        echo "$RESPONSE" | jq '.'
        return 1
    fi
}

# Function to list agreements
list_agreements() {
    print_header "Listing Agreements"

    RESPONSE=$(curl -s "${BASE_URL}/api/v1/agreements/list")
    echo "$RESPONSE" | jq '.'
}

# Function to generate JWT token
generate_token() {
    local AGREEMENT_ID="$1"
    local USER_EMAIL="${2:-user@example.com}"
    local DURATION="${3:-31536000}"  # Default 1 year in seconds

    print_header "Generating JWT Token"

    RESPONSE=$(curl -s -X POST "${BASE_URL}/api/v1/tokens" \
        -H "Content-Type: application/json" \
        -d "{
            \"agreement_id\": $AGREEMENT_ID,
            \"user_email\": \"$USER_EMAIL\",
            \"duration_seconds\": $DURATION
        }")

    TOKEN=$(echo "$RESPONSE" | jq -r '.token')

    if [ "$TOKEN" != "null" ] && [ -n "$TOKEN" ]; then
        print_success "JWT token generated"
        echo "$RESPONSE" | jq '.'

        # Save token to file
        mkdir -p "$(dirname "$LICENSE_OUTPUT")"
        echo "$TOKEN" > "$LICENSE_OUTPUT"
        chmod 600 "$LICENSE_OUTPUT"
        print_success "Token saved to: $LICENSE_OUTPUT"

        return 0
    else
        print_error "Failed to generate token"
        echo "$RESPONSE" | jq '.'
        return 1
    fi
}

# Function to validate a token
validate_token() {
    local TOKEN="${1:-$(cat $LICENSE_OUTPUT 2>/dev/null)}"

    print_header "Validating JWT Token"

    if [ -z "$TOKEN" ]; then
        print_error "No token provided or found in $LICENSE_OUTPUT"
        return 1
    fi

    RESPONSE=$(curl -s -X POST "${BASE_URL}/api/v1/tokens/validate" \
        -H "Content-Type: application/json" \
        -d "{\"token\": \"$TOKEN\"}")

    echo "$RESPONSE" | jq '.'
}

# Function to build Janus with embedded public key
build_janus_with_key() {
    print_header "Building Janus with Embedded Public Key"

    # Get master public key
    PUBLIC_KEY=$(get_master_public_key)

    if [ -z "$PUBLIC_KEY" ]; then
        print_error "Failed to retrieve public key"
        return 1
    fi

    # Build Janus with embedded key
    print_info "Building Janus binary..."

    cd /home/dbreeden/Documents/development/github.com/pharmalytica/janus

    go build -ldflags "-X 'github.com/pharmalytica/janus/internal/license/publickey.EmbeddedPublicKey=${PUBLIC_KEY}'" \
        -o ./janus .

    if [ $? -eq 0 ]; then
        print_success "Janus built successfully with embedded public key"
        print_info "Binary location: ./janus"
        return 0
    else
        print_error "Failed to build Janus"
        return 1
    fi
}

# Complete workflow function
complete_workflow() {
    print_header "Complete License Workflow"

    # 1. Check server
    check_server || exit 1

    # 2. Generate master key
    generate_master_key || exit 1

    # 3. Create organization
    ORG_ID=$(create_organization "Development Team" "dev-team")
    [ -z "$ORG_ID" ] && exit 1

    # 4. Create agreement with full features
    AGREEMENT_ID=$(create_agreement "$ORG_ID" "enterprise" "50" '["audit", "grid", "validation"]')
    [ -z "$AGREEMENT_ID" ] && exit 1

    # 5. Generate token
    generate_token "$AGREEMENT_ID" "developer@example.com" || exit 1

    # 6. Validate token
    validate_token || exit 1

    # 7. Build Janus with embedded key
    build_janus_with_key || exit 1

    print_header "Workflow Complete!"
    print_success "You can now run: ./janus"
}

# Main menu
show_menu() {
    echo -e "\n${BLUE}License Server Workflow Menu${NC}"
    echo "1)  Check server health"
    echo "2)  Generate master signing key"
    echo "3)  List all signing keys"
    echo "4)  Get master public key"
    echo "5)  Create organization"
    echo "6)  List organizations"
    echo "7)  Create license agreement"
    echo "8)  List agreements"
    echo "9)  Generate JWT token"
    echo "10) Validate token"
    echo "11) Build Janus with embedded key"
    echo "12) Complete workflow (all steps)"
    echo "0)  Exit"
    echo
    read -p "Select option: " choice

    case $choice in
        1) check_server ;;
        2) generate_master_key ;;
        3) list_keys ;;
        4) get_master_public_key ;;
        5)
            read -p "Organization name: " org_name
            read -p "Customer ID (e.g., acme-pharma): " customer_id
            # Auto-generate customer_id if not provided
            if [ -z "$customer_id" ]; then
                customer_id=$(echo "$org_name" | tr '[:upper:]' '[:lower:]' | tr ' ' '-')
            fi
            create_organization "$org_name" "$customer_id"
            ;;
        6) list_organizations ;;
        7)
            read -p "Organization ID: " org_id
            read -p "Tier (enterprise/professional/starter): " tier
            read -p "Max seats: " max_seats
            read -p "Features (JSON array, e.g., [\"audit\",\"grid\"]): " features
            read -p "Start date (YYYY-MM-DD, default=today): " start_date
            read -p "End date (YYYY-MM-DD, optional): " end_date
            # Use today if start_date is empty
            if [ -z "$start_date" ]; then
                start_date=$(date +%Y-%m-%d)
            fi
            create_agreement "$org_id" "$tier" "$max_seats" "$features" "$start_date" "$end_date"
            ;;
        8) list_agreements ;;
        9)
            read -p "Agreement ID: " agr_id
            read -p "User email: " user_email
            generate_token "$agr_id" "$user_email"
            ;;
        10) validate_token ;;
        11) build_janus_with_key ;;
        12) complete_workflow ;;
        0) exit 0 ;;
        *) print_error "Invalid option" ;;
    esac
}

# Parse command line arguments
if [ $# -eq 0 ]; then
    # Interactive mode
    while true; do
        show_menu
    done
else
    # Command mode
    case "$1" in
        health) check_server ;;
        key) generate_master_key ;;
        keys) list_keys ;;
        pubkey) get_master_public_key ;;
        orgs) list_organizations ;;
        agreements) list_agreements ;;
        workflow) complete_workflow ;;
        build) build_janus_with_key ;;
        *)
            echo "Usage: $0 [health|key|keys|pubkey|orgs|agreements|workflow|build]"
            echo "  Or run without arguments for interactive menu"
            exit 1
            ;;
    esac
fi
