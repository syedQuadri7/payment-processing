package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestAccount_IsSystemAccount(t *testing.T) {
	tests := []struct {
		name     string
		ownerID  *string
		expected bool
	}{
		{
			name:     "system account has nil owner",
			ownerID:  nil,
			expected: true,
		},
		{
			name:     "customer account has owner",
			ownerID:  strPtr("customer-123"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := &Account{OwnerID: tt.ownerID}
			if got := account.IsSystemAccount(); got != tt.expected {
				t.Errorf("IsSystemAccount() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAccount_CalculateAvailableBalance(t *testing.T) {
	tests := []struct {
		name            string
		ledgerBalance   string
		pendingBalance  string
		reservedBalance string
		expected        string
	}{
		{
			name:            "all zeros",
			ledgerBalance:   "0",
			pendingBalance:  "0",
			reservedBalance: "0",
			expected:        "0",
		},
		{
			name:            "ledger only",
			ledgerBalance:   "1000",
			pendingBalance:  "0",
			reservedBalance: "0",
			expected:        "1000",
		},
		{
			name:            "with pending",
			ledgerBalance:   "1000",
			pendingBalance:  "200",
			reservedBalance: "0",
			expected:        "800",
		},
		{
			name:            "with pending and reserved",
			ledgerBalance:   "1000",
			pendingBalance:  "200",
			reservedBalance: "100",
			expected:        "700",
		},
		{
			name:            "negative available (overdraft)",
			ledgerBalance:   "100",
			pendingBalance:  "150",
			reservedBalance: "0",
			expected:        "-50",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := &Account{
				LedgerBalance:   decimal.RequireFromString(tt.ledgerBalance),
				PendingBalance:  decimal.RequireFromString(tt.pendingBalance),
				ReservedBalance: decimal.RequireFromString(tt.reservedBalance),
			}
			expected := decimal.RequireFromString(tt.expected)
			if got := account.CalculateAvailableBalance(); !got.Equal(expected) {
				t.Errorf("CalculateAvailableBalance() = %v, want %v", got, expected)
			}
		})
	}
}

func TestAccount_CanDebit(t *testing.T) {
	tests := []struct {
		name             string
		status           AccountStatus
		availableBalance string
		debitAmount      string
		expected         bool
	}{
		{
			name:             "sufficient balance active account",
			status:           AccountStatusActive,
			availableBalance: "1000",
			debitAmount:      "500",
			expected:         true,
		},
		{
			name:             "exact balance",
			status:           AccountStatusActive,
			availableBalance: "500",
			debitAmount:      "500",
			expected:         true,
		},
		{
			name:             "insufficient balance",
			status:           AccountStatusActive,
			availableBalance: "100",
			debitAmount:      "500",
			expected:         false,
		},
		{
			name:             "frozen account",
			status:           AccountStatusFrozen,
			availableBalance: "1000",
			debitAmount:      "100",
			expected:         false,
		},
		{
			name:             "closed account",
			status:           AccountStatusClosed,
			availableBalance: "1000",
			debitAmount:      "100",
			expected:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := &Account{
				Status:           tt.status,
				AvailableBalance: decimal.RequireFromString(tt.availableBalance),
			}
			amount := decimal.RequireFromString(tt.debitAmount)
			if got := account.CanDebit(amount); got != tt.expected {
				t.Errorf("CanDebit() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAccount_IsNormalDebitCredit(t *testing.T) {
	tests := []struct {
		accountType    AccountType
		isNormalDebit  bool
		isNormalCredit bool
	}{
		{AccountTypeAsset, true, false},
		{AccountTypeExpense, true, false},
		{AccountTypeLiability, false, true},
		{AccountTypeEquity, false, true},
		{AccountTypeRevenue, false, true},
		// Clearing accounts are neither - they should net to zero
		{AccountTypeClearing, false, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.accountType), func(t *testing.T) {
			account := &Account{Type: tt.accountType}
			if got := account.IsNormalDebit(); got != tt.isNormalDebit {
				t.Errorf("IsNormalDebit() = %v, want %v", got, tt.isNormalDebit)
			}
			if got := account.IsNormalCredit(); got != tt.isNormalCredit {
				t.Errorf("IsNormalCredit() = %v, want %v", got, tt.isNormalCredit)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}
