package server

import (
	"io"
	"log"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/license/billing"
	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/offboarding"
	"github.com/pharmalytica/janus/internal/license/portal"
	"github.com/pharmalytica/janus/internal/license/signup"
	"github.com/pharmalytica/janus/internal/license/sso"
)

// routedTestServer builds a Server with every optional service set non-nil so
// all gated route groups (customer-admin, billing) register. The services are
// zero-value pointers — route registration only captures them in handler
// closures, it never calls into them — so no database or fakes are needed.
func routedTestServer() *Server {
	s := &Server{
		db:          &db.DB{},
		logger:      log.New(io.Discard, "", 0),
		mux:         http.NewServeMux(),
		portalSvc:   &portal.Service{},
		adminSvc:    &portal.AdminService{},
		offboarder:  &offboarding.Service{},
		ssoSvc:      &sso.Service{},
		catalogSvc:  &billing.CatalogService{},
		proposalSvc: &billing.ProposalService{},
		seatSvc:     &billing.SeatService{},
		signupSvc:   &signup.Service{},
	}
	s.routes()

	return s
}

// TestSignupRouteIsPublicAndUnaudited documents that /api/v1/signup is
// intentionally public and exempt from the audit-coverage requirement (like the
// Stripe webhook): a prospect has no identity to attribute an audit event to.
func TestSignupRouteIsPublicAndUnaudited(t *testing.T) {
	s := routedTestServer()

	var found *routeRecord

	for i := range s.routeRecords {
		if strings.Contains(s.routeRecords[i].pattern, "/api/v1/signup") {
			found = &s.routeRecords[i]

			break
		}
	}

	require.NotNil(t, found, "signup route should be registered when signupSvc is set")
	assert.False(t, found.audited, "signup is public/unauthenticated → not audited")
	assert.False(t, isPortalRoute("/api/v1/signup"), "signup must not count as a portal route")
}

// isPortalRoute reports whether a route path is customer-facing (the surface the
// audit log must cover): the end-user /me/* routes and the customer-admin
// /orgs/{id}/* routes. Staff/public routes (/api/v1/billing/*, /organizations,
// /agreements, token/key management) are out of scope.
func isPortalRoute(path string) bool {
	return path == "/api/v1/me" ||
		strings.HasPrefix(path, "/api/v1/me/") ||
		strings.HasPrefix(path, "/api/v1/orgs/{id}/")
}

var mutatingMethods = map[string]bool{
	http.MethodPost:   true,
	http.MethodPut:    true,
	http.MethodPatch:  true,
	http.MethodDelete: true,
}

// auditCoverage partitions the registered routes into the count of mutating
// portal routes seen and the subset of those missing an Audit tag.
func auditCoverage(records []routeRecord) (mutatingPortal int, unaudited []string) {
	for _, r := range records {
		_, path, _ := strings.Cut(r.pattern, " ")
		if !mutatingMethods[r.method] || !isPortalRoute(path) {
			continue
		}

		mutatingPortal++
		if !r.audited {
			unaudited = append(unaudited, r.pattern)
		}
	}

	return mutatingPortal, unaudited
}

// TestPortalMutatingRoutesAreAudited is the A9 guardrail (issue #146): every
// mutating (POST/PUT/PATCH/DELETE) customer-facing portal route must be
// registered with an Audit(...) tag. A new mutating route added without one — via
// adminRead/meRead or a bare s.mux.Handle — fails here instead of silently going
// unaudited.
func TestPortalMutatingRoutesAreAudited(t *testing.T) {
	s := routedTestServer()

	mutatingPortalRoutes, unaudited := auditCoverage(s.routeRecords)

	// Guard against a vacuous pass: if registration silently produced no mutating
	// portal routes, the check above would have nothing to verify.
	require.GreaterOrEqual(t, mutatingPortalRoutes, 10,
		"expected the mutating portal routes to be registered; got %d (did the route groups register?)", mutatingPortalRoutes)

	assert.Empty(t, unaudited,
		"these mutating portal routes are NOT wrapped with Audit(...) — register them via adminMutate/meMutate: %v", unaudited)
}

// TestGuardrailDetectsUnauditedRoute proves the guardrail itself works: a
// mutating portal route registered without an Audit tag must be flagged. (If this
// ever passed silently, the check above would be worthless.)
func TestGuardrailDetectsUnauditedRoute(t *testing.T) {
	records := []routeRecord{
		{method: http.MethodGet, pattern: "GET /api/v1/orgs/{id}/members", audited: false},                   // read — fine
		{method: http.MethodPost, pattern: "POST /api/v1/orgs/{id}/members/{userId}/promote", audited: true}, // audited — fine
		{method: http.MethodPost, pattern: "POST /api/v1/orgs/{id}/widgets", audited: false},                 // mutating + unaudited — must be caught
		{method: http.MethodPost, pattern: "POST /api/v1/billing/webhook", audited: false},                   // staff route, out of scope — ignored
	}

	mutatingPortal, unaudited := auditCoverage(records)

	assert.Equal(t, 2, mutatingPortal, "should count both the promote and widgets POSTs, but not the staff webhook")
	assert.Equal(t, []string{"POST /api/v1/orgs/{id}/widgets"}, unaudited)
}
