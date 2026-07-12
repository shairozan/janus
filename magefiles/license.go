//go:build mage
// +build mage

package main

import (
	"fmt"
	"os"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// License namespace for license-server operations
type License mg.Namespace

const (
	licenseServerDir    = "./cmd/license-server"
	licenseServerBinary = "license-server"
	licenseVersionPkg   = "github.com/pharmalytica/janus/internal/license/version"
)

// Build builds the license-server binary (standalone, no dependencies)
func (License) Build() error {
	fmt.Println("🔨 Building license-server...")

	// Get git commit
	commit, _ := sh.Output("git", "rev-parse", "HEAD")
	if commit == "" {
		commit = "unknown"
	}

	// Get current date
	date, _ := sh.Output("date", "-u", "+%Y-%m-%dT%H:%M:%SZ")
	if date == "" {
		date = "unknown"
	}

	ldflags := fmt.Sprintf("-s -w -X %s.Version=dev -X %s.Commit=%s -X %s.Date=%s -X %s.BuiltBy=mage",
		licenseVersionPkg, licenseVersionPkg, commit, licenseVersionPkg, date, licenseVersionPkg)

	env := map[string]string{
		"CGO_ENABLED": "1",
	}

	return sh.RunWithV(env, goexe, "build", "-buildvcs=false", "-ldflags", ldflags, "-o", licenseServerBinary, licenseServerDir)
}

// Release builds a release version with proper versioning (combined workflow)
func (License) Release(version string) error {
	if version == "" {
		return fmt.Errorf("version is required (e.g., v1.0.0)")
	}

	fmt.Printf("🚀 Building license-server release version %s...\n", version)

	// Get git commit
	commit, _ := sh.Output("git", "rev-parse", "HEAD")
	if commit == "" {
		commit = "unknown"
	}

	// Get current date
	date, _ := sh.Output("date", "-u", "+%Y-%m-%dT%H:%M:%SZ")
	if date == "" {
		date = "unknown"
	}

	ldflags := fmt.Sprintf("-s -w -X %s.Version=%s -X %s.Commit=%s -X %s.Date=%s -X %s.BuiltBy=mage",
		licenseVersionPkg, version, licenseVersionPkg, commit, licenseVersionPkg, date, licenseVersionPkg)

	env := map[string]string{
		"CGO_ENABLED": "1",
	}

	return sh.RunWithV(env, goexe, "build", "-buildvcs=false", "-ldflags", ldflags, "-o", licenseServerBinary, licenseServerDir)
}

// Test runs all license-server tests
func (License) Test() error {
	fmt.Println("🧪 Running license-server tests...")
	return sh.RunV(goexe, "test", "-v", "-race", "-coverprofile=coverage-license.out", "./internal/license/...", "./cmd/license-server/...")
}

// Unit runs license-server unit tests (no external dependencies)
func (License) Unit() error {
	fmt.Println("🧪 Running license-server unit tests...")
	return sh.RunV(goexe, "test", "-v", "-race", "-tags=unit", "./internal/license/...", "./cmd/license-server/...")
}

// Integration runs license-server integration tests (requires PostgreSQL)
func (License) Integration() error {
	fmt.Println("🧪 Running license-server integration tests (with PostgreSQL)...")
	fmt.Println("⚠️  Requires PostgreSQL running with migrations applied")
	fmt.Println("   Default: postgres://janus:janus_dev_password@localhost:5432/janus_license?sslmode=disable")
	fmt.Println("   Override with LICENSING_DATABASE_URL and LICENSING_ENCRYPTION_KEY")

	// Set default environment variables if not already set
	env := map[string]string{
		"LICENSING_DATABASE_URL":   os.Getenv("LICENSING_DATABASE_URL"),
		"LICENSING_ENCRYPTION_KEY": os.Getenv("LICENSING_ENCRYPTION_KEY"),
	}

	if env["LICENSING_DATABASE_URL"] == "" {
		env["LICENSING_DATABASE_URL"] = "postgres://janus:janus_dev_password@localhost:5432/janus_license?sslmode=disable"
	}
	if env["LICENSING_ENCRYPTION_KEY"] == "" {
		env["LICENSING_ENCRYPTION_KEY"] = "12345678901234567890123456789012" // 32-byte test key
	}

	if err := sh.RunWithV(env, goexe, "test", "-v", "-timeout", "60s", "-tags=integration,server", "./internal/license/...", "./cmd/license-server/..."); err != nil {
		return fmt.Errorf("integration tests failed: %w", err)
	}

	fmt.Println("✅ Integration tests completed!")
	return nil
}

// Check runs format, lint, and unit tests for license-server
func (License) Check() error {
	mg.Deps(Format, Lint)
	fmt.Println("🧪 Running license-server unit tests...")
	if err := sh.RunV(goexe, "test", "-v", "-race", "-tags=unit", "./internal/license/...", "./cmd/license-server/..."); err != nil {
		return err
	}
	fmt.Println("✅ License-server checks passed!")
	return nil
}

// CheckAll runs format, lint, unit tests, and build for license-server
func (License) CheckAll() error {
	mg.Deps(Format, Lint)
	fmt.Println("🧪 Running license-server unit tests...")
	if err := sh.RunV(goexe, "test", "-v", "-race", "-tags=unit", "./internal/license/...", "./cmd/license-server/..."); err != nil {
		return err
	}

	l := License{}
	if err := l.Build(); err != nil {
		return err
	}

	fmt.Println("✅ License-server complete validation passed!")
	return nil
}

// Clean removes license-server build artifacts
func (License) Clean() error {
	fmt.Println("🧹 Cleaning license-server artifacts...")

	filesToRemove := []string{
		licenseServerBinary,
		"license-server.exe",
		"license-server-ubuntu2004",
		"license-server-ubuntu2204",
		"license-server-ubuntu2404",
		"coverage-license.out",
		"coverage-license.html",
	}

	for _, file := range filesToRemove {
		if err := sh.Rm(file); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove %s: %w", file, err)
		}
	}

	fmt.Println("✅ License-server cleanup complete!")
	return nil
}

// Serve runs the license-server with environment variables from .env
func (License) Serve() error {
	fmt.Println("🚀 Starting license-server...")
	fmt.Println("📋 Loading environment from .env file...")

	env := map[string]string{
		"CGO_ENABLED": "1",
	}

	// Load .env file if it exists
	if _, err := os.Stat(".env"); err == nil {
		// Read .env and set environment variables
		if err := sh.RunV("bash", "-c", "source .env && ./"+licenseServerBinary+" serve"); err != nil {
			return err
		}
		return nil
	}

	fmt.Println("⚠️  .env file not found, using default configuration")
	return sh.RunWithV(env, "./"+licenseServerBinary, "serve")
}

// Migrate applies database migrations for license-server
func (License) Migrate() error {
	fmt.Println("🗄️  Running license-server migrations...")
	fmt.Println("📋 Loading environment from .env file...")

	// Migrations run automatically on server startup, but can be invoked separately
	return sh.RunV("bash", "-c", "source .env && ./"+licenseServerBinary+" migrate")
}

// Seed populates the database with seed data idempotently
func (License) Seed() error {
	fmt.Println("🌱 Seeding license-server database...")

	// Check if .env exists
	if _, err := os.Stat(".env"); os.IsNotExist(err) {
		return fmt.Errorf(".env file not found - please create it from .env.example")
	}

	// Check if seed_data.json exists
	seedFile := "./cmd/license-server/seed_data.json"
	if _, err := os.Stat(seedFile); os.IsNotExist(err) {
		return fmt.Errorf("seed data file not found: %s", seedFile)
	}

	// Run seeding script with curl commands
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
    echo "❌ Server failed to start within 30 seconds"
    exit 1
  fi
  sleep 1
done

# Parse JSON and create organizations
echo "🏢 Creating organizations..."
cat ./cmd/license-server/seed_data.json | jq -r '.organizations[] | @json' | while read -r org; do
  name=$(echo "$org" | jq -r '.name')
  echo "  - Creating organization: $name"

  # Check if organization already exists
  existing=$(curl -s "$BASE_URL/api/v1/organizations/list" | jq -r --arg name "$name" '.[] | select(.name == $name) | .id')

  if [ -n "$existing" ]; then
    echo "    ✓ Organization already exists (ID: $existing)"
  else
    response=$(curl -s -X POST "$BASE_URL/api/v1/organizations" \
      -H "Content-Type: application/json" \
      -d "$org")
    org_id=$(echo "$response" | jq -r '.id')
    if [ "$org_id" != "null" ] && [ -n "$org_id" ]; then
      echo "    ✓ Created organization (ID: $org_id)"
    else
      echo "    ✗ Failed to create organization: $response"
    fi
  fi
done

# Parse JSON and create agreements
echo "📄 Creating agreements..."
cat ./cmd/license-server/seed_data.json | jq -c '.agreements[]' | while read -r agr; do
  org_name=$(echo "$agr" | jq -r '.organization_name')
  tier=$(echo "$agr" | jq -r '.tier')

  echo "  - Creating agreement for $org_name ($tier tier)"

  # Get organization ID
  org_id=$(curl -s "$BASE_URL/api/v1/organizations/list" | jq -r --arg name "$org_name" '.[] | select(.name == $name) | .id')

  if [ -z "$org_id" ] || [ "$org_id" == "null" ]; then
    echo "    ✗ Organization not found: $org_name"
    continue
  fi

  # Check if agreement already exists
  existing=$(curl -s "$BASE_URL/api/v1/agreements/list?organization_id=$org_id" | jq -r --arg tier "$tier" '.[] | select(.tier == $tier and .active == true) | .id')

  if [ -n "$existing" ]; then
    echo "    ✓ Agreement already exists (ID: $existing)"
  else
    # Build agreement request
    agreement_data=$(echo "$agr" | jq --argjson org_id "$org_id" '. + {organization_id: $org_id} | del(.organization_name)')

    response=$(curl -s -X POST "$BASE_URL/api/v1/agreements" \
      -H "Content-Type: application/json" \
      -d "$agreement_data")
    agr_id=$(echo "$response" | jq -r '.id')
    if [ "$agr_id" != "null" ] && [ -n "$agr_id" ]; then
      echo "    ✓ Created agreement (ID: $agr_id)"
    else
      echo "    ✗ Failed to create agreement: $response"
    fi
  fi
done

echo "✅ Seeding complete!"
`

	return sh.RunV("bash", "-c", script)
}
