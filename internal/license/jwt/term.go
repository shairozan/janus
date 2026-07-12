package jwt

import (
	"time"

	"github.com/pharmalytica/janus/internal/license/models"
)

// LicenseValidity returns how long a license issued at `now` should remain valid,
// derived from the negotiated agreement term: the span from now until the
// agreement's EndDate. If EndDate is unset it defaults to one calendar year from
// the agreement's start (or now). The result is never negative — an already-
// expired agreement yields 0, and the caller should refuse to issue.
func LicenseValidity(agr *models.Agreement, now time.Time) time.Duration {
	d := licenseExpiry(agr, now).Sub(now)
	if d < 0 {
		return 0
	}

	return d
}

func licenseExpiry(agr *models.Agreement, now time.Time) time.Time {
	if agr.EndDate != nil {
		return *agr.EndDate
	}

	start := now
	if agr.StartDate != nil {
		start = *agr.StartDate
	}

	return start.AddDate(1, 0, 0)
}
