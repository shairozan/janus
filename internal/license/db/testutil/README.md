# License-Server Test Utilities

Utilities for integration testing with database state management.

## Overview

This package provides helpers for license-server integration tests that need to interact with the PostgreSQL database. The key feature is **state reset** - the ability to truncate all tables between tests to ensure a clean, known state.

## Prerequisites

Integration tests require:
- PostgreSQL database running
- `LICENSING_DATABASE_URL` environment variable set
- `LICENSING_ENCRYPTION_KEY` environment variable set (for key tests)
- Tests tagged with `//go:build integration`

## Usage Patterns

### Pattern 1: Test with Database Setup/Cleanup

```go
//go:build integration
// +build integration

package mypackage_test

import (
    "testing"
    "github.com/pharmalytica/janus/internal/license/db/testutil"
)

func TestMyFeature(t *testing.T) {
    db := testutil.SetupTestDB(t)
    defer testutil.CleanupTestDB(t, db)

    // Your test code here
}
```

### Pattern 2: Reset State Between Sub-Tests

```go
func TestMultipleScenarios(t *testing.T) {
    db := testutil.SetupTestDB(t)
    defer testutil.CleanupTestDB(t, db)

    t.Run("scenario 1", func(t *testing.T) {
        testutil.TruncateAllTables(t, db)  // Start with clean state

        // Test scenario 1
        org := testutil.SeedOrganization(t, db, "Org1")
        // ... assertions ...
    })

    t.Run("scenario 2", func(t *testing.T) {
        testutil.TruncateAllTables(t, db)  // Clean state again

        // Test scenario 2 - database is empty
        org := testutil.SeedOrganization(t, db, "Org2")
        // ... assertions ...
    })
}
```

### Pattern 3: Test with Standard Fixtures

```go
func TestWithStandardData(t *testing.T) {
    db := testutil.SetupTestDB(t)
    defer testutil.CleanupTestDB(t, db)

    testutil.TruncateAllTables(t, db)

    // Seed standard test data
    org, license, agr := testutil.StandardFixtures(t, db)

    // Now test with known data
    assert.Equal(t, org.ID, agr.OrganizationID)
}
```

### Pattern 4: Test with Transaction Rollback

```go
func TestRollback(t *testing.T) {
    db := testutil.SetupTestDB(t)
    defer testutil.CleanupTestDB(t, db)

    testutil.TruncateAllTables(t, db)

    // Start transaction
    tx := testutil.BeginTestTx(t, db)
    defer tx.Rollback(t)

    // Make changes in transaction
    _, err := tx.Exec("INSERT INTO organizations ...")
    require.NoError(t, err)

    // Rollback
    tx.Rollback(t)

    // Changes are gone
    var count int
    db.Get(&count, "SELECT COUNT(*) FROM organizations")
    assert.Equal(t, 0, count)
}
```

### Pattern 5: WithCleanDB Helper

```go
func TestWithHelpers(t *testing.T) {
    db := testutil.SetupTestDB(t)
    defer testutil.CleanupTestDB(t, db)

    // Automatically truncates before running test
    testutil.WithCleanDB(t, db, func() {
        org := testutil.SeedOrganization(t, db, "TestOrg")
        // ... test code ...
    })

    // Another clean test
    testutil.WithCleanDB(t, db, func() {
        // Database is empty again
    })
}
```

## Fixture Functions

### SeedOrganization
```go
org := testutil.SeedOrganization(t, db, "OrgName")
// Returns: *models.Organization with ID populated
```

### SeedLicense
```go
license := testutil.SeedLicense(t, db, "Pro", "professional",
    []string{"audit", "grid"}, 100000)
// Returns: *models.License with ID populated
```

### SeedAgreement
```go
agr := testutil.SeedAgreement(t, db, orgID, licenseID,
    10, 10000) // 10 seats, $100 per seat
// Returns: *models.Agreement with ID populated
```

### SeedSigningKey
```go
key := testutil.SeedSigningKey(t, db, orgID, "key-001",
    publicKeyPEM, encryptedPrivateKey)
// Returns: *models.SigningKey with ID populated
```

### StandardFixtures
```go
org, license, agr := testutil.StandardFixtures(t, db)
// Creates a standard set of related test data
```

## State Reset Details

### TruncateAllTables
Removes all data while preserving schema and relationships:
- Faster than DROP/CREATE
- Maintains foreign key constraints
- Resets auto-increment sequences
- Order: `issued_tokens`, `signing_keys`, `agreements`, `licenses`, `organizations`

### When to Use State Reset
- ✅ Between unrelated test scenarios
- ✅ When you need guaranteed empty database
- ✅ To prevent test pollution
- ❌ Within a single logical test (use subtests instead)

## Running Integration Tests

### Local Development
```bash
# Start PostgreSQL (via docker-compose or local)
docker-compose up -d postgres

# Set environment variables
export LICENSING_DATABASE_URL="postgres://janus:password@localhost:5432/janus_license_test?sslmode=disable"
export LICENSING_ENCRYPTION_KEY="your-32-byte-encryption-key-here"

# Run integration tests
go test -v -tags=integration ./internal/license/...
```

### CI/CD
Integration tests run automatically in GitHub Actions with PostgreSQL service container.

## Best Practices

1. **Always use build tags**: `//go:build integration` at the top of test files
2. **Clean state for independence**: Call `TruncateAllTables` between unrelated tests
3. **Use fixtures for common data**: Don't manually insert the same data repeatedly
4. **Test transactions when needed**: Use `BeginTestTx` for rollback scenarios
5. **Defer cleanup**: Always `defer testutil.CleanupTestDB(t, db)`

## Examples

See:
- [internal/license/db/integration_test.go](../integration_test.go) - Database state management examples
- [internal/license/keys/integration_test.go](../../keys/integration_test.go) - Key generation and storage tests
