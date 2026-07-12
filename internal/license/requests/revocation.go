package requests

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/models"
)

// IsTokenRevoked reports whether the issued token with the given JTI has been
// revoked. This backs the online revocation check on /tokens/validate.
//
// A JTI not present in issued_tokens is treated as NOT revoked: the token's
// signature has already been verified by the caller, and absence just means the
// token isn't tracked (e.g. predates tracking) rather than that it was revoked.
func IsTokenRevoked(ctx context.Context, database *db.DB, jti string) (bool, error) {
	var tok models.IssuedToken

	err := database.DB.WithContext(ctx).
		Select("revoked_at").
		Where("jti = ?", jti).
		First(&tok).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("lookup token %q: %w", jti, err)
	}

	return tok.RevokedAt != nil, nil
}
