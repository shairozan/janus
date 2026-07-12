//go:build integration && server
// +build integration,server

package testutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/pharmalytica/janus/internal/license/db"
)

// PostgresContainer wraps the testcontainers postgres container
type PostgresContainer struct {
	Container *postgres.PostgresContainer
	ConnStr   string
}

// SetupPostgresContainer creates a new PostgreSQL container for testing
func SetupPostgresContainer(t *testing.T) *PostgresContainer {
	t.Helper()

	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("Failed to start PostgreSQL container: %v", err)
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("Failed to get connection string: %v", err)
	}

	return &PostgresContainer{
		Container: pgContainer,
		ConnStr:   connStr,
	}
}

// Close terminates the container
func (pc *PostgresContainer) Close(t *testing.T) {
	t.Helper()

	ctx := context.Background()
	if err := pc.Container.Terminate(ctx); err != nil {
		t.Errorf("Failed to terminate container: %v", err)
	}
}

// SetupTestDatabase creates a container, connects, and runs migrations
func SetupTestDatabase(t *testing.T) (*db.DB, func()) {
	t.Helper()

	// Start container
	container := SetupPostgresContainer(t)

	// Connect and run migrations
	database, err := db.Connect(container.ConnStr, true)
	if err != nil {
		container.Close(t)
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// Return database and cleanup function
	cleanup := func() {
		database.Close()
		container.Close(t)
	}

	return database, cleanup
}

// WithTestDatabase is a helper that sets up a containerized database, runs migrations,
// executes the test function, and cleans up afterwards
func WithTestDatabase(t *testing.T, testFn func(t *testing.T, db *db.DB)) {
	t.Helper()

	database, cleanup := SetupTestDatabase(t)
	defer cleanup()

	testFn(t, database)
}

// TruncateAllTablesGORM removes all data from tables using GORM
func TruncateAllTablesGORM(t *testing.T, database *db.DB) {
	t.Helper()

	tables := []string{
		"issued_tokens",
		"signing_keys",
		"agreements",
		"licenses",
		"organizations",
	}

	for _, table := range tables {
		sql := fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", table)
		if err := database.DB.Exec(sql).Error; err != nil {
			t.Fatalf("Failed to truncate table %s: %v", table, err)
		}
	}
}
