package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"go.temporal.io/sdk/client"

	"payment-processing/shared/adapter"
	"payment-processing/shared/outbox"
	"payment-processing/shared/repository"
	"payment-processing/services/payment-api"
	"payment-processing/services/payment-api/internal/handlers"
	"payment-processing/services/payment-api/internal/middleware"
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

	// Create database connection (required in production)
	var db *repository.DB
	dbConfig, dbErr := loadDBConfig()
	if dbErr != nil {
		log.Fatalf("Database configuration error: %v", dbErr)
	}
	if dbConfig == nil {
		log.Fatalf("Database configuration is required - set DB_USER and DB_PASSWORD environment variables")
	}
	db, err = repository.NewDB(ctx, *dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := db.RunMigrations(*dbConfig); err != nil {
		log.Fatalf("Failed to run database migrations: %v", err)
	}

	// Get repositories
	repos := db.Repositories()

	// Start outbox consumer if enabled
	var outboxConsumer *outbox.Consumer
	if getEnv("OUTBOX_CONSUMER_ENABLED", "true") == "true" {
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

	// Build HTTP handler with cleanup channels
	handler, cleanupFn := buildRouter(temporalClient, db)

	// Create HTTP server for graceful shutdown
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: handler,
	}

	// Start HTTP server in goroutine
	go func() {
		log.Printf("Starting Payment API server on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Stop cleanup routines
	cleanupFn()

	// Give outstanding requests 30 seconds to complete
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
}

// buildRouter creates the chi router with all routes and middleware
// Returns the router and a cleanup function to stop background routines
func buildRouter(temporalClient client.Client, db *repository.DB) (*chi.Mux, func()) {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.CorrelationID)

	// Create middleware stores
	idempotencyStore := middleware.NewIdempotencyStore(nil)
	rateLimiter := middleware.NewRateLimiter(nil)

	// Start background cleanup routines and capture stop channels
	idempotencyStop := idempotencyStore.StartCleanupRoutine(5 * time.Minute)
	rateLimitStop := rateLimiter.StartCleanupRoutine(1 * time.Minute)

	// Cleanup function to stop background routines
	cleanupFn := func() {
		close(idempotencyStop)
		close(rateLimitStop)
	}

	// Create metrics collector
	metricsCollector := handlers.NewMetricsCollector()

	// Create health handler
	dbChecker := &dbPingChecker{pool: db.Pool}
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
		// M4: Limit webhook body size to 1MB
		r.Use(middleware.WebhookBodyLimit())
		r.Post("/stripe", webhookHandler.Stripe)
		r.Post("/adyen", webhookHandler.Adyen)
		r.Post("/paypal", webhookHandler.PayPal)
	})

	// API v1 routes (require auth, rate limiting, idempotency)
	r.Route("/api/v1", func(r chi.Router) {
		// M4: Limit API body size to 64KB
		r.Use(middleware.APIBodyLimit())
		r.Use(middleware.Auth(nil))
		r.Use(middleware.RateLimit(rateLimiter))
		r.Use(middleware.Idempotency(idempotencyStore))

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

		// Audit log endpoints
		auditHandler := handlers.NewAuditHandler(repos.AuditLog)
		r.Route("/audit", func(r chi.Router) {
			r.Get("/entity/{type}/{id}", auditHandler.GetByEntity)
			r.Get("/actor/{type}/{id}", auditHandler.GetByActor)
			r.Get("/action/{action}", auditHandler.GetByAction)
			r.Get("/range", auditHandler.GetByTimeRange)
		})
	})

	// Legacy endpoints for backwards compatibility
	legacyServer := server.New(temporalClient)
	r.Post("/payment", legacyServer.HandlePayment)
	r.Get("/payment/status", legacyServer.HandlePaymentStatus)

	return r, cleanupFn
}

// loadDBConfig loads database configuration from environment
// Returns error if required configuration is missing
func loadDBConfig() (*repository.Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL != "" {
		// Parse DATABASE_URL format would go here
	}

	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "localhost"
	}

	user := os.Getenv("DB_USER")
	if user == "" {
		return nil, nil // No database config provided
	}

	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		return nil, fmt.Errorf("DB_PASSWORD is required when DB_USER is set")
	}

	cfg := repository.DefaultConfig()
	cfg.Host = host
	cfg.User = user
	cfg.Password = password

	if dbName := os.Getenv("DB_NAME"); dbName != "" {
		cfg.Database = dbName
	}
	if sslMode := os.Getenv("DB_SSLMODE"); sslMode != "" {
		cfg.SSLMode = sslMode
	}

	return &cfg, nil
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
