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

// CreateOrganizationRequest represents the request to create an organization
type CreateOrganizationRequest struct {
	Name       string `json:"name" validate:"required"`
	CustomerID string `json:"customer_id" validate:"required"`
}

// UpdateOrganizationRequest represents the request to update an organization
type UpdateOrganizationRequest struct {
	Name *string `json:"name,omitempty"`
}

// OrganizationResponse represents the response for organization operations
type OrganizationResponse struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	CustomerID    string     `json:"customer_id"`
	CreatedAt     time.Time  `json:"created_at"`
	DeactivatedAt *time.Time `json:"deactivated_at,omitempty"`
}

func organizationToResponse(org *models.Organization) OrganizationResponse {
	return OrganizationResponse{
		ID:            org.ID,
		Name:          org.Name,
		CustomerID:    org.CustomerID,
		CreatedAt:     org.CreatedAt,
		DeactivatedAt: org.DeactivatedAt,
	}
}

// CreateOrganization creates a new organization
func CreateOrganization(db *gorm.DB, logger *log.Logger,
	writeJSON func(http.ResponseWriter, int, interface{}),
	writeError func(http.ResponseWriter, int, string)) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateOrganizationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request body")

			return
		}

		if req.Name == "" || req.CustomerID == "" {
			writeError(w, http.StatusBadRequest, "Organization name and customer_id are required")

			return
		}

		org := &models.Organization{
			Name:       req.Name,
			CustomerID: req.CustomerID,
			CreatedAt:  time.Now(),
		}

		if err := db.Create(org).Error; err != nil {
			logger.Printf("Failed to create organization: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to create organization")

			return
		}

		logger.Printf("Created organization: %s (ID: %d)", org.Name, org.ID)
		writeJSON(w, http.StatusCreated, organizationToResponse(org))
	}
}

// GetOrganization retrieves an organization by ID
func GetOrganization(db *gorm.DB, logger *log.Logger,
	writeJSON func(http.ResponseWriter, int, interface{}),
	writeError func(http.ResponseWriter, int, string)) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.URL.Query().Get("id")
		if idStr == "" {
			writeError(w, http.StatusBadRequest, "Organization ID is required")

			return
		}

		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid organization ID")

			return
		}

		var org models.Organization
		if err := db.First(&org, id).Error; err != nil {
			logger.Printf("Failed to get organization %d: %v", id, err)
			writeError(w, http.StatusNotFound, "Organization not found")

			return
		}

		writeJSON(w, http.StatusOK, organizationToResponse(&org))
	}
}

// ListOrganizations retrieves all organizations
func ListOrganizations(db *gorm.DB, logger *log.Logger,
	writeJSON func(http.ResponseWriter, int, interface{}),
	writeError func(http.ResponseWriter, int, string)) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		var orgs []models.Organization
		if err := db.Where("deactivated_at IS NULL").Order("created_at DESC").Find(&orgs).Error; err != nil {
			logger.Printf("Failed to list organizations: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to list organizations")

			return
		}

		response := make([]OrganizationResponse, len(orgs))
		for i, org := range orgs {
			response[i] = organizationToResponse(&org)
		}

		writeJSON(w, http.StatusOK, response)
	}
}

// UpdateOrganization updates an existing organization
func UpdateOrganization(db *gorm.DB, logger *log.Logger,
	writeJSON func(http.ResponseWriter, int, interface{}),
	writeError func(http.ResponseWriter, int, string)) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.URL.Query().Get("id")
		if idStr == "" {
			writeError(w, http.StatusBadRequest, "Organization ID is required")

			return
		}

		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid organization ID")

			return
		}

		var req UpdateOrganizationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request body")

			return
		}

		var org models.Organization
		if err := db.First(&org, id).Error; err != nil {
			logger.Printf("Failed to get organization %d: %v", id, err)
			writeError(w, http.StatusNotFound, "Organization not found")

			return
		}

		if req.Name != nil {
			org.Name = *req.Name
		}

		if err := db.Save(&org).Error; err != nil {
			logger.Printf("Failed to update organization %d: %v", id, err)
			writeError(w, http.StatusInternalServerError, "Failed to update organization")

			return
		}

		logger.Printf("Updated organization: %s (ID: %d)", org.Name, org.ID)
		writeJSON(w, http.StatusOK, organizationToResponse(&org))
	}
}

// DeleteOrganization soft-deletes an organization by setting deactivated_at
func DeleteOrganization(db *gorm.DB, logger *log.Logger,
	writeJSON func(http.ResponseWriter, int, interface{}),
	writeError func(http.ResponseWriter, int, string)) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.URL.Query().Get("id")
		if idStr == "" {
			writeError(w, http.StatusBadRequest, "Organization ID is required")

			return
		}

		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid organization ID")

			return
		}

		now := time.Now()
		result := db.Model(&models.Organization{}).Where("id = ?", id).Update("deactivated_at", now)
		if result.Error != nil {
			logger.Printf("Failed to delete organization %d: %v", id, result.Error)
			writeError(w, http.StatusInternalServerError, "Failed to delete organization")

			return
		}

		if result.RowsAffected == 0 {
			writeError(w, http.StatusNotFound, "Organization not found")

			return
		}

		logger.Printf("Deleted organization ID: %d", id)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"message": "Organization deleted successfully",
			"id":      id,
		})
	}
}
