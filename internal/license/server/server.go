package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/pharmalytica/janus/internal/license/auth"
	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/jwt"
	"github.com/pharmalytica/janus/internal/license/keys"
	"github.com/pharmalytica/janus/internal/license/server/handlers"
	"github.com/pharmalytica/janus/internal/license/server/middleware"
)

// Server represents the HTTP server for the license service.
type Server struct {
	db                 *db.DB
	keyMgr             *keys.Manager
	jwtGen             *jwt.Generator
	oidcValidator      *auth.OIDCValidator
	oidcAllowedDomains []string
	logger             *log.Logger
	mux                *http.ServeMux
	server             *http.Server
}

// Config holds the server configuration.
type Config struct {
	Port               int
	Database           *db.DB
	KeyManager         *keys.Manager
	JWTGenerator       *jwt.Generator
	OIDCValidator      *auth.OIDCValidator
	OIDCAllowedDomains []string
	Logger             *log.Logger
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
		logger:             cfg.Logger,
		mux:                http.NewServeMux(),
	}

	s.routes()

	s.server = &http.Server{
		Addr: fmt.Sprintf(":%d", cfg.Port),
		// Apply global middleware: authentication first, then logging
		Handler:      middleware.Chain(s.mux, middleware.Authentication(cfg.JWTGenerator, cfg.OIDCValidator, cfg.Logger), middleware.Logging(cfg.Logger)),
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

	// Protected endpoints (authentication required)

	// Token management
	s.mux.Handle("/api/v1/tokens",
		middleware.RequireAuth(s.oidcAllowedDomains)(http.HandlerFunc(handlers.GenerateToken(s.db, s.jwtGen, s.logger, s.writeJSON, s.writeError))))
	s.mux.Handle("/api/v1/tokens/validate",
		middleware.RequireAuth(s.oidcAllowedDomains)(http.HandlerFunc(handlers.ValidateToken(s.jwtGen, s.writeJSON, s.writeError))))

	// Key management
	s.mux.Handle("/api/v1/keys/rotate",
		middleware.RequireAuth(s.oidcAllowedDomains)(http.HandlerFunc(handlers.RotateKey(s.db, s.keyMgr, s.logger, s.writeJSON, s.writeError))))
	s.mux.Handle("/api/v1/keys",
		middleware.RequireAuth(s.oidcAllowedDomains)(http.HandlerFunc(handlers.ListKeys(s.db.DB, s.logger, s.writeJSON, s.writeError))))

	// Organization CRUD
	s.mux.Handle("/api/v1/organizations",
		middleware.RequireAuth(s.oidcAllowedDomains)(s.routeOrganizations()))
	s.mux.Handle("/api/v1/organizations/get",
		middleware.RequireAuth(s.oidcAllowedDomains)(http.HandlerFunc(handlers.GetOrganization(s.db.DB, s.logger, s.writeJSON, s.writeError))))
	s.mux.Handle("/api/v1/organizations/list",
		middleware.RequireAuth(s.oidcAllowedDomains)(http.HandlerFunc(handlers.ListOrganizations(s.db.DB, s.logger, s.writeJSON, s.writeError))))
	s.mux.Handle("/api/v1/organizations/update",
		middleware.RequireAuth(s.oidcAllowedDomains)(http.HandlerFunc(handlers.UpdateOrganization(s.db.DB, s.logger, s.writeJSON, s.writeError))))
	s.mux.Handle("/api/v1/organizations/delete",
		middleware.RequireAuth(s.oidcAllowedDomains)(http.HandlerFunc(handlers.DeleteOrganization(s.db.DB, s.logger, s.writeJSON, s.writeError))))

	// Agreement CRUD
	s.mux.Handle("/api/v1/agreements",
		middleware.RequireAuth(s.oidcAllowedDomains)(s.routeAgreements()))
	s.mux.Handle("/api/v1/agreements/get",
		middleware.RequireAuth(s.oidcAllowedDomains)(http.HandlerFunc(handlers.GetAgreement(s.db.DB, s.logger, s.writeJSON, s.writeError))))
	s.mux.Handle("/api/v1/agreements/list",
		middleware.RequireAuth(s.oidcAllowedDomains)(http.HandlerFunc(handlers.ListAgreements(s.db.DB, s.logger, s.writeJSON, s.writeError))))
	s.mux.Handle("/api/v1/agreements/update",
		middleware.RequireAuth(s.oidcAllowedDomains)(http.HandlerFunc(handlers.UpdateAgreement(s.db.DB, s.logger, s.writeJSON, s.writeError))))
	s.mux.Handle("/api/v1/agreements/delete",
		middleware.RequireAuth(s.oidcAllowedDomains)(http.HandlerFunc(handlers.DeleteAgreement(s.db.DB, s.logger, s.writeJSON, s.writeError))))
}

// routeOrganizations routes based on HTTP method for /api/v1/organizations
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

// routeAgreements routes based on HTTP method for /api/v1/agreements
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