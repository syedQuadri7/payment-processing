package workflow

import (
	"fmt"

	"go.temporal.io/sdk/temporal"
)

// Error types for Temporal retry control
// These errors are used to signal non-retryable failures to Temporal

// HardDeclineError indicates a non-retryable decline
type HardDeclineError struct {
	Code    string
	Message string
}

func (e *HardDeclineError) Error() string {
	return fmt.Sprintf("hard decline: %s - %s", e.Code, e.Message)
}

// NewHardDeclineError creates a non-retryable error for Temporal
func NewHardDeclineError(code, message string) error {
	return temporal.NewApplicationError(
		fmt.Sprintf("hard decline: %s - %s", code, message),
		"HardDeclineError",
	)
}

// FraudError indicates a fraud-related decline
type FraudError struct {
	Code    string
	Message string
}

func (e *FraudError) Error() string {
	return fmt.Sprintf("fraud: %s - %s", e.Code, e.Message)
}

// NewFraudError creates a non-retryable fraud error for Temporal
func NewFraudError(code, message string) error {
	return temporal.NewApplicationError(
		fmt.Sprintf("fraud: %s - %s", code, message),
		"FraudError",
	)
}

// ValidationError indicates invalid input
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: %s - %s", e.Field, e.Message)
}

// NewValidationError creates a non-retryable validation error
func NewValidationError(field, message string) error {
	return temporal.NewApplicationError(
		fmt.Sprintf("validation error: %s - %s", field, message),
		"ValidationError",
	)
}
