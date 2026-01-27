package workflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"payment-processing/shared/domain"
)

// MockAccountRepository is a mock implementation of AccountRepository
type MockAccountRepository struct {
	mock.Mock
}

func (m *MockAccountRepository) GetByID(ctx context.Context, id string) (*domain.Account, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Account), args.Error(1)
}

func (m *MockAccountRepository) GetByName(ctx context.Context, name string, currency string) (*domain.Account, error) {
	args := m.Called(ctx, name, currency)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Account), args.Error(1)
}

func (m *MockAccountRepository) GetClearingAccounts(ctx context.Context, currency string) ([]*domain.Account, error) {
	args := m.Called(ctx, currency)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Account), args.Error(1)
}

func (m *MockAccountRepository) UpdateBalances(ctx context.Context, id string, ledger, pending, available decimal.Decimal, expectedVersion int) error {
	args := m.Called(ctx, id, ledger, pending, available, expectedVersion)
	return args.Error(0)
}

// MockJournalEntryRepository is a mock implementation of JournalEntryRepository
type MockJournalEntryRepository struct {
	mock.Mock
}

func (m *MockJournalEntryRepository) Create(ctx context.Context, entry *domain.JournalEntry) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

func (m *MockJournalEntryRepository) GetByID(ctx context.Context, id string) (*domain.JournalEntry, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.JournalEntry), args.Error(1)
}

func (m *MockJournalEntryRepository) GetByReference(ctx context.Context, refType domain.ReferenceType, refID string) ([]*domain.JournalEntry, error) {
	args := m.Called(ctx, refType, refID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.JournalEntry), args.Error(1)
}

// MockLedgerEntryRepository is a mock implementation of LedgerEntryRepository
type MockLedgerEntryRepository struct {
	mock.Mock
}

func (m *MockLedgerEntryRepository) Create(ctx context.Context, entry *domain.LedgerEntry) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

func (m *MockLedgerEntryRepository) GetByJournalEntryID(ctx context.Context, journalEntryID string) ([]*domain.LedgerEntry, error) {
	args := m.Called(ctx, journalEntryID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.LedgerEntry), args.Error(1)
}

func (m *MockLedgerEntryRepository) GetByAccountID(ctx context.Context, accountID string, limit int) ([]*domain.LedgerEntry, error) {
	args := m.Called(ctx, accountID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.LedgerEntry), args.Error(1)
}

func (m *MockLedgerEntryRepository) GetRunningBalance(ctx context.Context, accountID string) (decimal.Decimal, error) {
	args := m.Called(ctx, accountID)
	return args.Get(0).(decimal.Decimal), args.Error(1)
}

// Helper function to create a test account
func newTestAccount(id, name string, ledgerBalance, pendingBalance decimal.Decimal, version int) *domain.Account {
	return &domain.Account{
		ID:               id,
		Type:             domain.AccountTypeAsset,
		Name:             name,
		Currency:         "USD",
		LedgerBalance:    ledgerBalance,
		PendingBalance:   pendingBalance,
		AvailableBalance: ledgerBalance.Sub(pendingBalance),
		ReservedBalance:  decimal.Zero,
		Status:           domain.AccountStatusActive,
		Version:          version,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
}

func newTestClearingAccount(id, name string, ledgerBalance decimal.Decimal, version int) *domain.Account {
	return &domain.Account{
		ID:               id,
		Type:             domain.AccountTypeClearing,
		Name:             name,
		Currency:         "USD",
		LedgerBalance:    ledgerBalance,
		PendingBalance:   decimal.Zero,
		AvailableBalance: ledgerBalance,
		ReservedBalance:  decimal.Zero,
		Status:           domain.AccountStatusActive,
		Version:          version,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
}

// decimalMatcher creates a matcher for decimal.Decimal values
func decimalMatcher(expected decimal.Decimal) interface{} {
	return mock.MatchedBy(func(d decimal.Decimal) bool { return d.Equal(expected) })
}

func TestPlaceAuthorizationHold_Success(t *testing.T) {
	ctx := context.Background()
	mockAccountRepo := new(MockAccountRepository)

	activities := NewLedgerActivities(mockAccountRepo, nil, nil)

	// Setup - customer has $1000, placing $100 hold
	customerAccount := newTestAccount("acc_customer_001", "Customer Account", decimal.NewFromInt(1000), decimal.Zero, 1)

	mockAccountRepo.On("GetByID", ctx, "acc_customer_001").Return(customerAccount, nil)
	mockAccountRepo.On("UpdateBalances", ctx, "acc_customer_001",
		decimalMatcher(decimal.NewFromInt(1000)), // ledger unchanged
		decimalMatcher(decimal.NewFromInt(100)),  // pending increased
		decimalMatcher(decimal.NewFromInt(900)),  // available decreased
		1,                                         // expected version
	).Return(nil)

	result, err := activities.PlaceAuthorizationHold(ctx, PlaceAuthorizationHoldInput{
		CustomerAccountID: "acc_customer_001",
		Amount:            decimal.NewFromInt(100),
		PaymentIntentID:   "pi_test_001",
	})

	require.NoError(t, err)
	require.True(t, result.Success)
	assert.Equal(t, 2, result.NewVersion)

	mockAccountRepo.AssertExpectations(t)
}

func TestPlaceAuthorizationHold_InsufficientFunds(t *testing.T) {
	ctx := context.Background()
	mockAccountRepo := new(MockAccountRepository)

	activities := NewLedgerActivities(mockAccountRepo, nil, nil)

	// Setup - customer has $100, trying to hold $500
	customerAccount := newTestAccount("acc_customer_001", "Customer Account", decimal.NewFromInt(100), decimal.Zero, 1)

	mockAccountRepo.On("GetByID", ctx, "acc_customer_001").Return(customerAccount, nil)

	result, err := activities.PlaceAuthorizationHold(ctx, PlaceAuthorizationHoldInput{
		CustomerAccountID: "acc_customer_001",
		Amount:            decimal.NewFromInt(500),
		PaymentIntentID:   "pi_test_001",
	})

	require.NoError(t, err)
	require.False(t, result.Success)
	assert.Contains(t, result.ErrorMessage, "insufficient funds")

	// UpdateBalances should NOT be called
	mockAccountRepo.AssertNotCalled(t, "UpdateBalances", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestPlaceAuthorizationHold_AccountNotFound(t *testing.T) {
	ctx := context.Background()
	mockAccountRepo := new(MockAccountRepository)

	activities := NewLedgerActivities(mockAccountRepo, nil, nil)

	mockAccountRepo.On("GetByID", ctx, "acc_nonexistent").Return(nil, nil)

	result, err := activities.PlaceAuthorizationHold(ctx, PlaceAuthorizationHoldInput{
		CustomerAccountID: "acc_nonexistent",
		Amount:            decimal.NewFromInt(100),
		PaymentIntentID:   "pi_test_001",
	})

	require.NoError(t, err)
	require.False(t, result.Success)
	assert.Equal(t, "account not found", result.ErrorMessage)
}

func TestReleaseAuthorizationHold_Success(t *testing.T) {
	ctx := context.Background()
	mockAccountRepo := new(MockAccountRepository)

	activities := NewLedgerActivities(mockAccountRepo, nil, nil)

	// Setup - customer has $1000 with $100 pending, releasing hold
	customerAccount := newTestAccount("acc_customer_001", "Customer Account", decimal.NewFromInt(1000), decimal.NewFromInt(100), 1)

	mockAccountRepo.On("GetByID", ctx, "acc_customer_001").Return(customerAccount, nil)
	mockAccountRepo.On("UpdateBalances", ctx, "acc_customer_001",
		decimalMatcher(decimal.NewFromInt(1000)), // ledger unchanged
		decimalMatcher(decimal.Zero),             // pending decreased to 0
		decimalMatcher(decimal.NewFromInt(1000)), // available increased
		1,                                         // expected version
	).Return(nil)

	result, err := activities.ReleaseAuthorizationHold(ctx, ReleaseAuthorizationHoldInput{
		CustomerAccountID: "acc_customer_001",
		Amount:            decimal.NewFromInt(100),
		PaymentIntentID:   "pi_test_001",
		Reason:            "void",
	})

	require.NoError(t, err)
	require.True(t, result.Success)
	assert.Equal(t, 2, result.NewVersion)

	mockAccountRepo.AssertExpectations(t)
}

func TestReleaseAuthorizationHold_ExceedsPending(t *testing.T) {
	ctx := context.Background()
	mockAccountRepo := new(MockAccountRepository)

	activities := NewLedgerActivities(mockAccountRepo, nil, nil)

	// Setup - customer has only $50 pending, trying to release $100
	customerAccount := newTestAccount("acc_customer_001", "Customer Account", decimal.NewFromInt(1000), decimal.NewFromInt(50), 1)

	mockAccountRepo.On("GetByID", ctx, "acc_customer_001").Return(customerAccount, nil)

	result, err := activities.ReleaseAuthorizationHold(ctx, ReleaseAuthorizationHoldInput{
		CustomerAccountID: "acc_customer_001",
		Amount:            decimal.NewFromInt(100),
		PaymentIntentID:   "pi_test_001",
		Reason:            "void",
	})

	require.NoError(t, err)
	require.False(t, result.Success)
	assert.Contains(t, result.ErrorMessage, "exceeds pending")
}

func TestRecordCapture_Success(t *testing.T) {
	ctx := context.Background()
	mockAccountRepo := new(MockAccountRepository)
	mockJournalRepo := new(MockJournalEntryRepository)
	mockLedgerRepo := new(MockLedgerEntryRepository)

	activities := NewLedgerActivities(mockAccountRepo, mockJournalRepo, mockLedgerRepo)

	// Setup accounts
	customerAccount := newTestAccount("acc_customer_001", "Customer Account", decimal.NewFromInt(1000), decimal.NewFromInt(100), 1)
	clearingAccount := newTestClearingAccount("acc_clearing_001", ClearingAccountSettlementName, decimal.Zero, 1)

	// Check for existing capture (idempotency)
	mockJournalRepo.On("GetByReference", ctx, domain.ReferenceTypePaymentIntent, "pi_test_001").
		Return([]*domain.JournalEntry{}, nil)

	mockAccountRepo.On("GetByName", ctx, ClearingAccountSettlementName, "USD").Return(clearingAccount, nil)
	mockAccountRepo.On("GetByID", ctx, "acc_customer_001").Return(customerAccount, nil)

	mockJournalRepo.On("Create", ctx, mock.AnythingOfType("*domain.JournalEntry")).Return(nil)
	mockLedgerRepo.On("Create", ctx, mock.AnythingOfType("*domain.LedgerEntry")).Return(nil).Times(2)

	// Customer account update after debit
	mockAccountRepo.On("UpdateBalances", ctx, "acc_customer_001",
		decimalMatcher(decimal.NewFromInt(900)), // ledger decreased by 100
		decimalMatcher(decimal.Zero),            // pending released
		decimalMatcher(decimal.NewFromInt(900)), // available
		1,                                        // expected version
	).Return(nil)

	// Clearing account update after credit
	mockAccountRepo.On("UpdateBalances", ctx, "acc_clearing_001",
		decimalMatcher(decimal.NewFromInt(100)), // ledger increased by 100
		decimalMatcher(decimal.Zero),            // no pending
		decimalMatcher(decimal.NewFromInt(100)), // available
		1,                                        // expected version
	).Return(nil)

	result, err := activities.RecordCapture(ctx, RecordCaptureInput{
		PaymentIntentID:   "pi_test_001",
		CustomerAccountID: "acc_customer_001",
		Amount:            decimal.NewFromInt(100),
		Currency:          "USD",
		IdempotencyKey:    "idem_test_001",
	})

	require.NoError(t, err)
	require.True(t, result.Success)
	assert.NotEmpty(t, result.JournalEntryID)

	mockAccountRepo.AssertExpectations(t)
	mockJournalRepo.AssertExpectations(t)
	mockLedgerRepo.AssertExpectations(t)
}

func TestRecordCapture_Idempotent(t *testing.T) {
	ctx := context.Background()
	mockAccountRepo := new(MockAccountRepository)
	mockJournalRepo := new(MockJournalEntryRepository)
	mockLedgerRepo := new(MockLedgerEntryRepository)

	activities := NewLedgerActivities(mockAccountRepo, mockJournalRepo, mockLedgerRepo)

	// Existing capture entry found
	existingEntry := &domain.JournalEntry{
		ID:          "je_existing_001",
		Description: "Capture payment pi_test_001",
	}
	mockJournalRepo.On("GetByReference", ctx, domain.ReferenceTypePaymentIntent, "pi_test_001").
		Return([]*domain.JournalEntry{existingEntry}, nil)

	result, err := activities.RecordCapture(ctx, RecordCaptureInput{
		PaymentIntentID:   "pi_test_001",
		CustomerAccountID: "acc_customer_001",
		Amount:            decimal.NewFromInt(100),
		Currency:          "USD",
		IdempotencyKey:    "idem_test_001",
	})

	require.NoError(t, err)
	require.True(t, result.Success)
	assert.Equal(t, "je_existing_001", result.JournalEntryID)

	// No new entries should be created
	mockJournalRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	mockLedgerRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestRecordCapture_ClearingAccountNotFound(t *testing.T) {
	ctx := context.Background()
	mockAccountRepo := new(MockAccountRepository)
	mockJournalRepo := new(MockJournalEntryRepository)
	mockLedgerRepo := new(MockLedgerEntryRepository)

	activities := NewLedgerActivities(mockAccountRepo, mockJournalRepo, mockLedgerRepo)

	mockJournalRepo.On("GetByReference", ctx, domain.ReferenceTypePaymentIntent, "pi_test_001").
		Return([]*domain.JournalEntry{}, nil)

	mockAccountRepo.On("GetByName", ctx, ClearingAccountSettlementName, "USD").Return(nil, nil)

	result, err := activities.RecordCapture(ctx, RecordCaptureInput{
		PaymentIntentID:   "pi_test_001",
		CustomerAccountID: "acc_customer_001",
		Amount:            decimal.NewFromInt(100),
		Currency:          "USD",
	})

	require.NoError(t, err)
	require.False(t, result.Success)
	assert.Contains(t, result.ErrorMessage, "settlement clearing account not found")
}

func TestRecordRefund_Success(t *testing.T) {
	ctx := context.Background()
	mockAccountRepo := new(MockAccountRepository)
	mockJournalRepo := new(MockJournalEntryRepository)
	mockLedgerRepo := new(MockLedgerEntryRepository)

	activities := NewLedgerActivities(mockAccountRepo, mockJournalRepo, mockLedgerRepo)

	// Setup accounts
	customerAccount := newTestAccount("acc_customer_001", "Customer Account", decimal.NewFromInt(900), decimal.Zero, 1)
	merchantAccount := newTestAccount("acc_merchant_001", "Merchant Account", decimal.NewFromInt(100), decimal.Zero, 1)
	merchantAccount.Type = domain.AccountTypeLiability

	// Check for existing refund (idempotency)
	mockJournalRepo.On("GetByReference", ctx, domain.ReferenceTypeRefund, "ref_test_001").
		Return([]*domain.JournalEntry{}, nil)

	mockAccountRepo.On("GetByID", ctx, "acc_customer_001").Return(customerAccount, nil)
	mockAccountRepo.On("GetByID", ctx, "acc_merchant_001").Return(merchantAccount, nil)

	mockJournalRepo.On("Create", ctx, mock.AnythingOfType("*domain.JournalEntry")).Return(nil)
	mockLedgerRepo.On("Create", ctx, mock.AnythingOfType("*domain.LedgerEntry")).Return(nil).Times(2)

	// Merchant account update after debit (refund decreases merchant balance)
	mockAccountRepo.On("UpdateBalances", ctx, "acc_merchant_001",
		decimalMatcher(decimal.Zero), // ledger decreased by 100
		decimalMatcher(decimal.Zero), // no pending
		decimalMatcher(decimal.Zero), // available
		1,                             // expected version
	).Return(nil)

	// Customer account update after credit (refund increases customer balance)
	mockAccountRepo.On("UpdateBalances", ctx, "acc_customer_001",
		decimalMatcher(decimal.NewFromInt(1000)), // ledger increased by 100
		decimalMatcher(decimal.Zero),             // no pending
		decimalMatcher(decimal.NewFromInt(1000)), // available
		1,                                         // expected version
	).Return(nil)

	result, err := activities.RecordRefund(ctx, RecordRefundInput{
		PaymentIntentID:   "pi_test_001",
		RefundID:          "ref_test_001",
		CustomerAccountID: "acc_customer_001",
		MerchantAccountID: "acc_merchant_001",
		Amount:            decimal.NewFromInt(100),
		Currency:          "USD",
		IdempotencyKey:    "idem_test_001",
	})

	require.NoError(t, err)
	require.True(t, result.Success)
	assert.NotEmpty(t, result.JournalEntryID)

	mockAccountRepo.AssertExpectations(t)
	mockJournalRepo.AssertExpectations(t)
	mockLedgerRepo.AssertExpectations(t)
}

func TestMonitorClearingAccounts_NoAlerts(t *testing.T) {
	ctx := context.Background()
	mockAccountRepo := new(MockAccountRepository)

	activities := NewLedgerActivities(mockAccountRepo, nil, nil)

	// All clearing accounts have zero balance
	clearingAccounts := []*domain.Account{
		newTestClearingAccount("acc_clearing_001", ClearingAccountSettlementName, decimal.Zero, 1),
		newTestClearingAccount("acc_clearing_002", ClearingAccountFeeName, decimal.Zero, 1),
	}

	mockAccountRepo.On("GetClearingAccounts", ctx, "USD").Return(clearingAccounts, nil)

	result, err := activities.MonitorClearingAccounts(ctx, MonitorClearingAccountsInput{
		Currency: "USD",
	})

	require.NoError(t, err)
	assert.Empty(t, result.Alerts)
	assert.False(t, result.HasWarnings)
	assert.False(t, result.HasCritical)
}

func TestMonitorClearingAccounts_WithWarning(t *testing.T) {
	ctx := context.Background()
	mockAccountRepo := new(MockAccountRepository)

	activities := NewLedgerActivities(mockAccountRepo, nil, nil)

	// One account has non-zero balance updated 13 hours ago (warning threshold)
	warningAccount := newTestClearingAccount("acc_clearing_001", ClearingAccountSettlementName, decimal.NewFromInt(100), 1)
	warningAccount.UpdatedAt = time.Now().Add(-13 * time.Hour)

	clearingAccounts := []*domain.Account{
		warningAccount,
		newTestClearingAccount("acc_clearing_002", ClearingAccountFeeName, decimal.Zero, 1),
	}

	mockAccountRepo.On("GetClearingAccounts", ctx, "USD").Return(clearingAccounts, nil)

	result, err := activities.MonitorClearingAccounts(ctx, MonitorClearingAccountsInput{
		Currency: "USD",
	})

	require.NoError(t, err)
	require.Len(t, result.Alerts, 1)
	assert.True(t, result.HasWarnings)
	assert.False(t, result.HasCritical)
	assert.Equal(t, "warning", result.Alerts[0].AlertLevel)
	assert.Equal(t, ClearingAccountSettlementName, result.Alerts[0].AccountName)
}

func TestMonitorClearingAccounts_WithCritical(t *testing.T) {
	ctx := context.Background()
	mockAccountRepo := new(MockAccountRepository)

	activities := NewLedgerActivities(mockAccountRepo, nil, nil)

	// One account has non-zero balance updated 25 hours ago (critical threshold)
	criticalAccount := newTestClearingAccount("acc_clearing_001", ClearingAccountSettlementName, decimal.NewFromInt(500), 1)
	criticalAccount.UpdatedAt = time.Now().Add(-25 * time.Hour)

	clearingAccounts := []*domain.Account{criticalAccount}

	mockAccountRepo.On("GetClearingAccounts", ctx, "USD").Return(clearingAccounts, nil)

	result, err := activities.MonitorClearingAccounts(ctx, MonitorClearingAccountsInput{
		Currency: "USD",
	})

	require.NoError(t, err)
	require.Len(t, result.Alerts, 1)
	assert.True(t, result.HasCritical)
	assert.Equal(t, "critical", result.Alerts[0].AlertLevel)
}

func TestGetAccountBalance_Success(t *testing.T) {
	ctx := context.Background()
	mockAccountRepo := new(MockAccountRepository)

	activities := NewLedgerActivities(mockAccountRepo, nil, nil)

	account := newTestAccount("acc_customer_001", "Customer Account",
		decimal.NewFromInt(1000),
		decimal.NewFromInt(100),
		5,
	)

	mockAccountRepo.On("GetByID", ctx, "acc_customer_001").Return(account, nil)

	result, err := activities.GetAccountBalance(ctx, GetAccountBalanceInput{
		AccountID: "acc_customer_001",
	})

	require.NoError(t, err)
	require.True(t, result.Found)
	assert.True(t, result.LedgerBalance.Equal(decimal.NewFromInt(1000)))
	assert.True(t, result.PendingBalance.Equal(decimal.NewFromInt(100)))
	assert.True(t, result.AvailableBalance.Equal(decimal.NewFromInt(900)))
	assert.Equal(t, 5, result.Version)
}

func TestGetAccountBalance_NotFound(t *testing.T) {
	ctx := context.Background()
	mockAccountRepo := new(MockAccountRepository)

	activities := NewLedgerActivities(mockAccountRepo, nil, nil)

	mockAccountRepo.On("GetByID", ctx, "acc_nonexistent").Return(nil, nil)

	result, err := activities.GetAccountBalance(ctx, GetAccountBalanceInput{
		AccountID: "acc_nonexistent",
	})

	require.NoError(t, err)
	require.False(t, result.Found)
}

func TestPlaceAuthorizationHold_OptimisticLockFailure(t *testing.T) {
	ctx := context.Background()
	mockAccountRepo := new(MockAccountRepository)

	activities := NewLedgerActivities(mockAccountRepo, nil, nil)

	customerAccount := newTestAccount("acc_customer_001", "Customer Account", decimal.NewFromInt(1000), decimal.Zero, 1)

	mockAccountRepo.On("GetByID", ctx, "acc_customer_001").Return(customerAccount, nil)
	mockAccountRepo.On("UpdateBalances", ctx, "acc_customer_001",
		mock.Anything, mock.Anything, mock.Anything, 1,
	).Return(errors.New("optimistic lock conflict"))

	_, err := activities.PlaceAuthorizationHold(ctx, PlaceAuthorizationHoldInput{
		CustomerAccountID: "acc_customer_001",
		Amount:            decimal.NewFromInt(100),
		PaymentIntentID:   "pi_test_001",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "optimistic lock conflict")
}

func TestLedgerActivities_NilRepositories(t *testing.T) {
	ctx := context.Background()

	// Test that activities work gracefully without repositories (for testing)
	activities := NewLedgerActivities(nil, nil, nil)

	t.Run("PlaceAuthorizationHold without repo", func(t *testing.T) {
		result, err := activities.PlaceAuthorizationHold(ctx, PlaceAuthorizationHoldInput{
			CustomerAccountID: "acc_test",
			Amount:            decimal.NewFromInt(100),
		})
		require.NoError(t, err)
		assert.True(t, result.Success)
	})

	t.Run("ReleaseAuthorizationHold without repo", func(t *testing.T) {
		result, err := activities.ReleaseAuthorizationHold(ctx, ReleaseAuthorizationHoldInput{
			CustomerAccountID: "acc_test",
			Amount:            decimal.NewFromInt(100),
		})
		require.NoError(t, err)
		assert.True(t, result.Success)
	})

	t.Run("RecordCapture without repo", func(t *testing.T) {
		result, err := activities.RecordCapture(ctx, RecordCaptureInput{
			PaymentIntentID:   "pi_test",
			CustomerAccountID: "acc_test",
			Amount:            decimal.NewFromInt(100),
			Currency:          "USD",
		})
		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.NotEmpty(t, result.JournalEntryID)
	})

	t.Run("RecordRefund without repo", func(t *testing.T) {
		result, err := activities.RecordRefund(ctx, RecordRefundInput{
			PaymentIntentID:   "pi_test",
			RefundID:          "ref_test",
			CustomerAccountID: "acc_customer",
			MerchantAccountID: "acc_merchant",
			Amount:            decimal.NewFromInt(100),
			Currency:          "USD",
		})
		require.NoError(t, err)
		assert.True(t, result.Success)
	})

	t.Run("MonitorClearingAccounts without repo", func(t *testing.T) {
		result, err := activities.MonitorClearingAccounts(ctx, MonitorClearingAccountsInput{
			Currency: "USD",
		})
		require.NoError(t, err)
		assert.Empty(t, result.Alerts)
	})

	t.Run("GetAccountBalance without repo", func(t *testing.T) {
		result, err := activities.GetAccountBalance(ctx, GetAccountBalanceInput{
			AccountID: "acc_test",
		})
		require.NoError(t, err)
		assert.False(t, result.Found)
	})

	t.Run("GetClearingAccountBalances without repo", func(t *testing.T) {
		result, err := activities.GetClearingAccountBalances(ctx, GetClearingAccountBalancesInput{
			Currency: "USD",
		})
		require.NoError(t, err)
		assert.Empty(t, result.Accounts)
	})
}
