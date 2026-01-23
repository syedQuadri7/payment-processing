package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"payment-processing/internal/domain"
)

// PaymentIntentWorkflow orchestrates the full lifecycle of a payment intent
// State machine: CREATED -> REQUIRES_AUTH -> AUTHORIZED -> CAPTURED/VOIDED
// With RECOVERING state for retry handling
func PaymentIntentWorkflow(ctx workflow.Context, input PaymentWorkflowInput) (*PaymentWorkflowResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting PaymentIntentWorkflow", "payment_intent_id", input.PaymentIntentID)

	// Initialize workflow state
	state := &PaymentState{
		PaymentIntentID: input.PaymentIntentID,
		Status:          domain.PaymentIntentStatusCreated,
		Provider:        input.Provider,
		Amount:          input.Amount,
		Currency:        input.Currency,
		CaptureMethod:   input.CaptureMethod,
		CurrentAttempt:  0,
		Attempts:        []AttemptRecord{},
		CreatedAt:       workflow.Now(ctx),
		UpdatedAt:       workflow.Now(ctx),
	}

	// Set up query handlers
	if err := setupQueryHandlers(ctx, state); err != nil {
		return nil, fmt.Errorf("failed to setup query handlers: %w", err)
	}

	// Set up signal channels
	webhookEventCh := workflow.GetSignalChannel(ctx, SignalWebhookEvent)
	cancelCh := workflow.GetSignalChannel(ctx, SignalCancelPayment)
	updatePaymentMethodCh := workflow.GetSignalChannel(ctx, SignalUpdatePaymentMethod)

	// Activity options for provider calls
	providerActivityOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumAttempts:    3,
			NonRetryableErrorTypes: []string{
				"HardDeclineError",
				"FraudError",
				"ValidationError",
			},
		},
	}
	providerCtx := workflow.WithActivityOptions(ctx, providerActivityOpts)

	// Activity options for database operations
	dbActivityOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    500 * time.Millisecond,
			BackoffCoefficient: 2.0,
			MaximumAttempts:    5,
		},
	}
	dbCtx := workflow.WithActivityOptions(ctx, dbActivityOpts)

	// Get activities struct
	var activities *Activities

	// Track payment method ID - may be updated via signal
	var currentPaymentMethodID string
	if input.PaymentMethodID != nil {
		currentPaymentMethodID = *input.PaymentMethodID
	}

	// Check if payment method is present
	if currentPaymentMethodID == "" {
		state.Status = domain.PaymentIntentStatusRequiresMethod
		state.UpdatedAt = workflow.Now(ctx)

		// Persist initial state
		if err := persistState(dbCtx, activities, state); err != nil {
			logger.Warn("Failed to persist initial state", "error", err)
		}

		// Wait for payment method to be attached
		for state.Status == domain.PaymentIntentStatusRequiresMethod {
			selector := workflow.NewSelector(ctx)

			selector.AddReceive(updatePaymentMethodCh, func(c workflow.ReceiveChannel, _ bool) {
				var paymentMethodID string
				c.Receive(ctx, &paymentMethodID)
				logger.Info("Payment method attached", "payment_method_id", paymentMethodID)
				currentPaymentMethodID = paymentMethodID
				state.Status = domain.PaymentIntentStatusRequiresAuth
				state.UpdatedAt = workflow.Now(ctx)
			})

			selector.AddReceive(cancelCh, func(c workflow.ReceiveChannel, _ bool) {
				var reason string
				c.Receive(ctx, &reason)
				logger.Info("Payment cancelled", "reason", reason)
				state.Status = domain.PaymentIntentStatusCancelled
				state.UpdatedAt = workflow.Now(ctx)
			})

			selector.Select(ctx)

			if state.Status == domain.PaymentIntentStatusCancelled {
				return buildResult(ctx, state), nil
			}
		}
	} else {
		state.Status = domain.PaymentIntentStatusRequiresAuth
	}

	// Persist state transition
	if err := persistState(dbCtx, activities, state); err != nil {
		logger.Warn("Failed to persist state", "error", err)
	}

	// Authorization loop with retry support
authLoop:
	for !state.IsTerminal() {
		state.CurrentAttempt++
		attemptStartTime := workflow.Now(ctx)

		attempt := AttemptRecord{
			AttemptNumber: state.CurrentAttempt,
			Status:        domain.AttemptStatusProcessing,
			StartedAt:     attemptStartTime,
		}
		state.Attempts = append(state.Attempts, attempt)

		logger.Info("Starting authorization attempt",
			"attempt", state.CurrentAttempt,
			"payment_intent_id", input.PaymentIntentID,
		)

		// Execute authorization
		var authResult AuthorizePaymentResult
		err := workflow.ExecuteActivity(providerCtx, activities.AuthorizePayment, AuthorizePaymentInput{
			PaymentIntentID: input.PaymentIntentID,
			AttemptNumber:   state.CurrentAttempt,
			Provider:        input.Provider,
			Amount:          input.Amount,
			Currency:        input.Currency,
			PaymentMethodID: currentPaymentMethodID,
			IdempotencyKey:  fmt.Sprintf("%s-auth-%d", input.IdempotencyKey, state.CurrentAttempt),
		}).Get(ctx, &authResult)

		completedAt := workflow.Now(ctx)
		state.Attempts[len(state.Attempts)-1].CompletedAt = &completedAt

		if err != nil {
			// Activity failed (network error, etc.)
			state.Attempts[len(state.Attempts)-1].Status = domain.AttemptStatusFailed
			errMsg := err.Error()
			state.LastErrorMessage = &errMsg

			// Check if this is a non-retryable error
			var appErr *temporal.ApplicationError
			if temporal.IsApplicationError(err) {
				err = err.(*temporal.ApplicationError).Unwrap()
			}
			if appErr != nil && (appErr.Type() == "HardDeclineError" || appErr.Type() == "FraudError") {
				state.Status = domain.PaymentIntentStatusFailed
				break authLoop
			}

			// Transient error - will be retried by activity retry policy
			continue
		}

		if authResult.Success {
			// Authorization succeeded
			state.Attempts[len(state.Attempts)-1].Status = domain.AttemptStatusSucceeded
			state.Status = domain.PaymentIntentStatusAuthorized
			state.ProviderPaymentID = &authResult.ProviderPaymentID
			state.AuthorizationCode = &authResult.AuthorizationCode
			state.AuthExpiresAt = authResult.ExpiresAt
			state.UpdatedAt = workflow.Now(ctx)

			logger.Info("Authorization succeeded",
				"provider_payment_id", authResult.ProviderPaymentID,
				"authorization_code", authResult.AuthorizationCode,
			)
			break authLoop
		}

		// Authorization failed with decline
		state.Attempts[len(state.Attempts)-1].Status = domain.AttemptStatusFailed
		state.Attempts[len(state.Attempts)-1].DeclineCode = authResult.DeclineCode
		state.Attempts[len(state.Attempts)-1].DeclineType = authResult.DeclineType
		state.Attempts[len(state.Attempts)-1].ErrorMessage = authResult.ErrorMessage

		state.LastDeclineCode = authResult.DeclineCode
		state.LastDeclineType = authResult.DeclineType
		state.LastErrorMessage = authResult.ErrorMessage

		// Check if we can retry
		if state.CanRetry() {
			state.Status = domain.PaymentIntentStatusRecovering
			state.UpdatedAt = workflow.Now(ctx)

			// Persist recovering state
			if err := persistState(dbCtx, activities, state); err != nil {
				logger.Warn("Failed to persist recovering state", "error", err)
			}

			// Wait for retry interval while listening for signals
			retryDelay := state.NextRetryDelay()
			logger.Info("Waiting before retry",
				"delay", retryDelay,
				"attempt", state.CurrentAttempt,
			)

			cancelled := waitWithSignals(ctx, retryDelay, webhookEventCh, cancelCh, state)
			if cancelled || state.IsTerminal() {
				break authLoop
			}

			// Continue to next attempt
			continue
		}

		// No more retries - fail the payment
		state.Status = domain.PaymentIntentStatusFailed
		state.UpdatedAt = workflow.Now(ctx)
		break authLoop
	}

	// Persist state after authorization phase
	if err := persistState(dbCtx, activities, state); err != nil {
		logger.Warn("Failed to persist state after auth", "error", err)
	}

	// Handle post-authorization states
	if state.Status == domain.PaymentIntentStatusAuthorized {
		if input.CaptureMethod == domain.CaptureMethodAutomatic {
			// Auto-capture
			state = handleCapture(ctx, providerCtx, dbCtx, activities, state, input)
		} else {
			// Manual capture - wait for signal or timeout
			state = waitForManualCapture(ctx, providerCtx, dbCtx, activities, state, input, webhookEventCh, cancelCh)
		}
	}

	// Persist final state
	if err := persistState(dbCtx, activities, state); err != nil {
		logger.Warn("Failed to persist final state", "error", err)
	}

	// Write outbox event for final state
	if err := writeOutboxEvent(dbCtx, activities, state, input); err != nil {
		logger.Warn("Failed to write outbox event", "error", err)
	}

	logger.Info("PaymentIntentWorkflow completed",
		"payment_intent_id", input.PaymentIntentID,
		"status", state.Status,
	)

	return buildResult(ctx, state), nil
}

// setupQueryHandlers registers query handlers for the workflow
func setupQueryHandlers(ctx workflow.Context, state *PaymentState) error {
	// Query for current status
	if err := workflow.SetQueryHandler(ctx, QueryPaymentStatus, func() (domain.PaymentIntentStatus, error) {
		return state.Status, nil
	}); err != nil {
		return err
	}

	// Query for full state
	if err := workflow.SetQueryHandler(ctx, QueryPaymentState, func() (*PaymentState, error) {
		return state, nil
	}); err != nil {
		return err
	}

	return nil
}

// waitWithSignals waits for a duration while remaining responsive to signals
func waitWithSignals(ctx workflow.Context, duration time.Duration, webhookCh, cancelCh workflow.ReceiveChannel, state *PaymentState) bool {
	timer := workflow.NewTimer(ctx, duration)
	selector := workflow.NewSelector(ctx)
	cancelled := false

	selector.AddFuture(timer, func(f workflow.Future) {
		// Timer completed - continue with retry
	})

	selector.AddReceive(webhookCh, func(c workflow.ReceiveChannel, _ bool) {
		var event domain.CanonicalEvent
		c.Receive(ctx, &event)
		handleWebhookEvent(ctx, &event, state)
	})

	selector.AddReceive(cancelCh, func(c workflow.ReceiveChannel, _ bool) {
		var reason string
		c.Receive(ctx, &reason)
		state.Status = domain.PaymentIntentStatusCancelled
		state.UpdatedAt = workflow.Now(ctx)
		cancelled = true
	})

	selector.Select(ctx)
	return cancelled
}

// handleWebhookEvent processes incoming webhook events
func handleWebhookEvent(ctx workflow.Context, event *domain.CanonicalEvent, state *PaymentState) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Processing webhook event",
		"event_type", event.Type,
		"event_id", event.ID,
	)

	switch event.Type {
	case domain.EventAuthorizationSucceeded:
		if state.Status == domain.PaymentIntentStatusRequiresAuth ||
			state.Status == domain.PaymentIntentStatusRecovering {
			state.Status = domain.PaymentIntentStatusAuthorized
			state.ProviderPaymentID = &event.ProviderPaymentID
			state.AuthorizationCode = event.AuthorizationCode
			state.AuthExpiresAt = event.ExpiresAt
		}

	case domain.EventAuthorizationFailed:
		state.LastDeclineCode = event.DeclineCode
		state.LastDeclineType = event.DeclineType
		state.LastErrorMessage = event.ErrorMessage

	case domain.EventCaptureSucceeded:
		if state.Status == domain.PaymentIntentStatusAuthorized {
			state.Status = domain.PaymentIntentStatusCaptured
			state.CapturedAmount = event.CapturedAmount
			capturedAt := workflow.Now(ctx)
			state.CapturedAt = &capturedAt
		}

	case domain.EventCaptureFailed:
		state.LastDeclineCode = event.DeclineCode
		state.LastDeclineType = event.DeclineType
		state.LastErrorMessage = event.ErrorMessage

	case domain.EventVoidSucceeded:
		if state.Status == domain.PaymentIntentStatusAuthorized {
			state.Status = domain.PaymentIntentStatusVoided
		}

	case domain.EventAuthorizationExpired:
		if state.Status == domain.PaymentIntentStatusAuthorized {
			state.Status = domain.PaymentIntentStatusFailed
			msg := "authorization expired"
			state.LastErrorMessage = &msg
		}
	}

	state.UpdatedAt = workflow.Now(ctx)
}

// handleCapture performs automatic capture
func handleCapture(ctx workflow.Context, providerCtx, dbCtx workflow.Context, activities *Activities, state *PaymentState, input PaymentWorkflowInput) *PaymentState {
	logger := workflow.GetLogger(ctx)
	logger.Info("Auto-capturing payment", "payment_intent_id", input.PaymentIntentID)

	var captureResult CapturePaymentResult
	err := workflow.ExecuteActivity(providerCtx, activities.CapturePayment, CapturePaymentInput{
		PaymentIntentID:   input.PaymentIntentID,
		ProviderPaymentID: *state.ProviderPaymentID,
		Provider:          input.Provider,
		Amount:            input.Amount,
		Currency:          input.Currency,
		IdempotencyKey:    fmt.Sprintf("%s-capture", input.IdempotencyKey),
	}).Get(ctx, &captureResult)

	if err != nil {
		errMsg := err.Error()
		state.LastErrorMessage = &errMsg
		logger.Error("Capture activity failed", "error", err)
		return state
	}

	if captureResult.Success {
		state.Status = domain.PaymentIntentStatusCaptured
		state.CapturedAmount = &captureResult.CapturedAmount
		capturedAt := workflow.Now(ctx)
		state.CapturedAt = &capturedAt
	} else {
		state.LastDeclineCode = captureResult.DeclineCode
		state.LastDeclineType = captureResult.DeclineType
		state.LastErrorMessage = captureResult.ErrorMessage
	}

	state.UpdatedAt = workflow.Now(ctx)
	return state
}

// waitForManualCapture waits for capture signal or authorization expiry
func waitForManualCapture(ctx workflow.Context, providerCtx, dbCtx workflow.Context, activities *Activities, state *PaymentState, input PaymentWorkflowInput, webhookCh, cancelCh workflow.ReceiveChannel) *PaymentState {
	logger := workflow.GetLogger(ctx)

	// Calculate time until auth expiry (default 7 days if not set)
	var expiryDuration time.Duration
	if state.AuthExpiresAt != nil {
		expiryDuration = state.AuthExpiresAt.Sub(workflow.Now(ctx))
	} else {
		expiryDuration = 7 * 24 * time.Hour
	}

	logger.Info("Waiting for manual capture",
		"payment_intent_id", input.PaymentIntentID,
		"expiry_duration", expiryDuration,
	)

	timer := workflow.NewTimer(ctx, expiryDuration)

	for state.Status == domain.PaymentIntentStatusAuthorized {
		selector := workflow.NewSelector(ctx)

		selector.AddFuture(timer, func(f workflow.Future) {
			// Authorization expired
			logger.Info("Authorization expired", "payment_intent_id", input.PaymentIntentID)
			state.Status = domain.PaymentIntentStatusFailed
			msg := "authorization expired"
			state.LastErrorMessage = &msg
			state.UpdatedAt = workflow.Now(ctx)
		})

		selector.AddReceive(webhookCh, func(c workflow.ReceiveChannel, _ bool) {
			var event domain.CanonicalEvent
			c.Receive(ctx, &event)
			handleWebhookEvent(ctx, &event, state)
		})

		selector.AddReceive(cancelCh, func(c workflow.ReceiveChannel, _ bool) {
			var reason string
			c.Receive(ctx, &reason)
			logger.Info("Cancellation requested, voiding authorization",
				"payment_intent_id", input.PaymentIntentID,
				"reason", reason,
			)

			// Void the authorization
			var voidResult VoidPaymentResult
			err := workflow.ExecuteActivity(providerCtx, activities.VoidPayment, VoidPaymentInput{
				PaymentIntentID:   input.PaymentIntentID,
				ProviderPaymentID: *state.ProviderPaymentID,
				Provider:          input.Provider,
				Reason:            reason,
				IdempotencyKey:    fmt.Sprintf("%s-void", input.IdempotencyKey),
			}).Get(ctx, &voidResult)

			if err != nil || !voidResult.Success {
				errMsg := "void failed"
				if err != nil {
					errMsg = err.Error()
				} else if voidResult.ErrorMessage != nil {
					errMsg = *voidResult.ErrorMessage
				}
				state.LastErrorMessage = &errMsg
				// Even if void fails, we should mark as cancelled
			}

			state.Status = domain.PaymentIntentStatusVoided
			state.UpdatedAt = workflow.Now(ctx)
		})

		selector.Select(ctx)
	}

	return state
}

// persistState persists the current workflow state to the database
func persistState(ctx workflow.Context, activities *Activities, state *PaymentState) error {
	return workflow.ExecuteActivity(ctx, activities.PersistPaymentState, PersistPaymentStateInput{
		PaymentIntentID:   state.PaymentIntentID,
		Status:            state.Status,
		ProviderPaymentID: state.ProviderPaymentID,
	}).Get(ctx, nil)
}

// writeOutboxEvent writes the final state event to the outbox
func writeOutboxEvent(ctx workflow.Context, activities *Activities, state *PaymentState, input PaymentWorkflowInput) error {
	eventType := fmt.Sprintf("payment_intent.%s", state.Status)

	return workflow.ExecuteActivity(ctx, activities.WriteOutboxEvent, WriteOutboxEventInput{
		EventType:      eventType,
		AggregateType:  "payment_intent",
		AggregateID:    state.PaymentIntentID,
		IdempotencyKey: fmt.Sprintf("%s-outbox-%s", input.IdempotencyKey, state.Status),
		Payload: map[string]any{
			"payment_intent_id": state.PaymentIntentID,
			"status":            state.Status,
			"amount":            state.Amount.String(),
			"currency":          state.Currency,
			"provider":          state.Provider,
		},
	}).Get(ctx, nil)
}

// buildResult constructs the final workflow result
func buildResult(ctx workflow.Context, state *PaymentState) *PaymentWorkflowResult {
	return &PaymentWorkflowResult{
		PaymentIntentID:   state.PaymentIntentID,
		Status:            state.Status,
		ProviderPaymentID: state.ProviderPaymentID,
		AuthorizationCode: state.AuthorizationCode,
		CapturedAmount:    state.CapturedAmount,
		DeclineCode:       state.LastDeclineCode,
		DeclineType:       state.LastDeclineType,
		ErrorMessage:      state.LastErrorMessage,
		AttemptCount:      state.CurrentAttempt,
		CompletedAt:       workflow.Now(ctx),
	}
}
