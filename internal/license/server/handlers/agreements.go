package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/pharmalytica/janus/internal/license/models"
	"gorm.io/gorm"
)

// CreateAgreementRequest represents the request to create an agreement
type CreateAgreementRequest struct {
	OrganizationID int64    `json:"organization_id" validate:"required"`
	Tier           string   `json:"tier" validate:"required"`
	MaxSeats       int      `json:"max_seats" validate:"required"`
	Features       []string `json:"features"`
	CostPerMonth   *float64 `json:"cost_per_month,omitempty"`
	StartDate      string   `json:"start_date" validate:"required"`
	EndDate        *string  `json:"end_date,omitempty"`
}

// UpdateAgreementRequest represents the request to update an agreement
type UpdateAgreementRequest struct {
	Tier         *string   `json:"tier,omitempty"`
	MaxSeats     *int      `json:"max_seats,omitempty"`
	Features     *[]string `json:"features,omitempty"`
	CostPerMonth *float64  `json:"cost_per_month,omitempty"`
	EndDate      *string   `json:"end_date,omitempty"`
}

// AgreementResponse represents the response for agreement operations
type AgreementResponse struct {
	ID             int64      `json:"id"`
	OrganizationID int64      `json:"organization_id"`
	Tier           string     `json:"tier"`
	MaxSeats       int        `json:"max_seats"`
	Features       []string   `json:"features"`
	CostPerMonth   *float64   `json:"cost_per_month,omitempty"`
	StartDate      time.Time  `json:"start_date"`
	EndDate        *time.Time `json:"end_date,omitempty"`
	DeactivatedAt  *time.Time `json:"deactivated_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func agreementToResponse(agr *models.Agreement) AgreementResponse {
	resp := AgreementResponse{
		ID:             agr.ID,
		OrganizationID: agr.OrganizationID,
		CostPerMonth:   agr.CostPerMonth,
		EndDate:        agr.EndDate,
		DeactivatedAt:  agr.DeactivatedAt,
		CreatedAt:      agr.CreatedAt,
		UpdatedAt:      agr.UpdatedAt,
	}

	if agr.Tier != nil {
		resp.Tier = *agr.Tier
	}
	if agr.MaxSeats != nil {
		resp.MaxSeats = *agr.MaxSeats
	}
	if agr.Features != nil {
		resp.Features = agr.Features
	}
	if agr.StartDate != nil {
		resp.StartDate = *agr.StartDate
	}

	return resp
}

// CreateAgreement creates a new agreement
func CreateAgreement(
	db *gorm.DB,
	logger *log.Logger,
	writeJSON func(http.ResponseWriter, int, interface{}),
	writeError func(http.ResponseWriter, int, string)) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateAgreementRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request body")

			return
		}

		// Validate required fields
		if req.OrganizationID == 0 {
			writeError(w, http.StatusBadRequest, "organization_id is required")

			return
		}
		if req.Tier == "" {
			writeError(w, http.StatusBadRequest, "tier is required")

			return
		}
		if req.MaxSeats <= 0 {
			writeError(w, http.StatusBadRequest, "max_seats must be greater than 0")

			return
		}
		if req.StartDate == "" {
			writeError(w, http.StatusBadRequest, "start_date is required")

			return
		}

		// Parse dates
		startDate, err := time.Parse("2006-01-02", req.StartDate)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid start_date format (use YYYY-MM-DD)")

			return
		}

		var endDate *time.Time
		if req.EndDate != nil && *req.EndDate != "" {
			parsed, err := time.Parse("2006-01-02", *req.EndDate)
			if err != nil {
				writeError(w, http.StatusBadRequest, "Invalid end_date format (use YYYY-MM-DD)")

				return
			}
			endDate = &parsed
		}

		// Look up license by tier using GORM
		var lic models.License
		if err := db.Where("tier = ? AND deactivated_at IS NULL", req.Tier).First(&lic).Error; err != nil {
			logger.Printf("Failed to find license for tier '%s': %v", req.Tier, err)
			writeError(w, http.StatusBadRequest, "Invalid tier or license not found")

			return
		}

		// Convert []string to NullableFeatures
		var features models.NullableFeatures
		if len(req.Features) > 0 {
			features = models.NullableFeatures(req.Features)
		}

		seats := req.MaxSeats
		licenseModel := "named"
		pricePerSeatCents := 0
		if req.CostPerMonth != nil && req.MaxSeats > 0 {
			pricePerSeatCents = int((*req.CostPerMonth * 100) / float64(req.MaxSeats))
		}

		agr := &models.Agreement{
			OrganizationID:    req.OrganizationID,
			LicenseID:         lic.ID,
			Seats:             seats,
			PricePerSeatCents: pricePerSeatCents,
			LicenseModel:      licenseModel,
			Tier:              &req.Tier,
			MaxSeats:          &req.MaxSeats,
			Features:          features,
			CostPerMonth:      req.CostPerMonth,
			StartDate:         &startDate,
			EndDate:           endDate,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}

		if err := db.Create(agr).Error; err != nil {
			logger.Printf("Failed to create agreement: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to create agreement")

			return
		}

		tierStr := ""
		if agr.Tier != nil {
			tierStr = *agr.Tier
		}
		logger.Printf("Created agreement for org %d: tier=%s (ID: %d)", agr.OrganizationID, tierStr, agr.ID)
		writeJSON(w, http.StatusCreated, agreementToResponse(agr))
	}
}

// GetAgreement retrieves an agreement by ID
func GetAgreement(
	db *gorm.DB,
	logger *log.Logger,
	writeJSON func(http.ResponseWriter, int, interface{}),
	writeError func(http.ResponseWriter, int, string)) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.URL.Query().Get("id")
		if idStr == "" {
			writeError(w, http.StatusBadRequest, "Agreement ID is required")

			return
		}

		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid agreement ID")

			return
		}

		var agr models.Agreement
		if err := db.First(&agr, id).Error; err != nil {
			logger.Printf("Failed to get agreement %d: %v", id, err)
			writeError(w, http.StatusNotFound, "Agreement not found")

			return
		}

		writeJSON(w, http.StatusOK, agreementToResponse(&agr))
	}
}

// ListAgreements retrieves all agreements (optionally filtered by organization)
func ListAgreements(
	db *gorm.DB,
	logger *log.Logger,
	writeJSON func(http.ResponseWriter, int, interface{}),
	writeError func(http.ResponseWriter, int, string)) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		orgIDStr := r.URL.Query().Get("organization_id")

		var agrs []models.Agreement
		query := db.Where("deactivated_at IS NULL")

		if orgIDStr != "" {
			orgID, parseErr := strconv.ParseInt(orgIDStr, 10, 64)
			if parseErr != nil {
				writeError(w, http.StatusBadRequest, "Invalid organization_id")

				return
			}
			query = query.Where("organization_id = ?", orgID)
		}

		if err := query.Find(&agrs).Error; err != nil {
			logger.Printf("Failed to list agreements: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to list agreements")

			return
		}

		response := make([]AgreementResponse, len(agrs))
		for i := range agrs {
			response[i] = agreementToResponse(&agrs[i])
		}

		writeJSON(w, http.StatusOK, response)
	}
}

// UpdateAgreement updates an existing agreement
func UpdateAgreement(
	db *gorm.DB,
	logger *log.Logger,
	writeJSON func(http.ResponseWriter, int, interface{}),
	writeError func(http.ResponseWriter, int, string)) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.URL.Query().Get("id")
		if idStr == "" {
			writeError(w, http.StatusBadRequest, "Agreement ID is required")

			return
		}

		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid agreement ID")

			return
		}

		var req UpdateAgreementRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request body")

			return
		}

		var agr models.Agreement
		if err := db.First(&agr, id).Error; err != nil {
			logger.Printf("Failed to get agreement %d: %v", id, err)
			writeError(w, http.StatusNotFound, "Agreement not found")

			return
		}

		// Update fields if provided
		if req.Tier != nil {
			agr.Tier = req.Tier
		}
		if req.MaxSeats != nil {
			agr.MaxSeats = req.MaxSeats
		}
		if req.Features != nil {
			agr.Features = models.NullableFeatures(*req.Features)
		}
		if req.CostPerMonth != nil {
			agr.CostPerMonth = req.CostPerMonth
		}
		if req.EndDate != nil && *req.EndDate != "" {
			parsed, err := time.Parse("2006-01-02", *req.EndDate)
			if err != nil {
				writeError(w, http.StatusBadRequest, "Invalid end_date format (use YYYY-MM-DD)")

				return
			}
			agr.EndDate = &parsed
		}

		if err := db.Save(&agr).Error; err != nil {
			logger.Printf("Failed to update agreement %d: %v", id, err)
			writeError(w, http.StatusInternalServerError, "Failed to update agreement")

			return
		}

		tierStr := ""
		if agr.Tier != nil {
			tierStr = *agr.Tier
		}
		logger.Printf("Updated agreement ID: %d (tier=%s)", agr.ID, tierStr)
		writeJSON(w, http.StatusOK, agreementToResponse(&agr))
	}
}

// DeleteAgreement soft-deletes an agreement by setting deactivated_at
func DeleteAgreement(
	db *gorm.DB,
	logger *log.Logger,
	writeJSON func(http.ResponseWriter, int, interface{}),
	writeError func(http.ResponseWriter, int, string)) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.URL.Query().Get("id")
		if idStr == "" {
			writeError(w, http.StatusBadRequest, "Agreement ID is required")

			return
		}

		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid agreement ID")

			return
		}

		now := time.Now()
		if err := db.Model(&models.Agreement{}).Where("id = ?", id).Update("deactivated_at", now).Error; err != nil {
			logger.Printf("Failed to delete agreement %d: %v", id, err)
			writeError(w, http.StatusInternalServerError, "Failed to delete agreement")

			return
		}

		logger.Printf("Deleted agreement ID: %d", id)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"message": "Agreement deleted successfully",
			"id":      id,
		})
	}
}
