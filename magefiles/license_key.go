//go:build mage
// +build mage

package main

import (
	"fmt"
	"os"

	"github.com/magefile/mage/sh"
)

// ExportPublicKey exports the master public key from the license-server to .license_public_key.pem
// This key will be automatically embedded into Janus during the next build.
// The license-server must be running for this to work.
func (License) ExportPublicKey() error {
	fmt.Println("🔑 Exporting license server master public key...")

	// Check if .env exists
	if _, err := os.Stat(".env"); os.IsNotExist(err) {
		return fmt.Errorf(".env file not found - please create it from .env.example")
	}

	script := `
set -e
source .env

# Base URL
BASE_URL="http://${LICENSING_HOST:-localhost}:${LICENSING_PORT:-8443}"

echo "📡 Connecting to license-server at $BASE_URL..."

# Wait for server to be ready
echo "⏳ Waiting for server to be ready..."
for i in {1..30}; do
  if curl -s "$BASE_URL/health" > /dev/null 2>&1; then
    echo "✅ Server is ready!"
    break
  fi
  if [ $i -eq 30 ]; then
    echo "❌ Server failed to respond within 30 seconds"
    echo "   Make sure the license-server is running:"
    echo "   mage license:serve"
    exit 1
  fi
  sleep 1
done

# Get list of all keys
echo "🔍 Fetching master key..."
keys_response=$(curl -s "$BASE_URL/api/v1/keys")

# Extract the first master key (organization_id = null)
master_key_id=$(echo "$keys_response" | jq -r '.[] | select(.organization_id == null) | .key_id' | head -1)

if [ -z "$master_key_id" ] || [ "$master_key_id" == "null" ]; then
  echo "❌ No master key found!"
  echo "   You need to generate a master key first:"
  echo "   curl -X POST $BASE_URL/api/v1/keys/rotate -H 'Content-Type: application/json' -d '{\"expires_in_days\": 365}'"
  exit 1
fi

echo "📥 Found master key: $master_key_id"

# Get the public key
public_key=$(curl -s "$BASE_URL/api/v1/keys/public?key_id=$master_key_id" | jq -r '.public_key_pem')

if [ -z "$public_key" ] || [ "$public_key" == "null" ]; then
  echo "❌ Failed to retrieve public key"
  exit 1
fi

# Save to file
echo "$public_key" > .license_public_key.pem

echo "✅ Public key exported to .license_public_key.pem"
echo "   Key ID: $master_key_id"
echo ""
echo "🔨 Next steps:"
echo "   1. Build Janus: mage build"
echo "      (The public key will be automatically embedded)"
echo "   2. The .license_public_key.pem file is git-ignored for security"
`

	return sh.RunV("bash", "-c", script)
}

// GenerateMasterKey generates a new master key on the license-server
// This is a convenience function for initial setup.
func (License) GenerateMasterKey() error {
	fmt.Println("🔑 Generating new master signing key...")

	// Check if .env exists
	if _, err := os.Stat(".env"); os.IsNotExist(err) {
		return fmt.Errorf(".env file not found - please create it from .env.example")
	}

	script := `
set -e
source .env

# Base URL
BASE_URL="http://${LICENSING_HOST:-localhost}:${LICENSING_PORT:-8443}"

echo "📡 Connecting to license-server at $BASE_URL..."

# Wait for server to be ready
for i in {1..30}; do
  if curl -s "$BASE_URL/health" > /dev/null 2>&1; then
    break
  fi
  if [ $i -eq 30 ]; then
    echo "❌ Server failed to respond - make sure it's running: mage license:serve"
    exit 1
  fi
  sleep 1
done

echo "🔐 Generating master key (valid for 365 days)..."

response=$(curl -s -X POST "$BASE_URL/api/v1/keys/rotate" \
  -H "Content-Type: application/json" \
  -d '{"expires_in_days": 365}')

new_key_id=$(echo "$response" | jq -r '.new_key_id')

if [ -z "$new_key_id" ] || [ "$new_key_id" == "null" ]; then
  echo "❌ Failed to generate key:"
  echo "$response"
  exit 1
fi

echo "✅ Master key generated successfully!"
echo "   Key ID: $new_key_id"
echo ""
echo "🔨 Next step: Export the public key"
echo "   mage license:exportPublicKey"
`

	return sh.RunV("bash", "-c", script)
}
