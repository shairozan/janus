package signup

import (
	"regexp"
	"strings"
	"testing"
)

var customerIDPattern = regexp.MustCompile(`^cust-[a-z0-9-]+-[a-z2-7]+$`)

func TestGenerateCustomerID(t *testing.T) {
	cases := []struct {
		name     string
		orgName  string
		wantSlug string // the slug segment expected (between prefix and suffix)
	}{
		{"simple", "Acme", "acme"},
		{"spaces and case", "Acme Labs", "acme-labs"},
		{"symbols collapse", "  Björn & Co., Inc!!  ", "bj-rn-co-inc"},
		{"empty falls back", "", customerIDFallback},
		{"symbols only falls back", "@#$ %^&", customerIDFallback},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id, err := GenerateCustomerID(tc.orgName)
			if err != nil {
				t.Fatalf("GenerateCustomerID: %v", err)
			}

			if !strings.HasPrefix(id, customerIDPrefix) {
				t.Errorf("id %q missing prefix %q", id, customerIDPrefix)
			}

			if len(id) > 100 {
				t.Errorf("id %q exceeds 100-char column limit (%d)", id, len(id))
			}

			if !customerIDPattern.MatchString(id) {
				t.Errorf("id %q does not match %v", id, customerIDPattern)
			}

			wantPrefix := customerIDPrefix + tc.wantSlug + "-"
			if !strings.HasPrefix(id, wantPrefix) {
				t.Errorf("id %q does not start with expected slug segment %q", id, wantPrefix)
			}
		})
	}
}

func TestGenerateCustomerID_Unique(t *testing.T) {
	// Two calls for the same org name must differ — proves the crypto-random suffix.
	a, err := GenerateCustomerID("Acme Labs")
	if err != nil {
		t.Fatalf("GenerateCustomerID: %v", err)
	}

	b, err := GenerateCustomerID("Acme Labs")
	if err != nil {
		t.Fatalf("GenerateCustomerID: %v", err)
	}

	if a == b {
		t.Errorf("expected distinct ids, got %q twice", a)
	}
}

func TestSlugifyCap(t *testing.T) {
	long := strings.Repeat("abcde ", 40) // far longer than the cap

	slug := slugify(long)
	if len(slug) > customerIDSlugMax {
		t.Errorf("slug %q length %d exceeds cap %d", slug, len(slug), customerIDSlugMax)
	}

	if strings.HasPrefix(slug, "-") || strings.HasSuffix(slug, "-") {
		t.Errorf("slug %q has leading/trailing dash", slug)
	}
}
