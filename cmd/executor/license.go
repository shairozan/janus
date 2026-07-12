package main

import (
	"embed"
	"fmt"
	"os"
	"strings"

	"github.com/pharmalytica/janus/internal/license/validator"
)

//go:embed *
var assets embed.FS

// verifyLicense validates the license JWT file.
// Uses the same embedded public key mechanism as Janus main binary.
// If no public key is embedded (dev mode), validation is skipped with a warning.
func verifyLicense(licensePath string) error {
	// Read the license JWT file
	licenseBytes, err := os.ReadFile(licensePath)
	if err != nil {
		return fmt.Errorf("license file not found or not readable: %s\n  Hint: Use --executor-license to specify license file path", licensePath)
	}

	licenseJWT := strings.TrimSpace(string(licenseBytes))
	if licenseJWT == "" {
		return fmt.Errorf("license file is empty: %s", licensePath)
	}

	// Create JWT validator with embedded public key
	v, err := validator.NewValidator(assets)
	if err != nil {
		// If no public key is embedded (dev mode), skip validation
		if strings.Contains(err.Error(), "no public key embedded") || strings.Contains(err.Error(), "file does not exist") {
			fmt.Fprintln(os.Stderr, "⚠️  Warning: No embedded public key - license validation skipped (dev mode)")

			return nil
		}

		return fmt.Errorf("failed to initialize license validator: %w", err)
	}

	// Validate the JWT token
	claims, err := v.ValidateToken(licenseJWT)
	if err != nil {
		return fmt.Errorf("license validation failed: %w\n  Your license may be expired, invalid, or tampered with", err)
	}

	// Check if expired
	if claims.IsExpired() {
		return fmt.Errorf("license has expired\n  Expiration date: %s\n  Contact your administrator for renewal", claims.ExpiresAt.Format("2006-01-02"))
	}

	// Optional: Print license info for debugging
	if os.Getenv("EXECUTOR_DEBUG_LICENSE") == "1" {
		fmt.Fprintf(os.Stderr, "✅ License validated successfully:\n")
		fmt.Fprintf(os.Stderr, "  Organization: %s (ID: %d)\n", claims.Tier, claims.OrganizationID)
		fmt.Fprintf(os.Stderr, "  Agreement ID: %d\n", claims.AgreementID)
		fmt.Fprintf(os.Stderr, "  Features: %v\n", claims.Features)
		fmt.Fprintf(os.Stderr, "  Max Seats: %d\n", claims.MaxSeats)
		fmt.Fprintf(os.Stderr, "  Expires: %s (in %s)\n", claims.ExpiresAt.Format("2006-01-02"), claims.TimeUntilExpiry())
	}

	return nil
}
