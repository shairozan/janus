package commands

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/pharmalytica/janus/internal/license/auth"
	"github.com/pharmalytica/janus/internal/license/billing"
	"github.com/pharmalytica/janus/internal/license/cognito"
	"github.com/pharmalytica/janus/internal/license/db"
	"github.com/pharmalytica/janus/internal/license/jwt"
	"github.com/pharmalytica/janus/internal/license/keys"
	"github.com/pharmalytica/janus/internal/license/notify"
	"github.com/pharmalytica/janus/internal/license/portal"
	"github.com/pharmalytica/janus/internal/license/server"
	"github.com/pharmalytica/janus/internal/license/sso"
)

// ssoLoginURLBuilder returns a function that builds the Cognito hosted-UI deep
// link sending users straight to their IdP. If the hosted domain is unset it
// returns an empty URL (set later via support tooling).
func ssoLoginURLBuilder(hostedDomain, appClientID string) sso.LoginURLFunc {
	return func(providerName string) string {
		if hostedDomain == "" {
			return ""
		}

		q := url.Values{}
		q.Set("identity_provider", providerName)
		q.Set("client_id", appClientID)
		q.Set("response_type", "code")
		q.Set("scope", "openid email profile")

		return "https://" + hostedDomain + "/oauth2/authorize?" + q.Encode()
	}
}

// ServeCommand creates the serve command.
func ServeCommand() *cobra.Command {
	var cfg *ServeConfig

	cmd := &cobra.Command{ //nolint:gosec // G101: the postgres:// example in Long is illustrative help text, not a credential
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

			log.Printf("Starting license server on %s:%d...", cfg.Host, cfg.Port)

			// Connect to database (migrations run automatically)
			database, err := db.Connect(cfg.DatabaseURL, true)
			if err != nil {
				return fmt.Errorf("failed to connect to database: %w", err)
			}
			defer database.Close()

			log.Println("Database connected and migrations applied")

			// Initialize key manager
			keyManager, err := keys.NewManager(database, []byte(cfg.EncryptionKey))
			if err != nil {
				return fmt.Errorf("failed to initialize key manager: %w", err)
			}

			// Initialize JWT generator
			jwtGenerator := jwt.NewGenerator(keyManager, database)

			// Initialize OIDC validator (optional). A ValidatorSet routes tokens
			// to a per-issuer validator; today one issuer is registered, but the
			// seam supports multiple Cognito user pools without a rewrite.
			var oidcValidator auth.TokenValidator
			if cfg.HasOIDC() {
				log.Println("Initializing OIDC authentication...")
				validatorSet := auth.NewValidatorSet()
				if regErr := validatorSet.Register(context.Background(), &auth.OIDCConfig{
					IssuerURL: cfg.OIDCIssuer,
					ClientID:  cfg.OIDCClientID,
				}); regErr != nil {
					return fmt.Errorf("failed to initialize OIDC validator: %w", regErr)
				}
				oidcValidator = validatorSet
				log.Printf("OIDC authentication enabled (issuer: %s)", cfg.OIDCIssuer)
			} else {
				log.Println("OIDC authentication disabled (no issuer/client-id provided)")
			}

			// Initialize Redis UserInfo cache (optional)
			var userInfoCache *auth.UserInfoCache
			if cfg.HasRedis() {
				userInfoCache, err = auth.NewUserInfoCache(cfg.RedisURL)
				if err != nil {
					return fmt.Errorf("failed to connect to Redis: %w", err)
				}
				defer func() {
					if closeErr := userInfoCache.Close(); closeErr != nil {
						log.Printf("warning: failed to close Redis connection: %v", closeErr)
					}
				}()
				log.Printf("UserInfo cache enabled (redis: %s)", cfg.RedisURL)
			} else {
				log.Println("UserInfo cache disabled (no redis-url provided)")
			}

			// Initialize Cognito admin provisioning (optional — enables the
			// customer-admin routes: members, SSO).
			var cognitoAdmin cognito.Admin
			var ssoEncryptor *keys.Encryptor
			if cfg.HasCognitoAdmin() {
				cognitoAdmin, err = cognito.NewClient(context.Background(), cfg.AWSRegion, cfg.CognitoUserPoolID, cognito.Credentials{
					AccessKeyID:     cfg.AWSAccessKeyID,
					SecretAccessKey: cfg.AWSSecretAccessKey,
					RoleARN:         cfg.AWSRoleARN,
				})
				if err != nil {
					return fmt.Errorf("failed to initialize Cognito admin client: %w", err)
				}

				ssoEncryptor, err = keys.NewEncryptor([]byte(cfg.EncryptionKey))
				if err != nil {
					return fmt.Errorf("failed to initialize SSO secret encryptor: %w", err)
				}

				// Read-only admin routes need no AWS creds (DB only). Warn when the
				// provisioning routes (promote/demote/offboard/SSO) can't actually
				// reach Cognito so a misconfig surfaces at startup, not mid-request.
				switch {
				case cfg.AWSRoleARN == "":
					log.Printf("Cognito admin provisioning enabled (pool: %s, region: %s); no aws-role-arn — provisioning ops will use ambient credentials", cfg.CognitoUserPoolID, cfg.AWSRegion)
				case cfg.AWSAccessKeyID == "":
					log.Printf("Cognito admin provisioning enabled (pool: %s, region: %s, role: %s); no aws-access-key-id — AssumeRole will use the default credential chain", cfg.CognitoUserPoolID, cfg.AWSRegion, cfg.AWSRoleARN)
				default:
					log.Printf("Cognito admin provisioning enabled (pool: %s, region: %s, assuming role: %s)", cfg.CognitoUserPoolID, cfg.AWSRegion, cfg.AWSRoleARN)
				}
			} else {
				log.Println("Cognito admin provisioning disabled (no aws-region/cognito-user-pool-id)")
			}

			// Initialize Mailgun notifications (optional).
			var notifier portal.Notifier
			if cfg.HasMailgun() {
				sender := notify.NewMailgunSender(cfg.MailgunDomain, cfg.MailgunAPIKey, cfg.MailgunRegion, cfg.MailgunFrom)
				notifier = notify.NewAdminNotifier(database, sender, log.Default())
				log.Printf("Mailgun notifications enabled (domain: %s)", cfg.MailgunDomain)
			} else {
				log.Println("Mailgun notifications disabled (no mailgun config)")
			}

			// Initialize Stripe billing (optional).
			var stripeClient billing.Stripe
			if cfg.HasStripe() {
				stripeClient = billing.NewStripeClient(cfg.StripeAPIKey, cfg.StripeWebhookSecret)
				log.Println("Stripe billing enabled")
			} else {
				log.Println("Stripe billing disabled (no stripe-api-key)")
			}

			// Create HTTP server
			srv, err := server.NewServer(server.Config{
				Port:                 cfg.Port,
				Database:             database,
				KeyManager:           keyManager,
				JWTGenerator:         jwtGenerator,
				OIDCValidator:        oidcValidator,
				OIDCAllowedDomains:   cfg.OIDCAllowedDomains,
				UserInfoCache:        userInfoCache,
				CognitoAdmin:         cognitoAdmin,
				Encryptor:            ssoEncryptor,
				CognitoUserPoolID:    cfg.CognitoUserPoolID,
				SSOLoginURL:          ssoLoginURLBuilder(cfg.CognitoHostedDomain, cfg.OIDCClientID),
				Notifier:             notifier,
				Stripe:               stripeClient,
				SignupAllowedOrigins: cfg.SignupAllowedOrigins,
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

			log.Printf("License server listening on %s:%d", cfg.Host, cfg.Port)
			log.Println("Endpoints:")
			log.Println("  GET  /health                    - Health check")
			log.Println("  POST /api/v1/tokens             - Generate JWT token")
			log.Println("  POST /api/v1/tokens/validate    - Validate JWT token")
			log.Println("  POST /api/v1/keys/rotate        - Rotate signing keys")

			// Wait for shutdown signal or server error
			select {
			case err := <-serverErrors:
				return fmt.Errorf("server error: %w", err)
			case sig := <-shutdown:
				log.Printf("%v signal received, shutting down...", sig)

				// Give outstanding requests 5 seconds to complete
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				if err := srv.Shutdown(ctx); err != nil {
					return fmt.Errorf("graceful shutdown failed: %w", err)
				}

				log.Println("Server stopped")
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
		"OIDC provider issuer URL (e.g., https://cognito-idp.<region>.amazonaws.com/<pool-id>) (env: LICENSING_OIDC_ISSUER)")
	c.Flags().String("oidc-client-id", "",
		"OIDC client ID for token validation (env: LICENSING_OIDC_CLIENT_ID)")
	c.Flags().StringSlice("oidc-allowed-domains", []string{"januspk.com"},
		"Comma-separated list of allowed email domains (env: LICENSING_OIDC_ALLOWED_DOMAINS)")
	c.Flags().String("redis-url", "",
		"Redis URL for UserInfo cache, e.g. redis://host:6379/0 (env: LICENSING_REDIS_URL)")
	c.Flags().String("aws-region", "us-east-2",
		"AWS region for the Cognito SDK client (env: LICENSING_AWS_REGION)")
	c.Flags().String("aws-access-key-id", "",
		"AWS access key id of the IAM user that assumes the Cognito role; GKE has no instance profile (env: LICENSING_AWS_ACCESS_KEY_ID)")
	c.Flags().String("aws-secret-access-key", "",
		"AWS secret access key for the IAM user above (env: LICENSING_AWS_SECRET_ACCESS_KEY)")
	c.Flags().String("aws-role-arn", "",
		"ARN of the role to assume for Cognito admin operations (env: LICENSING_AWS_ROLE_ARN)")
	c.Flags().String("cognito-user-pool-id", "",
		"Cognito user pool id for admin provisioning (env: LICENSING_COGNITO_USER_POOL_ID)")
	c.Flags().String("cognito-hosted-domain", "",
		"Cognito hosted-UI domain for SSO login deep-links (env: LICENSING_COGNITO_HOSTED_DOMAIN)")
	c.Flags().String("mailgun-domain", "", "Mailgun sending domain (env: LICENSING_MAILGUN_DOMAIN)")
	c.Flags().String("mailgun-api-key", "", "Mailgun API key (env: LICENSING_MAILGUN_API_KEY)")
	c.Flags().String("mailgun-region", "us", "Mailgun region: us or eu (env: LICENSING_MAILGUN_REGION)")
	c.Flags().String("mailgun-from", "", "Notification sender address (env: LICENSING_MAILGUN_FROM)")
	c.Flags().String("stripe-api-key", "", "Stripe secret API key (env: LICENSING_STRIPE_API_KEY)")
	c.Flags().String("stripe-webhook-secret", "", "Stripe webhook signing secret (env: LICENSING_STRIPE_WEBHOOK_SECRET)")
	c.Flags().StringSlice("signup-allowed-origins", nil,
		"CORS allowlist of marketing-site origins for the public signup endpoint (env: LICENSING_SIGNUP_ALLOWED_ORIGINS)")

	_ = viper.BindPFlags(c.Flags())
}
