package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	targetURL string
	secretKey string
	verbose   bool
)

var rootCmd = &cobra.Command{
	Use:   "webhook-simulator",
	Short: "Webhook simulation tool for payment processing",
	Long: `A CLI tool that generates and sends provider-specific webhook payloads
to test the payment processing service's adapter layer.

Supports Stripe, Adyen, and PayPal with proper signature generation
and configurable test scenarios.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&targetURL, "target", "t", "http://localhost:8080", "Target URL for webhook delivery")
	rootCmd.PersistentFlags().StringVarP(&secretKey, "secret", "s", "whsec_test_secret", "Webhook signing secret")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
}
