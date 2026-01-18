package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"webhook-simulator/internal/client"
	"webhook-simulator/internal/generator"
	"webhook-simulator/internal/signer"
)

var (
	dryRun        bool
	paymentID     string
	chargeID      string
	amount        int
	currency      string
	declineCode   string
	skipSignature bool
	invalidKey    bool
)

var sendCmd = &cobra.Command{
	Use:   "send <provider> <event>",
	Short: "Send a single webhook event",
	Long: `Send a single webhook event to the target server.

Examples:
  # Send a successful Stripe payment intent
  webhook-simulator send stripe payment_intent.succeeded --amount 10000

  # Send a failed payment with decline code
  webhook-simulator send stripe payment_intent.payment_failed --decline-code insufficient_funds

  # Dry run to see payload without sending
  webhook-simulator send stripe charge.captured --dry-run

  # Send Adyen authorization
  webhook-simulator send adyen AUTHORISATION --amount 5000

  # Send PayPal capture
  webhook-simulator send paypal PAYMENT.CAPTURE.COMPLETED --amount 7500`,
	Args: cobra.ExactArgs(2),
	RunE: runSend,
}

func init() {
	rootCmd.AddCommand(sendCmd)

	sendCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show payload without sending")
	sendCmd.Flags().StringVar(&paymentID, "payment-id", "", "Payment/transaction ID")
	sendCmd.Flags().StringVar(&chargeID, "charge-id", "", "Charge ID (Stripe)")
	sendCmd.Flags().IntVar(&amount, "amount", 10000, "Amount in smallest currency unit (e.g., cents)")
	sendCmd.Flags().StringVar(&currency, "currency", "usd", "Currency code")
	sendCmd.Flags().StringVar(&declineCode, "decline-code", "", "Decline code for failed payments")
	sendCmd.Flags().BoolVar(&skipSignature, "skip-signature", false, "Omit signature header")
	sendCmd.Flags().BoolVar(&invalidKey, "invalid-key", false, "Use invalid signing key")
}

func runSend(cmd *cobra.Command, args []string) error {
	provider := strings.ToLower(args[0])
	event := args[1]

	// Get generator
	genRegistry := generator.NewRegistry()
	gen, err := genRegistry.Get(provider)
	if err != nil {
		return fmt.Errorf("unknown provider %q: %w\nSupported providers: %s",
			provider, err, strings.Join(genRegistry.Providers(), ", "))
	}

	// Build data map from flags
	data := make(map[string]any)
	data["amount"] = amount
	data["currency"] = strings.ToUpper(currency)

	if paymentID != "" {
		data["payment_id"] = paymentID
		data["psp_reference"] = paymentID
		data["authorization_id"] = paymentID
		data["capture_id"] = paymentID
	}
	if chargeID != "" {
		data["charge_id"] = chargeID
	}
	if declineCode != "" {
		data["decline_code"] = declineCode
		data["reason"] = declineCode
	}

	// Generate payload
	payload, err := gen.Generate(event, data)
	if err != nil {
		return fmt.Errorf("generating payload: %w", err)
	}

	// Get signer
	signRegistry := signer.NewRegistry()
	sign, err := signRegistry.Get(provider)
	if err != nil {
		return fmt.Errorf("getting signer: %w", err)
	}

	// Sign payload
	signOpts := signer.SignOpts{
		SkipSignature: skipSignature,
		InvalidKey:    invalidKey,
	}
	headers, err := sign.Sign(payload, secretKey, signOpts)
	if err != nil {
		return fmt.Errorf("signing payload: %w", err)
	}

	// Build request
	req := &client.Request{
		Provider: provider,
		Endpoint: gen.Endpoint(),
		Payload:  payload,
		Headers:  headers,
	}

	if dryRun {
		fmt.Println("=== DRY RUN ===")
		fmt.Printf("Provider: %s\n", provider)
		fmt.Printf("Event: %s\n", event)
		fmt.Printf("Endpoint: %s%s\n", targetURL, gen.Endpoint())
		fmt.Println()
		fmt.Println("Headers:")
		for key, values := range headers {
			for _, value := range values {
				fmt.Printf("  %s: %s\n", key, value)
			}
		}
		fmt.Println()
		fmt.Println("Payload:")
		var prettyJSON map[string]any
		json.Unmarshal(payload, &prettyJSON)
		pretty, _ := json.MarshalIndent(prettyJSON, "", "  ")
		fmt.Println(string(pretty))
		return nil
	}

	// Send request
	c := client.NewClient(targetURL, verbose)
	ctx := context.Background()
	resp, err := c.SendRequest(ctx, req)
	if err != nil {
		return fmt.Errorf("sending webhook: %w", err)
	}

	// Print result
	fmt.Printf("Response Status: %d\n", resp.StatusCode)
	fmt.Printf("Response Time: %s\n", resp.Duration)
	if verbose && len(resp.Body) > 0 {
		fmt.Printf("Response Body: %s\n", string(resp.Body))
	}

	if resp.StatusCode >= 400 {
		os.Exit(1)
	}

	return nil
}
