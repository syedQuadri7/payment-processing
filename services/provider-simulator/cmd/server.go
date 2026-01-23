package cmd

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"provider-simulator/internal/server"
	"provider-simulator/internal/state"
)

var (
	serverPort     int
	webhookTarget  string
	webhookSecret  string
	webhookDelayMs int
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Run the provider simulator HTTP server",
	Long: `Start the provider simulator as an HTTP server that simulates
Stripe, Adyen, and PayPal payment APIs with stateful payment flows.

The server provides:
  - Provider APIs that mimic real payment providers
  - Admin API for state management and debugging
  - Automatic webhook delivery to your payment service

Examples:
  # Start server on default port 9000
  provider-simulator server

  # Start with custom port and webhook target
  provider-simulator server --port 9001 --webhook-target http://localhost:8080

  # Start with custom webhook delay for realistic timing
  provider-simulator server --webhook-delay 1000`,
	RunE: runServer,
}

func init() {
	rootCmd.AddCommand(serverCmd)

	serverCmd.Flags().IntVarP(&serverPort, "port", "p", 9000, "Server port")
	serverCmd.Flags().StringVar(&webhookTarget, "webhook-target", "http://localhost:8080", "Target URL for webhook delivery")
	serverCmd.Flags().StringVar(&webhookSecret, "webhook-secret", "whsec_test_secret", "Secret for webhook signatures")
	serverCmd.Flags().IntVar(&webhookDelayMs, "webhook-delay", 0, "Delay in ms before sending webhooks (0 = immediate)")
}

func runServer(cmd *cobra.Command, args []string) error {
	// Setup logger
	logLevel := slog.LevelInfo
	if verbose {
		logLevel = slog.LevelDebug
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))

	// Create store
	store := state.NewMemoryStore()

	// Create server config
	cfg := server.Config{
		Port:           serverPort,
		WebhookTarget:  webhookTarget,
		WebhookSecret:  webhookSecret,
		WebhookDelayMs: webhookDelayMs,
	}

	// Create and start server
	srv := server.New(cfg, store, logger)

	// Handle shutdown signals
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		logger.Info("received shutdown signal")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Stop(ctx); err != nil {
			logger.Error("shutdown error", "error", err)
		}
	}()

	return srv.Start()
}
