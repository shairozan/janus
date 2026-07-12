package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/pharmalytica/janus/internal/license/auth"
	"github.com/pharmalytica/janus/internal/license/billing"
	"github.com/pharmalytica/janus/internal/license/cognito"
	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/jwt"
	"github.com/pharmalytica/janus/internal/license/keys"
	"github.com/pharmalytica/janus/internal/license/models"
	"github.com/pharmalytica/janus/internal/license/offboarding"
	"github.com/pharmalytica/janus/internal/license/portal"
	"github.com/pharmalytica/janus/internal/license/server/handlers"
	"github.com/pharmalytica/janus/internal/license/server/middleware"
	"github.com/pharmalytica/janus/internal/license/signup"
	"github.com/pharmalytica/janus/internal/license/sso"
)

// Signup rate limits (per key, per window). The public signup endpoint is
// abuse-gated by these alone (no handoff token); email is the effective limiter
// on the portal BFF path, IP on direct browser calls.
const (
	signupPerEmailLimit = 3
	signupPerIPLimit    = 20
	signupRateWindow    = time.Hour
)

// Server represents the HTTP server for the license service.
type Server struct {
	db                 *db.DB
	keyMgr             *keys.Manager
	jwtGen             *jwt.Generator
	oidcValidator      auth.TokenValidator
	oidcAllowedDomains []string
	userInfoCache      *auth.UserInfoCache
	portalSvc          *portal.Service
	adminSvc           *portal.AdminService
	offboarder         *offboarding.Service
	ssoSvc             *sso.Service
	catalogSvc         *billing.CatalogService
	proposalSvc        *billing.ProposalService
	seatSvc            *billing.SeatService
	stripe             billing.Stripe
	signupSvc          *signup.Service
	signupOrigins      []string
	logger             *log.Logger
	mux                *http.ServeMux
	server             *http.Server

	// routeRecords logs every registered route (method, pattern, whether it
	// carries an Audit tag) so the audit-coverage guardrail test can assert that
	// every mutating portal route is audited. Populated by handle().
	routeRecords []routeRecord
}

// routeRecord is one registered route, captured for the audit-coverage guardrail.
type routeRecord struct {
	method  string // GET/POST/PUT/PATCH/DELETE, parsed from the pattern ("" if none)
	pattern string // the raw ServeMux pattern, e.g. "POST /api/v1/orgs/{id}/members/{userId}/promote"
	audited bool   // true iff registered with an Audit(...) tag in its chain
}

// handle registers a route on the mux and records it for the guardrail. Audited
// reflects whether the handler chain includes the Audit middleware; the
// registration helpers (mutate/read) set it intrinsically so the record can't
// drift from the actual middleware.
func (s *Server) handle(pattern string, audited bool, h http.Handler) {
	method, _, _ := strings.Cut(pattern, " ")
	s.routeRecords = append(s.routeRecords, routeRecord{method: method, pattern: pattern, audited: audited})
	s.mux.Handle(pattern, h)
}

// Config holds the server configuration.
type Config struct {
	Port               int
	Database           *db.DB
	KeyManager         *keys.Manager
	JWTGenerator       *jwt.Generator
	OIDCValidator      auth.TokenValidator
	OIDCAllowedDomains []string
	UserInfoCache      *auth.UserInfoCache
	Logger             *log.Logger

	// Cognito admin provisioning (optional). When CognitoAdmin is nil the
	// customer-admin routes that need it (members, SSO) are not registered.
	CognitoAdmin      cognito.Admin
	Encryptor         *keys.Encryptor
	CognitoUserPoolID string
	SSOLoginURL       sso.LoginURLFunc

	// Notifier alerts customer admins on allocation failures (A8). May be nil.
	Notifier portal.Notifier

	// Stripe backs the proposal/payment/seat-expansion routes (A7). May be nil.
	Stripe billing.Stripe

	// SignupAllowedOrigins is the CORS allowlist for the public signup endpoint
	// (the marketing-site origins that may POST from the browser). Empty ⇒ no CORS
	// headers (portal-only, since the portal calls signup server-side).
	SignupAllowedOrigins []string
}

// NewServer creates a new HTTP server.
func NewServer(cfg Config) (*Server, error) {
	if cfg.Database == nil {
		return nil, fmt.Errorf("database is required")
	}
	if cfg.KeyManager == nil {
		return nil, fmt.Errorf("key manager is required")
	}
	if cfg.JWTGenerator == nil {
		return nil, fmt.Errorf("JWT generator is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = log.New(os.Stdout, "[license-server] ", log.LstdFlags)
	}

	s := &Server{
		db:                 cfg.Database,
		keyMgr:             cfg.KeyManager,
		jwtGen:             cfg.JWTGenerator,
		oidcValidator:      cfg.OIDCValidator,
		oidcAllowedDomains: cfg.OIDCAllowedDomains,
		userInfoCache:      cfg.UserInfoCache,
		portalSvc:          portal.NewService(cfg.Database, cfg.JWTGenerator, cfg.Notifier, cfg.Logger),
		catalogSvc:         billing.NewCatalogService(cfg.Database),
		logger:             cfg.Logger,
		mux:                http.NewServeMux(),
	}

	// Customer-admin services require Cognito admin provisioning.
	if cfg.CognitoAdmin != nil {
		s.adminSvc = portal.NewAdminService(cfg.Database, cfg.CognitoAdmin, cfg.Logger)
		s.offboarder = offboarding.NewService(cfg.Database, cfg.CognitoAdmin, cfg.Logger)
		s.ssoSvc = sso.NewService(cfg.Database, cfg.CognitoAdmin, cfg.Encryptor, cfg.CognitoUserPoolID, cfg.SSOLoginURL, cfg.Logger)
	}

	// Billing services require Stripe.
	if cfg.Stripe != nil {
		s.stripe = cfg.Stripe
		s.proposalSvc = billing.NewProposalService(cfg.Database, cfg.Stripe, cfg.Logger)
		s.seatSvc = billing.NewSeatService(cfg.Database, cfg.Stripe, cfg.Logger)
	}

	// Public self-service signup needs Cognito (to provision the first admin) and
	// Stripe (to create the org's billing customer eagerly). Without both, the
	// route stays unregistered.
	s.signupOrigins = cfg.SignupAllowedOrigins
	if cfg.CognitoAdmin != nil && cfg.Stripe != nil {
		s.signupSvc = signup.NewService(cfg.Database, cfg.CognitoAdmin, cfg.Stripe, cfg.Logger)
	}

	s.routes()

	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      middleware.Chain(s.mux, middleware.Authentication(cfg.OIDCValidator, cfg.Logger), middleware.Logging(cfg.Logger)),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s, nil
}

// routes sets up all HTTP routes.
func (s *Server) routes() {
	// Public endpoints (no authentication required)
	s.mux.HandleFunc("/health", handlers.Health(s.db, s.logger, s.writeJSON, s.writeError))
	s.mux.HandleFunc("/api/v1/keys/public", handlers.GetPublicKey(s.db.DB, s.logger, s.writeJSON, s.writeError))

	// Public self-service signup (no auth — a new prospect has no identity yet).
	// Rate-limited and CORS-gated; registered without a method prefix so the CORS
	// middleware can answer the OPTIONS preflight and the handler enforces POST.
	if s.signupSvc != nil {
		s.handle("/api/v1/signup", false,
			middleware.Chain(
				handlers.Signup(s.signupSvc, s.logger, s.writeJSON, s.writeError),
				middleware.CORS(s.signupOrigins),
				middleware.SignupRateLimit(
					middleware.NewRateLimiter(signupPerEmailLimit, signupRateWindow),
					middleware.NewRateLimiter(signupPerIPLimit, signupRateWindow),
					s.logger,
				),
			))
	} else {
		s.logger.Println("Signup route disabled (needs Cognito admin + Stripe)")
	}

	// Protected endpoints — require a valid Cognito access token whose UserInfo
	// email is in the allowed domains list.
	requireAuth := s.requireUserInfo()

	// Token management
	s.mux.Handle("/api/v1/tokens",
		requireAuth(handlers.GenerateToken(s.db, s.jwtGen, s.logger, s.writeJSON, s.writeError)))
	s.mux.Handle("/api/v1/tokens/validate",
		requireAuth(handlers.ValidateToken(s.db, s.jwtGen, s.writeJSON, s.writeError)))

	// Key management
	s.mux.Handle("/api/v1/keys/rotate",
		requireAuth(handlers.RotateKey(s.db, s.keyMgr, s.logger, s.writeJSON, s.writeError)))
	s.mux.Handle("/api/v1/keys",
		requireAuth(handlers.ListKeys(s.db.DB, s.logger, s.writeJSON, s.writeError)))

	// Organization CRUD
	s.mux.Handle("/api/v1/organizations",
		requireAuth(s.routeOrganizations()))
	s.mux.Handle("/api/v1/organizations/get",
		requireAuth(handlers.GetOrganization(s.db.DB, s.logger, s.writeJSON, s.writeError)))
	s.mux.Handle("/api/v1/organizations/list",
		requireAuth(handlers.ListOrganizations(s.db.DB, s.logger, s.writeJSON, s.writeError)))
	s.mux.Handle("/api/v1/organizations/update",
		requireAuth(handlers.UpdateOrganization(s.db.DB, s.logger, s.writeJSON, s.writeError)))
	s.mux.Handle("/api/v1/organizations/delete",
		requireAuth(handlers.DeleteOrganization(s.db.DB, s.logger, s.writeJSON, s.writeError)))

	// Agreement CRUD
	s.mux.Handle("/api/v1/agreements",
		requireAuth(s.routeAgreements()))
	s.mux.Handle("/api/v1/agreements/get",
		requireAuth(handlers.GetAgreement(s.db.DB, s.logger, s.writeJSON, s.writeError)))
	s.mux.Handle("/api/v1/agreements/list",
		requireAuth(handlers.ListAgreements(s.db.DB, s.logger, s.writeJSON, s.writeError)))
	s.mux.Handle("/api/v1/agreements/update",
		requireAuth(handlers.UpdateAgreement(s.db.DB, s.logger, s.writeJSON, s.writeError)))
	s.mux.Handle("/api/v1/agreements/delete",
		requireAuth(handlers.DeleteAgreement(s.db.DB, s.logger, s.writeJSON, s.writeError)))

	// License tiers (Janus staff) — the catalog the agreement-create form draws
	// its tier options from.
	s.mux.Handle("GET /api/v1/licenses",
		requireAuth(handlers.ListLicenses(s.db.DB, s.logger, s.writeJSON, s.writeError)))

	// Pricing catalog (Janus staff).
	s.mux.Handle("GET /api/v1/billing/prices",
		requireAuth(handlers.CatalogList(s.catalogSvc, s.writeJSON, s.writeError)))
	s.mux.Handle("POST /api/v1/billing/prices",
		requireAuth(handlers.CatalogCreate(s.catalogSvc, s.writeJSON, s.writeError)))

	s.portalRoutes()
	s.billingRoutes()
}

// billingRoutes wires the Stripe-backed proposal/payment/seat-expansion routes.
// Registered only when Stripe is configured.
func (s *Server) billingRoutes() {
	if s.proposalSvc == nil {
		s.logger.Println("Billing routes disabled (no Stripe configured)")

		return
	}

	// Public, signature-verified Stripe webhook (not a portal route).
	s.handle("POST /api/v1/billing/webhook", false,
		handlers.StripeWebhook(s.stripe, s.proposalSvc, s.logger, s.writeJSON, s.writeError))

	// Janus-staff: make a priced offer on a proposal (staff route, not portal).
	s.handle("POST /api/v1/billing/proposals/{pid}/offer", false,
		s.requireUserInfo()(handlers.ProposalOffer(s.proposalSvc, s.writeJSON, s.writeError)))

	// Customer-admin proposal + seat routes (need the admin authz chain).
	if s.adminSvc == nil {
		return
	}

	s.adminRead("GET /api/v1/orgs/{id}/proposals", handlers.ProposalList(s.proposalSvc, s.writeJSON, s.writeError))
	s.adminRead("GET /api/v1/orgs/{id}/proposals/{pid}", handlers.ProposalGet(s.proposalSvc, s.writeJSON, s.writeError))
	s.adminMutate("POST /api/v1/orgs/{id}/proposals", models.AuditResourceProposal, models.AuditActionCreate, handlers.ProposalRequest(s.proposalSvc, s.writeJSON, s.writeError))
	s.adminMutate("POST /api/v1/orgs/{id}/proposals/{pid}/counter", models.AuditResourceProposal, "counter", handlers.ProposalCounter(s.proposalSvc, s.writeJSON, s.writeError))
	s.adminMutate("POST /api/v1/orgs/{id}/proposals/{pid}/accept", models.AuditResourceProposal, "accept", handlers.ProposalAccept(s.proposalSvc, s.writeJSON, s.writeError))
	s.adminMutate("POST /api/v1/orgs/{id}/proposals/{pid}/pay", models.AuditResourceProposal, "pay", handlers.ProposalPay(s.proposalSvc, s.writeJSON, s.writeError))

	s.adminRead("GET /api/v1/orgs/{id}/agreements/{aid}/seats/preview", handlers.SeatPreview(s.seatSvc, s.writeJSON, s.writeError))
	s.adminMutate("POST /api/v1/orgs/{id}/agreements/{aid}/seats", models.AuditResourceSeat, "add", handlers.SeatAdd(s.seatSvc, s.writeJSON, s.writeError))
}

// adminRead registers a read-only customer-admin route (authz chain, no audit).
func (s *Server) adminRead(pattern string, h http.HandlerFunc) {
	s.handle(pattern, false, s.adminChain(h))
}

// adminMutate registers a mutating customer-admin route: the authz chain plus the
// Audit middleware. Recording audited=true here is intrinsic — the same call both
// adds the tag and logs it — so the guardrail can't pass on a route that isn't
// actually audited.
func (s *Server) adminMutate(pattern, resource, action string, h http.HandlerFunc) {
	audit := middleware.Audit(middleware.AuditTag{Resource: resource, Action: action}, s.auditSink(), s.logger)
	s.handle(pattern, true, s.adminChain(h, audit))
}

// portalRoutes wires the end-user "/me" portal endpoints. These authorize by org
// membership (ResolveOrgUser) — NOT the staff email-domain allowlist — and audit
// every mutation.
func (s *Server) portalRoutes() {
	resolve := s.resolveOrgUserMW()

	// meRead/meMutate mirror adminRead/adminMutate but use the /me authz chain
	// (ResolveOrgUser, no group/scope check). meMutate ties audited=true to the
	// presence of the Audit middleware so the guardrail can't drift.
	meRead := func(pattern string, h http.HandlerFunc) {
		s.handle(pattern, false, middleware.Chain(h, resolve))
	}
	meMutate := func(pattern, resource, action string, h http.HandlerFunc) {
		audit := middleware.Audit(middleware.AuditTag{Resource: resource, Action: action}, s.auditSink(), s.logger)
		s.handle(pattern, true, middleware.Chain(h, resolve, audit))
	}

	meRead("GET /api/v1/me", handlers.MeProfile(s.portalSvc, s.oidcAllowedDomains, s.writeJSON, s.writeError))
	meRead("GET /api/v1/me/key", handlers.MeGetKey(s.portalSvc, s.writeJSON, s.writeError))
	meMutate("PUT /api/v1/me/key", models.AuditResourcePublicKey, models.AuditActionReplace, handlers.MeReplaceKey(s.portalSvc, s.logger, s.writeJSON, s.writeError))
	meMutate("POST /api/v1/me/license-requests", models.AuditResourceLicense, models.AuditActionAllocate, handlers.MeRequestLicense(s.portalSvc, s.writeJSON, s.writeError))
	meRead("GET /api/v1/me/license-requests", handlers.MeListRequests(s.portalSvc, s.writeJSON, s.writeError))
	meRead("GET /api/v1/me/license", handlers.MeGetLicense(s.portalSvc, s.writeJSON, s.writeError))

	s.adminRoutes()
}

// adminRoutes wires the customer-admin "/orgs/{id}/*" endpoints behind
// membership + customer_admin group + org-scope checks. Registered only when
// Cognito admin provisioning is configured.
func (s *Server) adminRoutes() {
	if s.adminSvc == nil {
		s.logger.Println("Customer-admin routes disabled (no Cognito admin configured)")

		return
	}

	// Members
	s.adminRead("GET /api/v1/orgs/{id}/members", handlers.AdminListMembers(s.adminSvc, s.writeJSON, s.writeError))
	s.adminMutate("POST /api/v1/orgs/{id}/members/{userId}/promote", models.AuditResourceMember, models.AuditActionPromote, handlers.AdminPromote(s.adminSvc, s.writeJSON, s.writeError))
	s.adminMutate("POST /api/v1/orgs/{id}/members/{userId}/demote", models.AuditResourceMember, "demote", handlers.AdminDemote(s.adminSvc, s.writeJSON, s.writeError))
	s.adminMutate("POST /api/v1/orgs/{id}/members/{userId}/offboard", models.AuditResourceMember, models.AuditActionOffboard, handlers.AdminOffboard(s.adminSvc, s.offboarder, s.writeJSON, s.writeError))

	// Activity log + agreements (read-only)
	s.adminRead("GET /api/v1/orgs/{id}/activity", handlers.AdminListActivity(s.adminSvc, s.writeJSON, s.writeError))
	s.adminRead("GET /api/v1/orgs/{id}/agreements", handlers.AdminListAgreements(s.adminSvc, s.writeJSON, s.writeError))

	// License requests
	s.adminRead("GET /api/v1/orgs/{id}/license-requests", handlers.AdminListRequests(s.adminSvc, s.writeJSON, s.writeError))
	s.adminMutate("POST /api/v1/orgs/{id}/license-requests/{reqId}/approve", models.AuditResourceLicense, models.AuditActionApprove, handlers.AdminApproveRequest(s.portalSvc, s.writeJSON, s.writeError))
	s.adminMutate("POST /api/v1/orgs/{id}/license-requests/{reqId}/reject", models.AuditResourceLicense, models.AuditActionReject, handlers.AdminRejectRequest(s.portalSvc, s.writeJSON, s.writeError))

	// Auto-acceptance rules
	s.adminRead("GET /api/v1/orgs/{id}/rules", handlers.AdminListRules(s.adminSvc, s.writeJSON, s.writeError))
	s.adminMutate("POST /api/v1/orgs/{id}/rules", "rule", models.AuditActionCreate, handlers.AdminCreateRule(s.adminSvc, s.writeJSON, s.writeError))
	s.adminMutate("PUT /api/v1/orgs/{id}/rules/{ruleId}", "rule", "update", handlers.AdminSetRuleEnabled(s.adminSvc, s.writeJSON, s.writeError))
	s.adminMutate("DELETE /api/v1/orgs/{id}/rules/{ruleId}", "rule", "delete", handlers.AdminDeleteRule(s.adminSvc, s.writeJSON, s.writeError))

	// SSO — create-once + read-only
	s.adminMutate("POST /api/v1/orgs/{id}/sso", models.AuditResourceSSOConfig, models.AuditActionCreate, handlers.AdminSSOSetup(s.ssoSvc, s.writeJSON, s.writeError))
	s.adminRead("GET /api/v1/orgs/{id}/sso", handlers.AdminSSOGet(s.ssoSvc, s.writeJSON, s.writeError))
}

// adminChain composes the customer-admin authorization chain: resolve OrgUser →
// require customer_admin → require the route's org scope → (audit) → handler.
func (s *Server) adminChain(h http.Handler, extra ...middleware.Middleware) http.Handler {
	mws := []middleware.Middleware{
		s.resolveOrgUserMW(),
		middleware.RequireCustomerAdmin(),
		middleware.RequireOrgScope(orgIDFromPath),
	}
	mws = append(mws, extra...)

	return middleware.Chain(h, mws...)
}

// orgIDFromPath extracts the {id} org wildcard from the route.
func orgIDFromPath(r *http.Request) (int64, bool) {
	v := r.PathValue("id")
	if v == "" {
		return 0, false
	}

	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, false
	}

	return n, true
}

// resolveOrgUserMW builds the ResolveOrgUser middleware backed by a DB lookup of
// the OrgUser by Cognito sub.
func (s *Server) resolveOrgUserMW() middleware.Middleware {
	return middleware.ResolveOrgUser(func(ctx context.Context, sub string) (*models.OrgUser, error) {
		var u models.OrgUser

		err := s.db.DB.WithContext(ctx).Where("cognito_sub = ?", sub).First(&u).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		if err != nil {
			return nil, err
		}

		return &u, nil
	}, s.logger)
}

func (s *Server) auditSink() middleware.AuditSink {
	return middleware.NewGormAuditSink(s.db)
}

// requireUserInfo returns the per-route UserInfo enforcement middleware bound
// to this server's OIDC validator, cache, and allowed domains.
func (s *Server) requireUserInfo() middleware.Middleware {
	var fetch middleware.UserInfoFunc
	if s.oidcValidator != nil {
		fetch = s.oidcValidator.FetchUserInfo
	}

	return middleware.RequireUserInfo(fetch, s.userInfoCache, s.oidcAllowedDomains, s.logger)
}

// routeOrganizations routes based on HTTP method for /api/v1/organizations.
func (s *Server) routeOrganizations() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handlers.CreateOrganization(s.db.DB, s.logger, s.writeJSON, s.writeError)(w, r)
		case http.MethodGet:
			handlers.ListOrganizations(s.db.DB, s.logger, s.writeJSON, s.writeError)(w, r)
		default:
			s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		}
	}
}

// routeAgreements routes based on HTTP method for /api/v1/agreements.
func (s *Server) routeAgreements() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handlers.CreateAgreement(s.db.DB, s.logger, s.writeJSON, s.writeError)(w, r)
		case http.MethodGet:
			handlers.ListAgreements(s.db.DB, s.logger, s.writeJSON, s.writeError)(w, r)
		default:
			s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		}
	}
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	s.logger.Printf("Starting server on %s", s.server.Addr)

	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Println("Shutting down server...")

	return s.server.Shutdown(ctx)
}

// writeJSON writes a JSON response with the given status code.
func (s *Server) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.logger.Printf("Error encoding JSON response: %v", err)
	}
}

// writeError writes a JSON error response.
func (s *Server) writeError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, map[string]string{
		"error": message,
	})
}
