package requests

import (
	"context"
	"fmt"
	"time"

	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/models"
)

// ActiveSeats returns the number of seats currently consumed on an agreement:
// active (non-revoked, non-expired) issued tokens linked to the agreement via
// its approved license requests.
func ActiveSeats(ctx context.Context, database *db.DB, agreementID int64) (int, error) {
	var count int64

	err := database.DB.WithContext(ctx).
		Model(&models.IssuedToken{}).
		Joins("JOIN license_requests lr ON lr.issued_token_id = issued_tokens.id").
		Where("lr.agreement_id = ?", agreementID).
		Where("issued_tokens.revoked_at IS NULL").
		Where("issued_tokens.expires_at > ?", time.Now()).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("count active seats for agreement %d: %w", agreementID, err)
	}

	return int(count), nil
}
