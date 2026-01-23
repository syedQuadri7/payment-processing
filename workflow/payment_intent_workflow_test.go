package workflow

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"payment-processing/pkg/domain"
)

// setupCommonActivityMocks sets up mocks for commonly used activities
func setupCommonActivityMocks(env *testsuite.TestWorkflowEnvironment, activities *Activities) {
	// Database activities that should always succeed
	env.OnActivity(activities.CreatePaymentAttempt, mock.Anything, mock.Anything).
		Return(&CreatePaymentAttemptResult{AttemptID: "attempt_test_001"}, nil)

	env.OnActivity(activities.CompletePaymentAttempt, mock.Anything, mock.Anything).
		Return(nil)

	env.OnActivity(activities.ClassifyDecline, mock.Anything, mock.Anything).
		Return(&ClassifyDeclineResult{
			Found:         true,
			DeclineType:   domain.DeclineTypeSoft,
			RetryEligible: true,
		}, nil)

	env.OnActivity(activities.PersistPaymentState, mock.Anything, mock.Anything).
		Return(nil)

	env.OnActivity(activities.WriteOutboxEvent, mock.Anything, mock.Anything).
		Return(nil)
}

func TestPaymentIntentWorkflow_SuccessfulAuthorization(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Set up activity mocks
	activities := &Activities{}
	setupCommonActivityMocks(env, activities)

	paymentMethodID := "pm_test_123"
	providerPaymentID := "psp_test_456"
	authCode := "AUTH123"
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	env.OnActivity(activities.AuthorizePayment, mock.Anything, mock.Anything).
		Return(&AuthorizePaymentResult{
			Success:           true,
			ProviderPaymentID: providerPaymentID,
			AuthorizationCode: authCode,
			ExpiresAt:         &expiresAt,
		}, nil)

	env.OnActivity(activities.CapturePayment, mock.Anything, mock.Anything).
		Return(&CapturePaymentResult{
			Success:        true,
			CapturedAmount: decimal.NewFromInt(10000),
		}, nil)

	// Execute workflow
	input := PaymentWorkflowInput{
		PaymentIntentID: "pi_test_001",
		CustomerID:      "cus_test_001",
		Amount:          decimal.NewFromInt(10000),
		Currency:        "USD",
		Provider:        domain.ProviderStripe,
		CaptureMethod:   domain.CaptureMethodAutomatic,
		PaymentMethodID: &paymentMethodID,
		IdempotencyKey:  "idem_test_001",
	}

	env.ExecuteWorkflow(PaymentIntentWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result PaymentWorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, domain.PaymentIntentStatusCaptured, result.Status)
	require.NotNil(t, result.ProviderPaymentID)
	require.Equal(t, providerPaymentID, *result.ProviderPaymentID)
	require.Equal(t, 1, result.AttemptCount)
}

func TestPaymentIntentWorkflow_ManualCapture(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	activities := &Activities{}
	setupCommonActivityMocks(env, activities)

	paymentMethodID := "pm_test_123"
	providerPaymentID := "psp_test_456"
	authCode := "AUTH123"
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	env.OnActivity(activities.AuthorizePayment, mock.Anything, mock.Anything).
		Return(&AuthorizePaymentResult{
			Success:           true,
			ProviderPaymentID: providerPaymentID,
			AuthorizationCode: authCode,
			ExpiresAt:         &expiresAt,
		}, nil)

	env.OnActivity(activities.CapturePayment, mock.Anything, mock.Anything).
		Return(&CapturePaymentResult{
			Success:        true,
			CapturedAmount: decimal.NewFromInt(10000),
		}, nil).Maybe() // May or may not be called depending on webhook handling

	// Send capture event after workflow starts
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(SignalWebhookEvent, domain.CanonicalEvent{
			ID:                "evt_test_001",
			Type:              domain.EventCaptureSucceeded,
			Provider:          domain.ProviderStripe,
			ProviderPaymentID: providerPaymentID,
			CapturedAmount:    ptrDecimal(decimal.NewFromInt(10000)),
		})
	}, 1*time.Second)

	input := PaymentWorkflowInput{
		PaymentIntentID: "pi_test_002",
		CustomerID:      "cus_test_001",
		Amount:          decimal.NewFromInt(10000),
		Currency:        "USD",
		Provider:        domain.ProviderStripe,
		CaptureMethod:   domain.CaptureMethodManual,
		PaymentMethodID: &paymentMethodID,
		IdempotencyKey:  "idem_test_002",
	}

	env.ExecuteWorkflow(PaymentIntentWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result PaymentWorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, domain.PaymentIntentStatusCaptured, result.Status)
}

func TestPaymentIntentWorkflow_SoftDeclineWithRetry(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	activities := &Activities{}
	setupCommonActivityMocks(env, activities)

	paymentMethodID := "pm_test_123"
	providerPaymentID := "psp_test_456"
	authCode := "AUTH123"
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	declineCode := "insufficient_funds"
	declineType := domain.DeclineTypeSoft

	// First attempt fails with soft decline
	env.OnActivity(activities.AuthorizePayment, mock.Anything, mock.MatchedBy(func(input AuthorizePaymentInput) bool {
		return input.AttemptNumber == 1
	})).Return(&AuthorizePaymentResult{
		Success:     false,
		DeclineCode: &declineCode,
		DeclineType: &declineType,
	}, nil)

	// Second attempt succeeds
	env.OnActivity(activities.AuthorizePayment, mock.Anything, mock.MatchedBy(func(input AuthorizePaymentInput) bool {
		return input.AttemptNumber == 2
	})).Return(&AuthorizePaymentResult{
		Success:           true,
		ProviderPaymentID: providerPaymentID,
		AuthorizationCode: authCode,
		ExpiresAt:         &expiresAt,
	}, nil)

	env.OnActivity(activities.CapturePayment, mock.Anything, mock.Anything).
		Return(&CapturePaymentResult{
			Success:        true,
			CapturedAmount: decimal.NewFromInt(10000),
		}, nil)

	input := PaymentWorkflowInput{
		PaymentIntentID: "pi_test_003",
		CustomerID:      "cus_test_001",
		Amount:          decimal.NewFromInt(10000),
		Currency:        "USD",
		Provider:        domain.ProviderStripe,
		CaptureMethod:   domain.CaptureMethodAutomatic,
		PaymentMethodID: &paymentMethodID,
		IdempotencyKey:  "idem_test_003",
	}

	env.ExecuteWorkflow(PaymentIntentWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result PaymentWorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, domain.PaymentIntentStatusCaptured, result.Status)
	require.Equal(t, 2, result.AttemptCount)
}

func TestPaymentIntentWorkflow_HardDeclineNoRetry(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	activities := &Activities{}

	// Set up common mocks but override ClassifyDecline for hard decline
	env.OnActivity(activities.CreatePaymentAttempt, mock.Anything, mock.Anything).
		Return(&CreatePaymentAttemptResult{AttemptID: "attempt_test_001"}, nil)

	env.OnActivity(activities.CompletePaymentAttempt, mock.Anything, mock.Anything).
		Return(nil)

	env.OnActivity(activities.ClassifyDecline, mock.Anything, mock.Anything).
		Return(&ClassifyDeclineResult{
			Found:         true,
			CanonicalCode: domain.DeclineCardExpired,
			DeclineType:   domain.DeclineTypeHard,
			RetryEligible: false,
		}, nil)

	env.OnActivity(activities.PersistPaymentState, mock.Anything, mock.Anything).
		Return(nil)

	env.OnActivity(activities.WriteOutboxEvent, mock.Anything, mock.Anything).
		Return(nil)

	paymentMethodID := "pm_test_123"
	declineCode := "expired_card"
	declineType := domain.DeclineTypeHard

	env.OnActivity(activities.AuthorizePayment, mock.Anything, mock.Anything).
		Return(&AuthorizePaymentResult{
			Success:     false,
			DeclineCode: &declineCode,
			DeclineType: &declineType,
		}, nil)

	input := PaymentWorkflowInput{
		PaymentIntentID: "pi_test_004",
		CustomerID:      "cus_test_001",
		Amount:          decimal.NewFromInt(10000),
		Currency:        "USD",
		Provider:        domain.ProviderStripe,
		CaptureMethod:   domain.CaptureMethodAutomatic,
		PaymentMethodID: &paymentMethodID,
		IdempotencyKey:  "idem_test_004",
	}

	env.ExecuteWorkflow(PaymentIntentWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result PaymentWorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, domain.PaymentIntentStatusFailed, result.Status)
	require.Equal(t, 1, result.AttemptCount)
	require.NotNil(t, result.DeclineCode)
	// Decline code is now the canonical code (converted from provider code via ClassifyDecline)
	require.Equal(t, string(domain.DeclineCardExpired), *result.DeclineCode)
}

func TestPaymentIntentWorkflow_Cancellation(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	activities := &Activities{}
	setupCommonActivityMocks(env, activities)

	paymentMethodID := "pm_test_123"
	providerPaymentID := "psp_test_456"
	authCode := "AUTH123"
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	env.OnActivity(activities.AuthorizePayment, mock.Anything, mock.Anything).
		Return(&AuthorizePaymentResult{
			Success:           true,
			ProviderPaymentID: providerPaymentID,
			AuthorizationCode: authCode,
			ExpiresAt:         &expiresAt,
		}, nil)

	env.OnActivity(activities.VoidPayment, mock.Anything, mock.Anything).
		Return(&VoidPaymentResult{
			Success: true,
		}, nil)

	// Send cancel signal after authorization
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(SignalCancelPayment, "user_requested")
	}, 1*time.Second)

	input := PaymentWorkflowInput{
		PaymentIntentID: "pi_test_005",
		CustomerID:      "cus_test_001",
		Amount:          decimal.NewFromInt(10000),
		Currency:        "USD",
		Provider:        domain.ProviderStripe,
		CaptureMethod:   domain.CaptureMethodManual,
		PaymentMethodID: &paymentMethodID,
		IdempotencyKey:  "idem_test_005",
	}

	env.ExecuteWorkflow(PaymentIntentWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result PaymentWorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, domain.PaymentIntentStatusVoided, result.Status)
}

func TestPaymentIntentWorkflow_RequiresPaymentMethod(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	activities := &Activities{}
	setupCommonActivityMocks(env, activities)

	providerPaymentID := "psp_test_456"
	authCode := "AUTH123"
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	env.OnActivity(activities.AuthorizePayment, mock.Anything, mock.Anything).
		Return(&AuthorizePaymentResult{
			Success:           true,
			ProviderPaymentID: providerPaymentID,
			AuthorizationCode: authCode,
			ExpiresAt:         &expiresAt,
		}, nil)

	env.OnActivity(activities.CapturePayment, mock.Anything, mock.Anything).
		Return(&CapturePaymentResult{
			Success:        true,
			CapturedAmount: decimal.NewFromInt(10000),
		}, nil)

	// Send payment method update signal
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(SignalUpdatePaymentMethod, "pm_new_123")
	}, 1*time.Second)

	input := PaymentWorkflowInput{
		PaymentIntentID: "pi_test_006",
		CustomerID:      "cus_test_001",
		Amount:          decimal.NewFromInt(10000),
		Currency:        "USD",
		Provider:        domain.ProviderStripe,
		CaptureMethod:   domain.CaptureMethodAutomatic,
		PaymentMethodID: nil, // No payment method
		IdempotencyKey:  "idem_test_006",
	}

	env.ExecuteWorkflow(PaymentIntentWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result PaymentWorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, domain.PaymentIntentStatusCaptured, result.Status)
}

func TestPaymentIntentWorkflow_QueryStatus(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	activities := &Activities{}
	setupCommonActivityMocks(env, activities)

	paymentMethodID := "pm_test_123"
	providerPaymentID := "psp_test_456"
	authCode := "AUTH123"
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	env.OnActivity(activities.AuthorizePayment, mock.Anything, mock.Anything).
		Return(&AuthorizePaymentResult{
			Success:           true,
			ProviderPaymentID: providerPaymentID,
			AuthorizationCode: authCode,
			ExpiresAt:         &expiresAt,
		}, nil)

	// Query status during manual capture wait (after authorization completes)
	var queriedStatus domain.PaymentIntentStatus
	env.RegisterDelayedCallback(func() {
		result, err := env.QueryWorkflow(QueryPaymentStatus)
		require.NoError(t, err)
		require.NoError(t, result.Get(&queriedStatus))
	}, 5*time.Second) // Increased to allow time for authorization activities to complete

	// Then capture (after query)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(SignalWebhookEvent, domain.CanonicalEvent{
			ID:                "evt_test_001",
			Type:              domain.EventCaptureSucceeded,
			Provider:          domain.ProviderStripe,
			ProviderPaymentID: providerPaymentID,
			CapturedAmount:    ptrDecimal(decimal.NewFromInt(10000)),
		})
	}, 10*time.Second)

	input := PaymentWorkflowInput{
		PaymentIntentID: "pi_test_007",
		CustomerID:      "cus_test_001",
		Amount:          decimal.NewFromInt(10000),
		Currency:        "USD",
		Provider:        domain.ProviderStripe,
		CaptureMethod:   domain.CaptureMethodManual,
		PaymentMethodID: &paymentMethodID,
		IdempotencyKey:  "idem_test_007",
	}

	env.ExecuteWorkflow(PaymentIntentWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, domain.PaymentIntentStatusAuthorized, queriedStatus)
}

func TestPaymentState_CanTransitionTo(t *testing.T) {
	tests := []struct {
		name      string
		from      domain.PaymentIntentStatus
		to        domain.PaymentIntentStatus
		canTransition bool
	}{
		{"created to requires_auth", domain.PaymentIntentStatusCreated, domain.PaymentIntentStatusRequiresAuth, true},
		{"created to authorized", domain.PaymentIntentStatusCreated, domain.PaymentIntentStatusAuthorized, true},
		{"created to captured", domain.PaymentIntentStatusCreated, domain.PaymentIntentStatusCaptured, false},
		{"authorized to captured", domain.PaymentIntentStatusAuthorized, domain.PaymentIntentStatusCaptured, true},
		{"authorized to voided", domain.PaymentIntentStatusAuthorized, domain.PaymentIntentStatusVoided, true},
		{"captured to voided", domain.PaymentIntentStatusCaptured, domain.PaymentIntentStatusVoided, false},
		{"failed to recovering", domain.PaymentIntentStatusFailed, domain.PaymentIntentStatusRecovering, true},
		{"recovering to authorized", domain.PaymentIntentStatusRecovering, domain.PaymentIntentStatusAuthorized, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &PaymentState{Status: tt.from}
			result := state.CanTransitionTo(tt.to)
			require.Equal(t, tt.canTransition, result)
		})
	}
}

func TestPaymentState_IsTerminal(t *testing.T) {
	tests := []struct {
		status     domain.PaymentIntentStatus
		isTerminal bool
	}{
		{domain.PaymentIntentStatusCreated, false},
		{domain.PaymentIntentStatusRequiresAuth, false},
		{domain.PaymentIntentStatusAuthorized, false},
		{domain.PaymentIntentStatusRecovering, false},
		{domain.PaymentIntentStatusCaptured, true},
		{domain.PaymentIntentStatusFailed, true},
		{domain.PaymentIntentStatusCancelled, true},
		{domain.PaymentIntentStatusVoided, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			state := &PaymentState{Status: tt.status}
			require.Equal(t, tt.isTerminal, state.IsTerminal())
		})
	}
}

func TestPaymentState_CanRetry(t *testing.T) {
	softDecline := domain.DeclineTypeSoft
	hardDecline := domain.DeclineTypeHard

	tests := []struct {
		name       string
		state      PaymentState
		canRetry   bool
	}{
		{
			name: "soft decline first attempt",
			state: PaymentState{
				LastDeclineType: &softDecline,
				CurrentAttempt:  1,
			},
			canRetry: true,
		},
		{
			name: "hard decline",
			state: PaymentState{
				LastDeclineType: &hardDecline,
				CurrentAttempt:  1,
			},
			canRetry: false,
		},
		{
			name: "soft decline max attempts",
			state: PaymentState{
				LastDeclineType: &softDecline,
				CurrentAttempt:  5, // Max is 4 retries + 1 initial
			},
			canRetry: false,
		},
		{
			name: "no decline type",
			state: PaymentState{
				LastDeclineType: nil,
				CurrentAttempt:  1,
			},
			canRetry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.canRetry, tt.state.CanRetry())
		})
	}
}

func TestPaymentState_NextRetryDelay(t *testing.T) {
	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 0},
		{1, 4 * time.Hour},
		{2, 12 * time.Hour},
		{3, 24 * time.Hour},
		{4, 48 * time.Hour},
		{5, 0}, // Beyond max
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.attempt)), func(t *testing.T) {
			state := &PaymentState{CurrentAttempt: tt.attempt}
			require.Equal(t, tt.expected, state.NextRetryDelay())
		})
	}
}

// Helper function to create pointer to decimal
func ptrDecimal(d decimal.Decimal) *decimal.Decimal {
	return &d
}

// TestPaymentIntentWorkflow_ImmediateRetryOnPaymentMethodUpdate tests FR-DEC-09:
// When a payment method is updated during RECOVERING state, retry is triggered immediately
func TestPaymentIntentWorkflow_ImmediateRetryOnPaymentMethodUpdate(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	activities := &Activities{}
	setupCommonActivityMocks(env, activities)

	paymentMethodID := "pm_test_123"
	newPaymentMethodID := "pm_new_456"
	providerPaymentID := "psp_test_456"
	authCode := "AUTH123"
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	declineCode := "insufficient_funds"
	declineType := domain.DeclineTypeSoft

	// First attempt fails with soft decline
	env.OnActivity(activities.AuthorizePayment, mock.Anything, mock.MatchedBy(func(input AuthorizePaymentInput) bool {
		return input.AttemptNumber == 1
	})).Return(&AuthorizePaymentResult{
		Success:     false,
		DeclineCode: &declineCode,
		DeclineType: &declineType,
	}, nil)

	// Second attempt (after payment method update) succeeds
	// This attempt should use the new payment method
	env.OnActivity(activities.AuthorizePayment, mock.Anything, mock.MatchedBy(func(input AuthorizePaymentInput) bool {
		return input.AttemptNumber == 2 && input.PaymentMethodID == newPaymentMethodID
	})).Return(&AuthorizePaymentResult{
		Success:           true,
		ProviderPaymentID: providerPaymentID,
		AuthorizationCode: authCode,
		ExpiresAt:         &expiresAt,
	}, nil)

	env.OnActivity(activities.CapturePayment, mock.Anything, mock.Anything).
		Return(&CapturePaymentResult{
			Success:        true,
			CapturedAmount: decimal.NewFromInt(10000),
		}, nil)

	// Send payment method update signal during retry wait
	// Normal retry wait is 4 hours, but we update payment method after 1 minute
	// This should trigger immediate retry
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(SignalUpdatePaymentMethod, newPaymentMethodID)
	}, 1*time.Minute)

	input := PaymentWorkflowInput{
		PaymentIntentID: "pi_test_immediate_retry",
		CustomerID:      "cus_test_001",
		Amount:          decimal.NewFromInt(10000),
		Currency:        "USD",
		Provider:        domain.ProviderStripe,
		CaptureMethod:   domain.CaptureMethodAutomatic,
		PaymentMethodID: &paymentMethodID,
		IdempotencyKey:  "idem_test_immediate_retry",
	}

	env.ExecuteWorkflow(PaymentIntentWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result PaymentWorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, domain.PaymentIntentStatusCaptured, result.Status)
	require.Equal(t, 2, result.AttemptCount)
}

// TestPaymentIntentWorkflow_ClassifyDecline tests FR-DEC-01:
// Decline codes are properly classified via the ClassifyDecline activity
func TestPaymentIntentWorkflow_ClassifyDecline(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	activities := &Activities{}

	// Set up common mocks
	env.OnActivity(activities.CreatePaymentAttempt, mock.Anything, mock.Anything).
		Return(&CreatePaymentAttemptResult{AttemptID: "attempt_test_001"}, nil)

	env.OnActivity(activities.CompletePaymentAttempt, mock.Anything, mock.Anything).
		Return(nil)

	env.OnActivity(activities.PersistPaymentState, mock.Anything, mock.Anything).
		Return(nil)

	env.OnActivity(activities.WriteOutboxEvent, mock.Anything, mock.Anything).
		Return(nil)

	// Custom ClassifyDecline mock that verifies the lookup is called correctly
	env.OnActivity(activities.ClassifyDecline, mock.Anything, mock.MatchedBy(func(input ClassifyDeclineInput) bool {
		return input.Provider == domain.ProviderStripe && input.ProviderCode == "card_declined"
	})).Return(&ClassifyDeclineResult{
		Found:           true,
		CanonicalCode:   domain.DeclineGenericDecline,
		DeclineType:     domain.DeclineTypeSoft,
		RetryEligible:   true,
		Description:     "Generic card decline",
		SuggestedAction: "Retry or request alternate payment method",
	}, nil)

	paymentMethodID := "pm_test_123"
	providerPaymentID := "psp_test_456"
	authCode := "AUTH123"
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	declineCode := "card_declined" // Provider-specific code

	// First attempt fails with provider-specific decline code
	env.OnActivity(activities.AuthorizePayment, mock.Anything, mock.MatchedBy(func(input AuthorizePaymentInput) bool {
		return input.AttemptNumber == 1
	})).Return(&AuthorizePaymentResult{
		Success:     false,
		DeclineCode: &declineCode,
	}, nil)

	// Second attempt succeeds
	env.OnActivity(activities.AuthorizePayment, mock.Anything, mock.MatchedBy(func(input AuthorizePaymentInput) bool {
		return input.AttemptNumber == 2
	})).Return(&AuthorizePaymentResult{
		Success:           true,
		ProviderPaymentID: providerPaymentID,
		AuthorizationCode: authCode,
		ExpiresAt:         &expiresAt,
	}, nil)

	env.OnActivity(activities.CapturePayment, mock.Anything, mock.Anything).
		Return(&CapturePaymentResult{
			Success:        true,
			CapturedAmount: decimal.NewFromInt(10000),
		}, nil)

	input := PaymentWorkflowInput{
		PaymentIntentID: "pi_test_classify",
		CustomerID:      "cus_test_001",
		Amount:          decimal.NewFromInt(10000),
		Currency:        "USD",
		Provider:        domain.ProviderStripe,
		CaptureMethod:   domain.CaptureMethodAutomatic,
		PaymentMethodID: &paymentMethodID,
		IdempotencyKey:  "idem_test_classify",
	}

	env.ExecuteWorkflow(PaymentIntentWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result PaymentWorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, domain.PaymentIntentStatusCaptured, result.Status)
	require.Equal(t, 2, result.AttemptCount)
}
