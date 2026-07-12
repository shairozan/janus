//go:build integration && server
// +build integration,server

package testutil

import (
	"testing"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/pharmalytica/janus/internal/license/models"
)

// Querier abstracts sqlx.DB and sqlx.Tx for test fixtures
// This allows fixtures to work with both direct DB connections and transactions
// Both *sqlx.DB and *sqlx.Tx implement this interface
type Querier interface {
	sqlx.Queryer
	sqlx.Execer
	Get(dest interface{}, query string, args ...interface{}) error
	Select(dest interface{}, query string, args ...interface{}) error
}

// SeedOrganization creates a test organization and returns it
func SeedOrganization(t *testing.T, q Querier, name string) *models.Organization {
	t.Helper()

	org := &models.Organization{
		Name:       name,
		CustomerID: "CUST-" + name,
		CreatedAt:  time.Now(),
	}

	query := `INSERT INTO organizations (name, customer_id, created_at)
	          VALUES ($1, $2, $3) RETURNING id`
	err := q.Get(&org.ID, query, org.Name, org.CustomerID, org.CreatedAt)
	if err != nil {
		t.Fatalf("Failed to seed organization: %v", err)
	}

	return org
}

// SeedLicense creates a test license and returns it
func SeedLicense(t *testing.T, q Querier, name, tier string, features []string, msrpCents int) *models.License {
	t.Helper()

	license := &models.License{
		Name:      name,
		Tier:      tier,
		Features:  models.Features(features),
		MSRPCents: msrpCents,
		CreatedAt: time.Now(),
	}

	query := `INSERT INTO licenses (name, tier, features, msrp_cents, created_at)
	          VALUES ($1, $2, $3, $4, $5) RETURNING id`
	err := q.Get(&license.ID, query, license.Name, license.Tier, license.Features, license.MSRPCents, license.CreatedAt)
	if err != nil {
		t.Fatalf("Failed to seed license: %v", err)
	}

	return license
}

// SeedAgreement creates a test agreement and returns it
func SeedAgreement(t *testing.T, q Querier, orgID, licenseID int64, seats, pricePerSeatCents int) *models.Agreement {
	t.Helper()

	agr := &models.Agreement{
		OrganizationID:    orgID,
		LicenseID:         licenseID,
		Seats:             seats,
		PricePerSeatCents: pricePerSeatCents,
		LicenseModel:      "perpetual",
		ActivatedAt:       time.Now(),
	}

	query := `INSERT INTO agreements (organization_id, license_id, seats, price_per_seat_cents, license_model, activated_at)
	          VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`
	err := q.Get(&agr.ID, query, agr.OrganizationID, agr.LicenseID, agr.Seats, agr.PricePerSeatCents, agr.LicenseModel, agr.ActivatedAt)
	if err != nil {
		t.Fatalf("Failed to seed agreement: %v", err)
	}

	return agr
}

// SeedSigningKey creates a test signing key and returns it
func SeedSigningKey(t *testing.T, q Querier, orgID int64, keyID, publicKeyPEM, encryptedPrivateKey string) *models.SigningKey {
	t.Helper()

	key := &models.SigningKey{
		OrganizationID:      &orgID,
		KeyID:               keyID,
		PublicKeyPEM:        publicKeyPEM,
		PrivateKeyEncrypted: &encryptedPrivateKey,
		CreatedAt:           time.Now(),
		ExpiresAt:           time.Now().AddDate(1, 0, 0), // 1 year from now
	}

	query := `INSERT INTO signing_keys (organization_id, key_id, public_key_pem, private_key_encrypted, created_at, expires_at)
	          VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`
	err := q.Get(&key.ID, query, key.OrganizationID, key.KeyID, key.PublicKeyPEM, key.PrivateKeyEncrypted, key.CreatedAt, key.ExpiresAt)
	if err != nil {
		t.Fatalf("Failed to seed signing key: %v", err)
	}

	return key
}

// StandardFixtures creates a standard set of test data
// Returns: org, license, agreement
func StandardFixtures(t *testing.T, q Querier) (*models.Organization, *models.License, *models.Agreement) {
	t.Helper()

	org := SeedOrganization(t, q, "TestOrg")
	license := SeedLicense(t, q, "Pro", "professional", []string{"runlog", "grid"}, 100000)
	agr := SeedAgreement(t, q, org.ID, license.ID, 10, 10000)

	return org, license, agr
}
