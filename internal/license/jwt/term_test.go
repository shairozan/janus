package jwt_test

import (
	"testing"
	"time"

	"github.com/pharmalytica/janus/internal/license/jwt"
	"github.com/pharmalytica/janus/internal/license/models"
)

func TestLicenseValidity(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	ptr := func(tm time.Time) *time.Time { return &tm }

	t.Run("derives from EndDate", func(t *testing.T) {
		end := now.AddDate(0, 6, 0) // 6 months out
		agr := &models.Agreement{EndDate: ptr(end)}

		got := jwt.LicenseValidity(agr, now)
		if got != end.Sub(now) {
			t.Fatalf("validity = %v, want %v", got, end.Sub(now))
		}
	})

	t.Run("defaults to one year from start when no EndDate", func(t *testing.T) {
		start := now
		agr := &models.Agreement{StartDate: ptr(start)}

		got := jwt.LicenseValidity(agr, now)
		want := start.AddDate(1, 0, 0).Sub(now)
		if got != want {
			t.Fatalf("validity = %v, want %v (1 year)", got, want)
		}
	})

	t.Run("defaults to one year from now when no dates", func(t *testing.T) {
		agr := &models.Agreement{}
		got := jwt.LicenseValidity(agr, now)
		want := now.AddDate(1, 0, 0).Sub(now)
		if got != want {
			t.Fatalf("validity = %v, want %v", got, want)
		}
	})

	t.Run("expired agreement yields zero", func(t *testing.T) {
		agr := &models.Agreement{EndDate: ptr(now.AddDate(0, -1, 0))} // ended a month ago
		if got := jwt.LicenseValidity(agr, now); got != 0 {
			t.Fatalf("validity = %v, want 0 for expired agreement", got)
		}
	})
}
