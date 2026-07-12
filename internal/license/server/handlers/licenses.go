package handlers

import (
	"log"
	"net/http"

	"gorm.io/gorm"

	"github.com/pharmalytica/janus/internal/license/models"
)

// LicenseResponse is the staff-facing view of a license tier/product. It's the
// catalog the agreement-create form picks a tier from (CreateAgreement resolves
// the agreement's License by tier, so a tier must exist here first).
type LicenseResponse struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name"`
	Tier      string   `json:"tier"`
	Features  []string `json:"features"`
	MSRPCents int      `json:"msrp_cents"`
}

// ListLicenses returns the active license tiers (Janus staff). A thin DB read
// mirroring ListAgreements — the authz (email-domain gate) is applied at the
// route.
func ListLicenses(
	db *gorm.DB,
	logger *log.Logger,
	writeJSON func(http.ResponseWriter, int, interface{}),
	writeError func(http.ResponseWriter, int, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		var licenses []models.License
		if err := db.Where("deactivated_at IS NULL").Order("msrp_cents").Find(&licenses).Error; err != nil {
			logger.Printf("Failed to list licenses: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to list licenses")

			return
		}

		response := make([]LicenseResponse, len(licenses))
		for i := range licenses {
			response[i] = LicenseResponse{
				ID:        licenses[i].ID,
				Name:      licenses[i].Name,
				Tier:      licenses[i].Tier,
				Features:  licenses[i].ResolveFeatures(),
				MSRPCents: licenses[i].MSRPCents,
			}
		}

		writeJSON(w, http.StatusOK, response)
	}
}
