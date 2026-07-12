package handlers

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/version"
)

// Health handles GET /health requests.
func Health(database *db.DB, logger *log.Logger, writeJSON func(http.ResponseWriter, int, interface{}), writeError func(http.ResponseWriter, int, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		// Check database connectivity via GORM
		sqlDB, err := database.DB.DB()
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, fmt.Sprintf("database unhealthy: %v", err))

			return
		}

		if err := sqlDB.Ping(); err != nil {
			writeError(w, http.StatusServiceUnavailable, fmt.Sprintf("database unhealthy: %v", err))

			return
		}

		// Build response with version information
		response := map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now().UTC(),
			"version":   version.GetBuildInfo(),
		}

		writeJSON(w, http.StatusOK, response)
	}
}