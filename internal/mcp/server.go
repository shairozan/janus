package mcp

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Config is the runtime configuration for the MCP server. It is a plain value
// built by the GUI layer from config.MCPConfig and handed in; the mcp package
// imports no config types.
type Config struct {
	Host         string
	Port         int
	AuthToken    string
	AllowExecute bool
	Version      string
}

// Server is the embedded MCP server. It binds an auth-wrapped Streamable HTTP
// handler to a loopback listener. It is safe for concurrent use and its Start
// and Stop methods are idempotent.
type Server struct {
	cfg    Config
	bridge Bridge

	mu      sync.Mutex
	httpSrv *http.Server
	ln      net.Listener
}

// NewServer constructs a Server from cfg and bridge. The bridge and a non-empty
// auth token are mandatory. The host defaults to loopback if unset. A zero port
// means "let the OS assign an ephemeral port" (the actual port is then available
// via Addr); callers that want a fixed port supply it — the config layer defaults
// it to 8731 when MCP is enabled.
func NewServer(cfg Config, bridge Bridge) (*Server, error) {
	if bridge == nil {
		return nil, errors.New("mcp: bridge is required")
	}

	if cfg.AuthToken == "" {
		return nil, errors.New("mcp: auth token is required")
	}

	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}

	return &Server{cfg: cfg, bridge: bridge}, nil
}

// Start binds the loopback listener and serves the MCP handler in a background
// goroutine. It is idempotent (a no-op if already running) and self-stops when
// ctx is cancelled. A bind failure (e.g. port conflict) is returned to the
// caller.
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.httpSrv != nil {
		return nil // already running
	}

	srv := newMCPServer(s.cfg, s.bridge)

	handler := mcpsdk.NewStreamableHTTPHandler(
		func(*http.Request) *mcpsdk.Server { return srv }, nil)

	mux := http.NewServeMux()
	mux.Handle("/mcp", authMiddleware(s.cfg.AuthToken, handler))

	ln, err := net.Listen("tcp", net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port)))
	if err != nil {
		return fmt.Errorf("mcp: listen on %s:%d: %w", s.cfg.Host, s.cfg.Port, err)
	}

	httpSrv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	s.ln = ln
	s.httpSrv = httpSrv

	go func() {
		if serveErr := httpSrv.Serve(ln); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			// Serve only returns a non-ErrServerClosed error on a genuine fault;
			// there is no caller to hand it to here, so surface it on the log.
			log.Printf("mcp: server stopped: %v", serveErr)
		}
	}()

	// Self-stop when the app-lifetime context is cancelled.
	go func() {
		<-ctx.Done()
		_ = s.Stop() //nolint:contextcheck // shutdown uses its own bounded context
	}()

	return nil
}

// Stop gracefully shuts the server down. It is idempotent.
func (s *Server) Stop() error {
	s.mu.Lock()
	srv := s.httpSrv
	s.httpSrv = nil
	s.ln = nil
	s.mu.Unlock()

	if srv == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return srv.Shutdown(ctx)
}

// Running reports whether the server is currently serving.
func (s *Server) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.httpSrv != nil
}

// Addr returns the actual listen address (useful when Port is 0 in tests), or
// "" if the server is not running.
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ln == nil {
		return ""
	}

	return s.ln.Addr().String()
}
