// Package server provides the HTTP server for the provider simulator.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"provider-simulator/internal/state"
	"provider-simulator/internal/webhook"
)

// Config holds server configuration.
type Config struct {
	Port           int
	WebhookTarget  string
	WebhookSecret  string
	WebhookDelayMs int
}

// Server is the provider simulator HTTP server.
type Server struct {
	config        Config
	store         state.Store
	httpServer    *http.Server
	webhookEngine *webhook.Engine
	logger        *slog.Logger
}

// New creates a new simulator server.
func New(cfg Config, store state.Store, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}

	// Create webhook engine
	webhookCfg := webhook.DefaultConfig()
	webhookCfg.TargetURL = cfg.WebhookTarget
	webhookCfg.Secret = cfg.WebhookSecret
	webhookCfg.DelayMs = cfg.WebhookDelayMs

	webhookEngine := webhook.NewEngine(webhookCfg, store, logger)

	s := &Server{
		config:        cfg,
		store:         store,
		webhookEngine: webhookEngine,
		logger:        logger,
	}

	router := s.setupRouter()

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

// Start begins listening for requests.
func (s *Server) Start() error {
	s.logger.Info("starting provider simulator",
		"port", s.config.Port,
		"webhook_target", s.config.WebhookTarget)

	// Start webhook engine
	s.webhookEngine.Start()

	if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}

// Stop gracefully shuts down the server.
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("shutting down provider simulator")

	// Stop webhook engine
	s.webhookEngine.Stop()

	return s.httpServer.Shutdown(ctx)
}

// WebhookEngine returns the webhook engine for external use.
func (s *Server) WebhookEngine() *webhook.Engine {
	return s.webhookEngine
}

// SetupRouter creates and returns the HTTP router for testing.
func (s *Server) SetupRouter() http.Handler {
	return s.setupRouter()
}

// Store returns the server's state store.
func (s *Server) Store() state.Store {
	return s.store
}

// Config returns the server's configuration.
func (s *Server) Config() Config {
	return s.config
}
