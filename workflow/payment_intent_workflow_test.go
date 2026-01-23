package workflow

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"payment-processing/internal/domain"
)

func TestPaymentIntentWorkflow_SuccessfulAuthorization(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Set up activity mocks
	activities := &Activities{}

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

	env.OnActivity(activities.PersistPaymentState, mock.Anything, mock.Anything).
		Return(nil)

	env.OnActivity(activities.WriteOutboxEvent, mock.Anything, mock.Anything).
		Return(nil)

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

	env.OnActivity(activities.PersistPaymentState, mock.Anything, mock.Anything).
		Return(nil)

	env.OnActivity(activities.WriteOutboxEvent, mock.Anything, mock.Anything).
		Return(nil)

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

	env.OnActivity(activities.PersistPaymentState, mock.Anything, mock.Anything).
		Return(nil)

	env.OnActivity(activities.WriteOutboxEvent, mock.Anything, mock.Anything).
		Return(nil)

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

	paymentMethodID := "pm_test_123"
	declineCode := "expired_card"
	declineType := domain.DeclineTypeHard

	env.OnActivity(activities.AuthorizePayment, mock.Anything, mock.Anything).
		Return(&AuthorizePaymentResult{
			Success:     false,
			DeclineCode: &declineCode,
			DeclineType: &declineType,
		}, nil)

	env.OnActivity(activities.PersistPaymentState, mock.Anything, mock.Anything).
		Return(nil)

	env.OnActivity(activities.WriteOutboxEvent, mock.Anything, mock.Anything).
		Return(nil)

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
	require.Equal(t, declineCode, *result.DeclineCode)
}

func TestPaymentIntentWorkflow_Cancellation(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	activities := &Activities{}

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

	env.OnActivity(activities.PersistPaymentState, mock.Anything, mock.Anything).
		Return(nil)

	env.OnActivity(activities.WriteOutboxEvent, mock.Anything, mock.Anything).
		Return(nil)

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

	env.OnActivity(activities.PersistPaymentState, mock.Anything, mock.Anything).
		Return(nil)

	env.OnActivity(activities.WriteOutboxEvent, mock.Anything, mock.Anything).
		Return(nil)

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

	env.OnActivity(activities.PersistPaymentState, mock.Anything, mock.Anything).
		Return(nil)

	env.OnActivity(activities.WriteOutboxEvent, mock.Anything, mock.Anything).
		Return(nil)

	// Query status during manual capture wait
	var queriedStatus domain.PaymentIntentStatus
	env.RegisterDelayedCallback(func() {
		result, err := env.QueryWorkflow(QueryPaymentStatus)
		require.NoError(t, err)
		require.NoError(t, result.Get(&queriedStatus))
	}, 1*time.Second)

	// Then capture
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(SignalWebhookEvent, domain.CanonicalEvent{
			ID:                "evt_test_001",
			Type:              domain.EventCaptureSucceeded,
			Provider:          domain.ProviderStripe,
			ProviderPaymentID: providerPaymentID,
			CapturedAmount:    ptrDecimal(decimal.NewFromInt(10000)),
		})
	}, 2*time.Second)

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
