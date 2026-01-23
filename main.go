package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"go.temporal.io/sdk/client"

	"payment-processing/internal/adapter"
	"payment-processing/internal/domain"
	"payment-processing/internal/outbox"
	"payment-processing/internal/repository"
	"payment-processing/server"
	"payment-processing/server/handlers"
	"payment-processing/server/middleware"
	"payment-processing/worker"
)

const version = "0.1.0"

func main() {
	ctx := context.Background()

	// Load configuration
	temporalHost := getEnv("TEMPORAL_HOST", "localhost:7233")
	port := getEnv("PORT", "8080")

	// Create Temporal client
	temporalClient, err := client.Dial(client.Options{
		HostPort: temporalHost,
	})
	if err != nil {
		log.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer temporalClient.Close()

	// Create database connection (optional - skip if not configured)
	var db *repository.DB
	dbConfig := loadDBConfig()
	if dbConfig != nil {
		db, err = repository.NewDB(ctx, *dbConfig)
		if err != nil {
			log.Printf("Warning: Failed to connect to database: %v", err)
			log.Printf("Running without database - some features will be unavailable")
		} else {
			defer db.Close()

			// Run migrations
			if err := db.RunMigrations(*dbConfig); err != nil {
				log.Printf("Warning: Failed to run migrations: %v", err)
			}
		}
	}

	// Get repositories if database is available
	var repos *domain.Repositories
	if db != nil {
		repos = db.Repositories()
	}

	// Start worker in a goroutine with repository dependencies
	go func() {
		if err := worker.StartWorkerWithDependencies(temporalClient, repos); err != nil {
			log.Fatalf("Worker failed: %v", err)
		}
	}()

	// Start outbox consumer if database is available and enabled
	var outboxConsumer *outbox.Consumer
	if db != nil && getEnv("OUTBOX_CONSUMER_ENABLED", "true") == "true" {
		consumerConfig := outbox.DefaultConsumerConfig()
		outboxConsumer = outbox.NewConsumer(
			repos.Outbox,
			outbox.LoggingHandler(),
			consumerConfig,
		)
		if err := outboxConsumer.Start(ctx); err != nil {
			log.Printf("Warning: Failed to start outbox consumer: %v", err)
		} else {
			defer outboxConsumer.Stop()
		}
	}

	// Build HTTP handler
	handler := buildRouter(temporalClient, db)

	// Start HTTP server
	log.Printf("Starting HTTP server on :%s", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatalf("HTTP server failed: %v", err)
	}
}

// buildRouter creates the chi router with all routes and middleware
func buildRouter(temporalClient client.Client, db *repository.DB) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.CorrelationID)

	// Create middleware stores
	idempotencyStore := middleware.NewIdempotencyStore(nil)
	rateLimiter := middleware.NewRateLimiter(nil)

	// Start background cleanup routines
	idempotencyStore.StartCleanupRoutine(5 * time.Minute)
	rateLimiter.StartCleanupRoutine(1 * time.Minute)

	// Create metrics collector
	metricsCollector := handlers.NewMetricsCollector()

	// Create health handler
	var dbChecker handlers.DatabaseHealthChecker
	if db != nil {
		dbChecker = &dbPingChecker{pool: db.Pool}
	}
	healthHandler := handlers.NewHealthHandler(version, dbChecker, temporalClient)
	metricsHandler := handlers.NewMetricsHandler(metricsCollector)

	// Health and metrics endpoints (no auth required)
	r.Group(func(r chi.Router) {
		r.Get("/health", healthHandler.Health)
		r.Get("/health/live", healthHandler.Live)
		r.Get("/health/ready", healthHandler.Ready)
		r.Get("/metrics", metricsHandler.Metrics)
	})

	// Webhook endpoints (signature verification in handler, no API auth)
	if db != nil {
		repos := db.Repositories()

		adapterCfg := adapter.AdapterConfig{
			StripeWebhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"),
			AdyenHMACKey:        os.Getenv("ADYEN_HMAC_KEY"),
			PayPalClientID:      os.Getenv("PAYPAL_CLIENT_ID"),
			PayPalClientSecret:  os.Getenv("PAYPAL_CLIENT_SECRET"),
			PayPalWebhookID:     os.Getenv("PAYPAL_WEBHOOK_ID"),
			PayPalAPIURL:        os.Getenv("PAYPAL_API_URL"),
		}
		adapterRegistry := adapter.NewAdapterRegistry(adapterCfg)

		webhookHandler := handlers.NewWebhookHandler(
			adapterRegistry,
			repos.PaymentIntents,
			repos.ProcessedEvents,
			temporalClient,
		)

		r.Route("/webhooks", func(r chi.Router) {
			r.Post("/stripe", webhookHandler.Stripe)
			r.Post("/adyen", webhookHandler.Adyen)
			r.Post("/paypal", webhookHandler.PayPal)
		})
	}

	// API v1 routes (require auth, rate limiting, idempotency)
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.Auth(nil))
		r.Use(middleware.RateLimit(rateLimiter))
		r.Use(middleware.Idempotency(idempotencyStore))

		if db != nil {
			repos := db.Repositories()
			intentHandler := handlers.NewIntentHandler(
				repos.PaymentIntents,
				repos.PaymentAttempts,
				repos.AuthorizationHolds,
				temporalClient,
			)

			r.Route("/intents", func(r chi.Router) {
				r.Post("/", intentHandler.Create)
				r.Get("/{id}", intentHandler.Get)
				r.Put("/{id}/method", intentHandler.AttachMethod)
				r.Post("/{id}/capture", intentHandler.Capture)
				r.Post("/{id}/cancel", intentHandler.Cancel)
				r.Get("/{id}/attempts", intentHandler.GetAttempts)
				r.Get("/{id}/hold", intentHandler.GetHold)
			})

			// Audit log endpoints (FR-AUD-05)
			auditHandler := handlers.NewAuditHandler(repos.AuditLog)
			r.Route("/audit", func(r chi.Router) {
				r.Get("/entity/{type}/{id}", auditHandler.GetByEntity)
				r.Get("/actor/{type}/{id}", auditHandler.GetByActor)
				r.Get("/action/{action}", auditHandler.GetByAction)
				r.Get("/range", auditHandler.GetByTimeRange)
			})
		}
	})

	// Legacy endpoints for backwards compatibility
	legacyServer := server.New(temporalClient)
	r.Post("/payment", legacyServer.HandlePayment)
	r.Get("/payment/status", legacyServer.HandlePaymentStatus)

	return r
}

// loadDBConfig loads database configuration from environment
func loadDBConfig() *repository.Config {
	// Check if DATABASE_URL is set
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL != "" {
		// Parse DATABASE_URL format would go here
		// For now, use defaults
	}

	// Check if any DB config is provided
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "localhost"
	}

	user := os.Getenv("DB_USER")
	if user == "" {
		// No user configured, assume no database
		return nil
	}

	cfg := repository.DefaultConfig()
	cfg.Host = host
	if user != "" {
		cfg.User = user
	}
	if pass := os.Getenv("DB_PASSWORD"); pass != "" {
		cfg.Password = pass
	}
	if dbName := os.Getenv("DB_NAME"); dbName != "" {
		cfg.Database = dbName
	}
	if sslMode := os.Getenv("DB_SSLMODE"); sslMode != "" {
		cfg.SSLMode = sslMode
	}

	return &cfg
}

// getEnv gets environment variable with a default value
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// dbPingChecker implements handlers.DatabaseHealthChecker
type dbPingChecker struct {
	pool interface {
		Ping(ctx context.Context) error
	}
}

func (c *dbPingChecker) Ping(ctx context.Context) error {
	return c.pool.Ping(ctx)
}
