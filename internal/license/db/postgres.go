package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // PostgreSQL driver
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/pharmalytica/janus/internal/license/models"
)

// DB wraps gorm.DB for database operations.
type DB struct {
	DB *gorm.DB // For model-based operations (primary interface)
}

// GetDB returns the underlying GORM database instance.
func (db *DB) GetDB() *gorm.DB {
	return db.DB
}

// GetSQLDB returns the underlying *sql.DB for migrations.
func (db *DB) GetSQLDB() (*sql.DB, error) {
	return db.DB.DB()
}

// Close closes the database connection.
func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}

	return sqlDB.Close()
}

// Connect opens a connection to PostgreSQL and applies migrations.
func Connect(databaseURL string, autoMigrate bool) (*DB, error) {
	// Connect with sqlx for raw SQL
	sqlxDB, err := sqlx.Connect("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	sqlxDB.SetMaxOpenConns(25)
	sqlxDB.SetMaxIdleConns(5)
	sqlxDB.SetConnMaxLifetime(5 * time.Minute)

	// Connect with GORM using the same underlying connection
	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlxDB.DB,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // Silent by default, can be configured later
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GORM: %w", err)
	}

	// Auto-apply migrations if requested (using goose, not GORM AutoMigrate)
	if autoMigrate {
		if err := ApplyMigrations(sqlxDB.DB); err != nil {
			return nil, fmt.Errorf("failed to apply migrations: %w", err)
		}
	}

	return &DB{
		DB: gormDB,
	}, nil
}

// Legacy methods for backward compatibility - will be removed after full refactor

// GetActiveSigningKey retrieves the active signing key for an organization (or master key if orgID is nil).
func (db *DB) GetActiveSigningKey(orgID *int64) (*models.SigningKey, error) {
	var key models.SigningKey
	query := db.DB.Where("revoked_at IS NULL AND expires_at > ?", time.Now())

	if orgID != nil {
		query = query.Where("organization_id = ?", *orgID)
	} else {
		query = query.Where("organization_id IS NULL")
	}

	if err := query.Order("created_at DESC").First(&key).Error; err != nil {
		return nil, err
	}

	return &key, nil
}

// CreateSigningKey creates a new signing key.
func (db *DB) CreateSigningKey(key *models.SigningKey) error {
	return db.DB.Create(key).Error
}

// GetSigningKeyByID retrieves a signing key by its key_id.
func (db *DB) GetSigningKeyByID(keyID string) (*models.SigningKey, error) {
	var key models.SigningKey
	if err := db.DB.Where("key_id = ?", keyID).First(&key).Error; err != nil {
		return nil, err
	}

	return &key, nil
}

// RevokeSigningKey revokes a signing key by setting revoked_at.
func (db *DB) RevokeSigningKey(keyID string) error {
	now := time.Now()

	return db.DB.Model(&models.SigningKey{}).Where("key_id = ?", keyID).Update("revoked_at", now).Error
}

// RotateSigningKey creates a new key and revokes the old one in a transaction.
func (db *DB) RotateSigningKey(newKey *models.SigningKey, oldKeyID string) error {
	return db.DB.Transaction(func(tx *gorm.DB) error {
		// Create new key
		if err := tx.Create(newKey).Error; err != nil {
			return err
		}

		// Revoke old key
		now := time.Now()
		if err := tx.Model(&models.SigningKey{}).Where("key_id = ?", oldKeyID).Update("revoked_at", now).Error; err != nil {
			return err
		}

		return nil
	})
}
