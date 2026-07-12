package signup

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"strings"
)

const (
	customerIDPrefix   = "cust-"
	customerIDSlugMax  = 40 // cap the slug so the full id stays well under the 100-char column
	customerIDRandLen  = 5  // random bytes → 8 unpadded base32 chars
	customerIDFallback = "org"
)

// GenerateCustomerID builds a collision-safe, human-legible immutable customer id
// of the form "cust-<slug>-<suffix>", e.g. "cust-acme-labs-jbswy3dp". The slug is
// the lowercased org name with non-alphanumeric runs collapsed to "-"; the suffix
// is crypto-random base32 (lowercase, unpadded). The result stays well under the
// 100-char customer_id column limit. Uniqueness is ultimately enforced by the DB
// unique index — the random suffix makes a collision negligible.
func GenerateCustomerID(orgName string) (string, error) {
	suffix, err := randomSuffix(customerIDRandLen)
	if err != nil {
		return "", err
	}

	return customerIDPrefix + slugify(orgName) + "-" + suffix, nil
}

// randomSuffix returns n crypto-random bytes as lowercase unpadded base32.
func randomSuffix(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate customer id suffix: %w", err)
	}

	enc := base32.StdEncoding.WithPadding(base32.NoPadding)

	return strings.ToLower(enc.EncodeToString(b)), nil
}

// slugify lowercases the name and collapses every run of non-alphanumeric
// characters into a single "-", trimming leading/trailing dashes and capping the
// length. An empty or symbol-only name yields the fallback slug.
func slugify(name string) string {
	var b strings.Builder

	lastDash := false

	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)

			lastDash = false
		} else if !lastDash && b.Len() > 0 {
			b.WriteByte('-')

			lastDash = true
		}

		if b.Len() >= customerIDSlugMax {
			break
		}
	}

	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		return customerIDFallback
	}

	return slug
}
