package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/pharmalytica/janus/internal/license/portal"
	"github.com/pharmalytica/janus/internal/license/server/middleware"
)

// jsonWriter / errWriter match the server's writeJSON/writeError helpers.
type jsonWriter = func(http.ResponseWriter, int, interface{})
type errWriter = func(http.ResponseWriter, int, string)

// MeProfile handles GET /api/v1/me. staffDomains is the Janus-staff email-domain
// allowlist; the handler stamps is_staff so the portal can render the staff
// "Admin" UI (the staff endpoints themselves stay gated by requireUserInfo).
func MeProfile(svc *portal.Service, staffDomains []string, writeJSON jsonWriter, writeError errWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := middleware.GetOrgUser(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")

			return
		}

		p, err := svc.Profile(r.Context(), user)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load profile")

			return
		}

		p.IsStaff = middleware.IsAllowedDomain(user.Email, staffDomains)

		writeJSON(w, http.StatusOK, p)
	}
}

// MeGetKey handles GET /api/v1/me/key.
func MeGetKey(svc *portal.Service, writeJSON jsonWriter, writeError errWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := middleware.GetOrgUser(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")

			return
		}

		active, history, err := svc.Keys(r.Context(), user)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load keys")

			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{"active": active, "history": history})
	}
}

// ReplaceKeyRequest is the PUT /api/v1/me/key body. Acknowledged must be true —
// the caller must explicitly accept the rotation caveat (it invalidates prior
// run-log signature continuity and requires updating the local private key).
type ReplaceKeyRequest struct {
	PublicKeyPEM string `json:"public_key_pem"`
	Title        string `json:"title"`
	Acknowledged bool   `json:"acknowledged"`
}

// MeReplaceKey handles PUT /api/v1/me/key.
func MeReplaceKey(svc *portal.Service, logger *log.Logger, writeJSON jsonWriter, writeError errWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := middleware.GetOrgUser(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")

			return
		}

		var req ReplaceKeyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))

			return
		}

		if req.PublicKeyPEM == "" {
			writeError(w, http.StatusBadRequest, "public_key_pem is required")

			return
		}

		if !req.Acknowledged {
			writeError(w, http.StatusBadRequest, "you must acknowledge that replacing your key invalidates prior run-log continuity and requires updating your local private key")

			return
		}

		key, err := svc.ReplaceKey(r.Context(), user, req.PublicKeyPEM, req.Title)
		if err != nil {
			logger.Printf("replace key failed for sub %s: %v", user.CognitoSub, err)
			writeError(w, http.StatusBadRequest, fmt.Sprintf("failed to set key: %v", err))

			return
		}

		middleware.SetAuditResourceID(r, fmt.Sprintf("%d", key.ID))
		middleware.Enrich(r, "fingerprint", key.Fingerprint)

		writeJSON(w, http.StatusOK, key)
	}
}

// RequestLicenseBody is the POST /api/v1/me/license-requests body.
type RequestLicenseBody struct {
	AgreementID int64 `json:"agreement_id"`
}

// MeRequestLicense handles POST /api/v1/me/license-requests.
func MeRequestLicense(svc *portal.Service, writeJSON jsonWriter, writeError errWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := middleware.GetOrgUser(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")

			return
		}

		var req RequestLicenseBody
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))

			return
		}

		if req.AgreementID == 0 {
			writeError(w, http.StatusBadRequest, "agreement_id is required")

			return
		}

		lr, err := svc.RequestLicense(r.Context(), user, req.AgreementID)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("failed to request license: %v", err))

			return
		}

		middleware.SetAuditResourceID(r, fmt.Sprintf("%d", lr.ID))
		middleware.Enrich(r, "status", lr.Status)

		writeJSON(w, http.StatusOK, lr)
	}
}

// MeListRequests handles GET /api/v1/me/license-requests.
func MeListRequests(svc *portal.Service, writeJSON jsonWriter, writeError errWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := middleware.GetOrgUser(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")

			return
		}

		reqs, err := svc.ListRequests(r.Context(), user)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list requests")

			return
		}

		writeJSON(w, http.StatusOK, reqs)
	}
}

// MeGetLicense handles GET /api/v1/me/license — downloads the active license JWT.
func MeGetLicense(svc *portal.Service, writeJSON jsonWriter, writeError errWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := middleware.GetOrgUser(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")

			return
		}

		token, err := svc.License(r.Context(), user)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())

			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"license": token})
	}
}
