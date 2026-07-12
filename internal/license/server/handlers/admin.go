package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/pharmalytica/janus/internal/license/models"
	"github.com/pharmalytica/janus/internal/license/offboarding"
	"github.com/pharmalytica/janus/internal/license/portal"
	"github.com/pharmalytica/janus/internal/license/server/middleware"
	"github.com/pharmalytica/janus/internal/license/sso"
)

// ssoResponse renders an SSO config with the client secret redacted to a presence flag.
func ssoResponse(c *models.SSOConfiguration) map[string]interface{} {
	return map[string]interface{}{
		"id":                c.ID,
		"organization_id":   c.OrganizationID,
		"provider_name":     c.ProviderName,
		"user_pool_id":      c.UserPoolID,
		"oidc_issuer":       c.OIDCIssuer,
		"oidc_client_id":    c.OIDCClientID,
		"scopes":            c.Scopes,
		"attribute_mapping": c.AttributeMapping,
		"cognito_status":    c.CognitoStatus,
		"login_url":         c.LoginURL,
		"has_client_secret": c.HasClientSecret(),
	}
}

// pathInt extracts an int64 path value (Go 1.22 mux wildcard).
func pathInt(r *http.Request, name string) (int64, bool) {
	v := r.PathValue(name)
	if v == "" {
		return 0, false
	}

	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, false
	}

	return n, true
}

// actingAdmin returns the resolved OrgUser (the customer admin) from context.
func actingAdmin(r *http.Request) (orgID int64, ok bool) {
	user, found := middleware.GetOrgUser(r)
	if !found || user == nil {
		return 0, false
	}

	return user.OrganizationID, true
}

// AdminListMembers handles GET /api/v1/orgs/{id}/members.
func AdminListMembers(svc *portal.AdminService, writeJSON jsonWriter, writeError errWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, ok := actingAdmin(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")

			return
		}

		members, err := svc.ListMembers(r.Context(), orgID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list members")

			return
		}

		writeJSON(w, http.StatusOK, members)
	}
}

// AdminPromote handles POST /api/v1/orgs/{id}/members/{userId}/promote.
func AdminPromote(svc *portal.AdminService, writeJSON jsonWriter, writeError errWriter) http.HandlerFunc {
	return roleChangeHandler(svc.Promote, "member", "promote", writeJSON, writeError)
}

// AdminDemote handles POST /api/v1/orgs/{id}/members/{userId}/demote.
func AdminDemote(svc *portal.AdminService, writeJSON jsonWriter, writeError errWriter) http.HandlerFunc {
	return roleChangeHandler(svc.Demote, "member", "demote", writeJSON, writeError)
}

func roleChangeHandler(
	change func(ctx context.Context, orgID, userID int64) (*models.OrgUser, error),
	_ string, _ string,
	writeJSON jsonWriter, writeError errWriter,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, ok := actingAdmin(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")

			return
		}

		userID, ok := pathInt(r, "userId")
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid user id")

			return
		}

		u, err := change(r.Context(), orgID, userID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())

			return
		}

		middleware.SetAuditResourceID(r, fmt.Sprintf("%d", userID))
		middleware.Enrich(r, "role", u.Role)

		writeJSON(w, http.StatusOK, u)
	}
}

// AdminOffboard handles POST /api/v1/orgs/{id}/members/{userId}/offboard.
func AdminOffboard(svc *portal.AdminService, off *offboarding.Service, writeJSON jsonWriter, writeError errWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, ok := actingAdmin(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")

			return
		}

		userID, ok := pathInt(r, "userId")
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid user id")

			return
		}

		member, err := svc.Member(r.Context(), orgID, userID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())

			return
		}

		if err := off.Offboard(r.Context(), member); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())

			return
		}

		middleware.SetAuditResourceID(r, fmt.Sprintf("%d", userID))

		writeJSON(w, http.StatusOK, map[string]string{"status": "offboarded"})
	}
}

// AdminListActivity handles GET /api/v1/orgs/{id}/activity.
func AdminListActivity(svc *portal.AdminService, writeJSON jsonWriter, writeError errWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, ok := actingAdmin(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")

			return
		}

		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

		events, err := svc.ListActivity(r.Context(), orgID, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load activity")

			return
		}

		writeJSON(w, http.StatusOK, events)
	}
}

// AdminListAgreements handles GET /api/v1/orgs/{id}/agreements.
func AdminListAgreements(svc *portal.AdminService, writeJSON jsonWriter, writeError errWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, ok := actingAdmin(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")

			return
		}

		agreements, err := svc.ListAgreements(r.Context(), orgID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load agreements")

			return
		}

		writeJSON(w, http.StatusOK, agreements)
	}
}

// AdminListRequests handles GET /api/v1/orgs/{id}/license-requests.
func AdminListRequests(svc *portal.AdminService, writeJSON jsonWriter, writeError errWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, ok := actingAdmin(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")

			return
		}

		reqs, err := svc.ListRequests(r.Context(), orgID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list requests")

			return
		}

		writeJSON(w, http.StatusOK, reqs)
	}
}

// AdminApproveRequest handles POST /api/v1/orgs/{id}/license-requests/{reqId}/approve.
func AdminApproveRequest(svc *portal.Service, writeJSON jsonWriter, writeError errWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		admin, ok := middleware.GetOrgUser(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")

			return
		}

		reqID, ok := pathInt(r, "reqId")
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid request id")

			return
		}

		lr, err := svc.ApproveRequest(r.Context(), admin, reqID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())

			return
		}

		middleware.SetAuditResourceID(r, fmt.Sprintf("%d", reqID))

		writeJSON(w, http.StatusOK, lr)
	}
}

// RejectBody is the POST .../reject body.
type RejectBody struct {
	Reason string `json:"reason"`
}

// AdminRejectRequest handles POST /api/v1/orgs/{id}/license-requests/{reqId}/reject.
func AdminRejectRequest(svc *portal.Service, writeJSON jsonWriter, writeError errWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		admin, ok := middleware.GetOrgUser(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")

			return
		}

		reqID, ok := pathInt(r, "reqId")
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid request id")

			return
		}

		var body RejectBody
		_ = json.NewDecoder(r.Body).Decode(&body)

		lr, err := svc.RejectRequest(r.Context(), admin, reqID, body.Reason)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())

			return
		}

		middleware.SetAuditResourceID(r, fmt.Sprintf("%d", reqID))

		writeJSON(w, http.StatusOK, lr)
	}
}

// CreateRuleBody is the POST /orgs/{id}/rules body.
type CreateRuleBody struct {
	AgreementID  *int64  `json:"agreement_id"`
	RuleType     string  `json:"rule_type"`
	MatchDomain  *string `json:"match_domain"`
	MaxAutoSeats *int    `json:"max_auto_seats"`
}

// AdminCreateRule handles POST /api/v1/orgs/{id}/rules.
func AdminCreateRule(svc *portal.AdminService, writeJSON jsonWriter, writeError errWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		admin, ok := middleware.GetOrgUser(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")

			return
		}

		var body CreateRuleBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		rule, err := svc.CreateRule(r.Context(), admin.OrganizationID, admin.ID, portal.RuleInput{
			AgreementID:  body.AgreementID,
			RuleType:     body.RuleType,
			MatchDomain:  body.MatchDomain,
			MaxAutoSeats: body.MaxAutoSeats,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())

			return
		}

		middleware.SetAuditResourceID(r, fmt.Sprintf("%d", rule.ID))

		writeJSON(w, http.StatusCreated, rule)
	}
}

// AdminListRules handles GET /api/v1/orgs/{id}/rules.
func AdminListRules(svc *portal.AdminService, writeJSON jsonWriter, writeError errWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, ok := actingAdmin(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")

			return
		}

		rules, err := svc.ListRules(r.Context(), orgID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list rules")

			return
		}

		writeJSON(w, http.StatusOK, rules)
	}
}

// SetRuleBody is the PUT /orgs/{id}/rules/{ruleId} body.
type SetRuleBody struct {
	Enabled bool `json:"enabled"`
}

// AdminSetRuleEnabled handles PUT /api/v1/orgs/{id}/rules/{ruleId}.
func AdminSetRuleEnabled(svc *portal.AdminService, writeJSON jsonWriter, writeError errWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, ok := actingAdmin(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")

			return
		}

		ruleID, ok := pathInt(r, "ruleId")
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid rule id")

			return
		}

		var body SetRuleBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		rule, err := svc.SetRuleEnabled(r.Context(), orgID, ruleID, body.Enabled)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())

			return
		}

		middleware.SetAuditResourceID(r, fmt.Sprintf("%d", ruleID))

		writeJSON(w, http.StatusOK, rule)
	}
}

// AdminDeleteRule handles DELETE /api/v1/orgs/{id}/rules/{ruleId}.
func AdminDeleteRule(svc *portal.AdminService, writeJSON jsonWriter, writeError errWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, ok := actingAdmin(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")

			return
		}

		ruleID, ok := pathInt(r, "ruleId")
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid rule id")

			return
		}

		if err := svc.DeleteRule(r.Context(), orgID, ruleID); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())

			return
		}

		middleware.SetAuditResourceID(r, fmt.Sprintf("%d", ruleID))

		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}

// SSOSetupBody is the POST /orgs/{id}/sso body.
type SSOSetupBody struct {
	ProviderName string            `json:"provider_name"`
	OIDCIssuer   string            `json:"oidc_issuer"`
	ClientID     string            `json:"client_id"`
	ClientSecret string            `json:"client_secret"`
	Scopes       string            `json:"scopes"`
	AttributeMap map[string]string `json:"attribute_map"`
}

// AdminSSOSetup handles POST /api/v1/orgs/{id}/sso (create-once).
func AdminSSOSetup(svc *sso.Service, writeJSON jsonWriter, writeError errWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, ok := actingAdmin(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")

			return
		}

		var body SSOSetupBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		cfg, err := svc.Setup(r.Context(), sso.SetupInput{
			OrganizationID: orgID,
			ProviderName:   body.ProviderName,
			OIDCIssuer:     body.OIDCIssuer,
			ClientID:       body.ClientID,
			ClientSecret:   body.ClientSecret,
			Scopes:         body.Scopes,
			AttributeMap:   body.AttributeMap,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())

			return
		}

		middleware.SetAuditResourceID(r, fmt.Sprintf("%d", cfg.ID))

		writeJSON(w, http.StatusCreated, ssoResponse(cfg))
	}
}

// AdminSSOGet handles GET /api/v1/orgs/{id}/sso (read-only).
func AdminSSOGet(svc *sso.Service, writeJSON jsonWriter, writeError errWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, ok := actingAdmin(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")

			return
		}

		cfg, err := svc.Get(r.Context(), orgID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())

			return
		}

		writeJSON(w, http.StatusOK, ssoResponse(cfg))
	}
}
