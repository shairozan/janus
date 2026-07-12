package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/pharmalytica/janus/internal/license/auth"
	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/jwt"
	"github.com/pharmalytica/janus/internal/license/keys"
	"github.com/pharmalytica/janus/internal/license/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// ServeCommand creates the serve command.
func ServeCommand() *cobra.Command {
	var cfg *ServeConfig

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the license server HTTP API",
		Long: `Start the license server HTTP API for token validation and management.

The server automatically applies pending database migrations on startup.

Example:
  license-server serve --database-url="postgres://user:pass@localhost/janus_license" --port=8443
`,
		PreRunE: func(c *cobra.Command, args []string) error {
			// Configure Viper for environment variable support
			viper.SetEnvPrefix("LICENSING")
			viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
			viper.AutomaticEnv()

			// Unmarshal configuration from viper
			var err error
			cfg, err = UnmarshalServeConfig()
			if err != nil {
				return err
			}

			return nil
		},
		RunE: func(c *cobra.Command, args []string) error {
			// Validate configuration
			if err := cfg.Validate(); err != nil {
				return err
			}

			fmt.Printf("Starting license server on %s:%d...\n", cfg.Host, cfg.Port)

			// Connect to database (migrations run automatically)
			database, err := db.Connect(cfg.DatabaseURL, true)
			if err != nil {
				return fmt.Errorf("failed to connect to database: %w", err)
			}
			defer database.Close()

			fmt.Println("✓ Database connected and migrations applied")

			// Initialize key manager
			keyManager, err := keys.NewManager(database, []byte(cfg.EncryptionKey))
			if err != nil {
				return fmt.Errorf("failed to initialize key manager: %w", err)
			}

			// Initialize JWT generator
			jwtGenerator := jwt.NewGenerator(keyManager, database)

			// Initialize OIDC validator (optional)
			var oidcValidator *auth.OIDCValidator
			if cfg.HasOIDC() {
				fmt.Println("Initializing OIDC authentication...")
				oidcConfig := &auth.OIDCConfig{
					IssuerURL: cfg.OIDCIssuer,
					ClientID:  cfg.OIDCClientID,
				}

				ctx := context.Background()
				oidcValidator, err = auth.NewOIDCValidator(ctx, oidcConfig)
				if err != nil {
					return fmt.Errorf("failed to initialize OIDC validator: %w", err)
				}
				fmt.Printf("✓ OIDC authentication enabled (issuer: %s)\n", cfg.OIDCIssuer)
			} else {
				fmt.Println("OIDC authentication disabled (no issuer/client-id provided)")
			}

			// Create HTTP server
			srv, err := server.NewServer(server.Config{
				Port:               cfg.Port,
				Database:           database,
				KeyManager:         keyManager,
				JWTGenerator:       jwtGenerator,
				OIDCValidator:      oidcValidator,
				OIDCAllowedDomains: cfg.OIDCAllowedDomains,
			})
			if err != nil {
				return fmt.Errorf("failed to create server: %w", err)
			}

			// Set up graceful shutdown
			shutdown := make(chan os.Signal, 1)
			signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

			// Start server in a goroutine
			serverErrors := make(chan error, 1)
			go func() {
				serverErrors <- srv.Start()
			}()

			fmt.Printf("✓ License server listening on %s:%d\n", cfg.Host, cfg.Port)
			fmt.Println("\nEndpoints:")
			fmt.Println("  GET  /health                    - Health check")
			fmt.Println("  POST /api/v1/tokens             - Generate JWT token")
			fmt.Println("  POST /api/v1/tokens/validate    - Validate JWT token")
			fmt.Println("  POST /api/v1/keys/rotate        - Rotate signing keys")

			// Wait for shutdown signal or server error
			select {
			case err := <-serverErrors:
				return fmt.Errorf("server error: %w", err)
			case sig := <-shutdown:
				fmt.Printf("\n%v signal received, shutting down...\n", sig)

				// Give outstanding requests 5 seconds to complete
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				if err := srv.Shutdown(ctx); err != nil {
					return fmt.Errorf("graceful shutdown failed: %w", err)
				}

				fmt.Println("✓ Server stopped")
			}

			return nil
		},
	}

	attributes(cmd)

	return cmd
}

func attributes(c *cobra.Command) {
	// Define flags (bound to viper in PreRunE)
	c.Flags().String("database-url", "",
		"PostgreSQL connection string (env: LICENSING_DATABASE_URL)")
	c.Flags().Int("port", 8443,
		"Port to listen on (env: LICENSING_PORT)")
	c.Flags().String("host", "0.0.0.0",
		"Host to bind to (env: LICENSING_HOST)")
	c.Flags().String("encryption-key", "",
		"32-byte encryption key for private key storage (env: LICENSING_ENCRYPTION_KEY)")
	c.Flags().String("oidc-issuer", "",
		"OIDC provider issuer URL (e.g., https://dev-xxx.us.auth0.com) (env: LICENSING_OIDC_ISSUER)")
	c.Flags().String("oidc-client-id", "",
		"OIDC client ID for token validation (env: LICENSING_OIDC_CLIENT_ID)")
	c.Flags().StringSlice("oidc-allowed-domains", []string{"pharmalytica.io"},
		"Comma-separated list of allowed email domains for OIDC authentication (env: LICENSING_OIDC_ALLOWED_DOMAINS)")

	_ = viper.BindPFlags(c.Flags())
}
