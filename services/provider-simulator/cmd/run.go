package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"provider-simulator/internal/client"
	"provider-simulator/internal/config"
	"provider-simulator/internal/generator"
	"provider-simulator/internal/signer"
)

var (
	scenarioTarget string
	stopOnError    bool
)

var runCmd = &cobra.Command{
	Use:   "run <scenario-file>",
	Short: "Run a scenario file",
	Long: `Run a YAML scenario file containing multiple webhook events.

Examples:
  # Run authorization and capture scenario
  webhook-simulator run scenarios/auth_capture.yaml

  # Run with custom target
  webhook-simulator run scenarios/decline_soft.yaml --target http://localhost:9090

  # Stop on first error
  webhook-simulator run scenarios/invalid.yaml --stop-on-error`,
	Args: cobra.ExactArgs(1),
	RunE: runScenario,
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringVar(&scenarioTarget, "target", "", "Override target URL from scenario file")
	runCmd.Flags().BoolVar(&stopOnError, "stop-on-error", false, "Stop execution on first error")
}

func runScenario(cmd *cobra.Command, args []string) error {
	scenarioPath := args[0]

	// Load scenario file
	sf, err := config.LoadScenarioFile(scenarioPath)
	if err != nil {
		return fmt.Errorf("loading scenario file: %w", err)
	}

	// Validate scenario
	if err := sf.Validate(); err != nil {
		return fmt.Errorf("validating scenario: %w", err)
	}

	// Determine target URL
	target := targetURL
	if scenarioTarget != "" {
		target = scenarioTarget
	} else if sf.Target != "" {
		target = sf.Target
	}

	fmt.Printf("Running scenario: %s\n", sf.Name)
	fmt.Printf("Target: %s\n", target)
	fmt.Println()

	// Initialize registries
	genRegistry := generator.NewRegistry()
	signRegistry := signer.NewRegistry()
	httpClient := client.NewClient(target, verbose)

	// Track results
	var passed, failed int
	ctx := context.Background()

	// Run each scenario
	for _, scenario := range sf.Scenarios {
		fmt.Printf("=== Scenario: %s (%s) ===\n", scenario.Name, scenario.Provider)

		gen, err := genRegistry.Get(scenario.Provider)
		if err != nil {
			fmt.Printf("  ERROR: %v\n", err)
			failed++
			if stopOnError {
				break
			}
			continue
		}

		sign, err := signRegistry.Get(scenario.Provider)
		if err != nil {
			fmt.Printf("  ERROR: %v\n", err)
			failed++
			if stopOnError {
				break
			}
			continue
		}

		// Run each step
		scenarioFailed := false
		for i, step := range scenario.Steps {
			// Handle delay
			if delay, err := step.GetDelay(); err == nil && delay > 0 {
				time.Sleep(delay)
			}

			// Build sign options from step
			signOpts := signer.SignOpts{}
			if step.SignOpts != nil {
				signOpts.SkipSignature = step.SignOpts.SkipSignature
				signOpts.InvalidKey = step.SignOpts.InvalidKey
				if offset, err := step.SignOpts.GetTimestampOffset(); err == nil {
					signOpts.TimestampOffset = offset
				}
			}

			// Generate payload
			payload, err := gen.Generate(step.Event, step.Data)
			if err != nil {
				fmt.Printf("  Step %d: FAIL - generate error: %v\n", i+1, err)
				scenarioFailed = true
				if stopOnError {
					break
				}
				continue
			}

			// Sign payload
			headers, err := sign.Sign(payload, secretKey, signOpts)
			if err != nil {
				fmt.Printf("  Step %d: FAIL - sign error: %v\n", i+1, err)
				scenarioFailed = true
				if stopOnError {
					break
				}
				continue
			}

			// Send request
			resp, err := httpClient.Send(ctx, gen.Endpoint(), payload, headers)
			if err != nil {
				fmt.Printf("  Step %d: FAIL - send error: %v\n", i+1, err)
				scenarioFailed = true
				if stopOnError {
					break
				}
				continue
			}

			// Check expected status
			expectedStatus := step.ExpectStatus
			if expectedStatus == 0 {
				expectedStatus = 200
			}

			if resp.StatusCode == expectedStatus {
				fmt.Printf("  Step %d: PASS - %s (%d, %s)\n", i+1, step.Event, resp.StatusCode, resp.Duration)
			} else {
				fmt.Printf("  Step %d: FAIL - %s (expected %d, got %d)\n", i+1, step.Event, expectedStatus, resp.StatusCode)
				if verbose {
					fmt.Printf("    Response: %s\n", string(resp.Body))
				}
				scenarioFailed = true
				if stopOnError {
					break
				}
			}
		}

		if scenarioFailed {
			failed++
		} else {
			passed++
		}

		if stopOnError && scenarioFailed {
			break
		}

		fmt.Println()
	}

	// Print summary
	fmt.Println("=== Summary ===")
	fmt.Printf("Passed: %d\n", passed)
	fmt.Printf("Failed: %d\n", failed)
	fmt.Printf("Total:  %d\n", passed+failed)

	if failed > 0 {
		os.Exit(1)
	}

	return nil
}
