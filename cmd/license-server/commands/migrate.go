package commands

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"github.com/pharmalytica/janus/internal/license/db"
)

// MigrateCommand creates the migrate command.
func MigrateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Database migration commands",
		Long:  `Manage database migrations (up, down, status).`,
	}

	cmd.AddCommand(migrateUpCommand())
	cmd.AddCommand(migrateDownCommand())
	cmd.AddCommand(migrateStatusCommand())

	return cmd
}

func migrateUpCommand() *cobra.Command {
	var databaseURL string

	cmd := &cobra.Command{
		Use:   "up",
		Short: "Apply all pending migrations",
		RunE: func(c *cobra.Command, args []string) error {
			database, err := db.Connect(databaseURL, false)
			if err != nil {
				return fmt.Errorf("failed to connect to database: %w", err)
			}
			defer database.Close()

			sqlDB, err := database.GetSQLDB()
			if err != nil {
				return fmt.Errorf("failed to get SQL DB: %w", err)
			}

			if err := db.ApplyMigrations(sqlDB); err != nil {
				return fmt.Errorf("failed to apply migrations: %w", err)
			}

			version, _ := db.GetMigrationStatus(sqlDB)
			log.Printf("Migrations applied (version: %d)", version)

			return nil
		},
	}

	cmd.Flags().StringVar(&databaseURL, "database-url", "",
		"PostgreSQL connection string")
	_ = cmd.MarkFlagRequired("database-url")

	return cmd
}

func migrateDownCommand() *cobra.Command {
	var databaseURL string

	cmd := &cobra.Command{
		Use:   "down",
		Short: "Rollback the last migration",
		RunE: func(c *cobra.Command, args []string) error {
			database, err := db.Connect(databaseURL, false)
			if err != nil {
				return fmt.Errorf("failed to connect to database: %w", err)
			}
			defer database.Close()

			sqlDB, err := database.GetSQLDB()
			if err != nil {
				return fmt.Errorf("failed to get SQL DB: %w", err)
			}

			if err := db.MigrateDown(sqlDB); err != nil {
				return fmt.Errorf("failed to rollback migration: %w", err)
			}

			version, _ := db.GetMigrationStatus(sqlDB)
			log.Printf("Migration rolled back (version: %d)", version)

			return nil
		},
	}

	cmd.Flags().StringVar(&databaseURL, "database-url", "",
		"PostgreSQL connection string")
	_ = cmd.MarkFlagRequired("database-url")

	return cmd
}

func migrateStatusCommand() *cobra.Command {
	var databaseURL string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show migration status",
		RunE: func(c *cobra.Command, args []string) error {
			database, err := db.Connect(databaseURL, false)
			if err != nil {
				return fmt.Errorf("failed to connect to database: %w", err)
			}
			defer database.Close()

			sqlDB, err := database.GetSQLDB()
			if err != nil {
				return fmt.Errorf("failed to get SQL DB: %w", err)
			}

			return db.MigrateStatus(sqlDB)
		},
	}

	cmd.Flags().StringVar(&databaseURL, "database-url", "",
		"PostgreSQL connection string")
	_ = cmd.MarkFlagRequired("database-url")

	return cmd
}
