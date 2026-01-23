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

func TestAccount_PlaceHold(t *testing.T) {
	tests := []struct {
		name             string
		status           AccountStatus
		ledgerBalance    string
		availableBalance string
		holdAmount       string
		expectError      bool
	}{
		{
			name:             "successful hold",
			status:           AccountStatusActive,
			ledgerBalance:    "1000",
			availableBalance: "1000",
			holdAmount:       "200",
			expectError:      false,
		},
		{
			name:             "insufficient funds",
			status:           AccountStatusActive,
			ledgerBalance:    "100",
			availableBalance: "100",
			holdAmount:       "200",
			expectError:      true,
		},
		{
			name:             "frozen account",
			status:           AccountStatusFrozen,
			ledgerBalance:    "1000",
			availableBalance: "1000",
			holdAmount:       "100",
			expectError:      true,
		},
		{
			name:             "zero amount",
			status:           AccountStatusActive,
			ledgerBalance:    "1000",
			availableBalance: "1000",
			holdAmount:       "0",
			expectError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := &Account{
				Status:           tt.status,
				LedgerBalance:    decimal.RequireFromString(tt.ledgerBalance),
				AvailableBalance: decimal.RequireFromString(tt.availableBalance),
				PendingBalance:   decimal.Zero,
				ReservedBalance:  decimal.Zero,
			}
			amount := decimal.RequireFromString(tt.holdAmount)
			err := account.PlaceHold(amount)

			if (err != nil) != tt.expectError {
				t.Errorf("PlaceHold() error = %v, expectError %v", err, tt.expectError)
			}

			if err == nil {
				if !account.PendingBalance.Equal(amount) {
					t.Errorf("PendingBalance = %v, want %v", account.PendingBalance, amount)
				}
			}
		})
	}
}

func TestAccount_ReleaseHold(t *testing.T) {
	tests := []struct {
		name           string
		pendingBalance string
		releaseAmount  string
		expectError    bool
	}{
		{
			name:           "full release",
			pendingBalance: "200",
			releaseAmount:  "200",
			expectError:    false,
		},
		{
			name:           "partial release",
			pendingBalance: "200",
			releaseAmount:  "100",
			expectError:    false,
		},
		{
			name:           "release exceeds pending",
			pendingBalance: "100",
			releaseAmount:  "200",
			expectError:    true,
		},
		{
			name:           "zero release",
			pendingBalance: "200",
			releaseAmount:  "0",
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := &Account{
				Status:           AccountStatusActive,
				LedgerBalance:    decimal.RequireFromString("1000"),
				PendingBalance:   decimal.RequireFromString(tt.pendingBalance),
				AvailableBalance: decimal.RequireFromString("800"),
				ReservedBalance:  decimal.Zero,
			}
			amount := decimal.RequireFromString(tt.releaseAmount)
			err := account.ReleaseHold(amount)

			if (err != nil) != tt.expectError {
				t.Errorf("ReleaseHold() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestAccount_Debit(t *testing.T) {
	tests := []struct {
		name          string
		status        AccountStatus
		ledgerBalance string
		debitAmount   string
		expectError   bool
	}{
		{
			name:          "successful debit",
			status:        AccountStatusActive,
			ledgerBalance: "1000",
			debitAmount:   "200",
			expectError:   false,
		},
		{
			name:          "frozen account",
			status:        AccountStatusFrozen,
			ledgerBalance: "1000",
			debitAmount:   "200",
			expectError:   true,
		},
		{
			name:          "zero debit",
			status:        AccountStatusActive,
			ledgerBalance: "1000",
			debitAmount:   "0",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := &Account{
				Status:           tt.status,
				LedgerBalance:    decimal.RequireFromString(tt.ledgerBalance),
				PendingBalance:   decimal.Zero,
				AvailableBalance: decimal.RequireFromString(tt.ledgerBalance),
				ReservedBalance:  decimal.Zero,
			}
			amount := decimal.RequireFromString(tt.debitAmount)
			err := account.Debit(amount)

			if (err != nil) != tt.expectError {
				t.Errorf("Debit() error = %v, expectError %v", err, tt.expectError)
			}

			if err == nil {
				expected := decimal.RequireFromString(tt.ledgerBalance).Sub(amount)
				if !account.LedgerBalance.Equal(expected) {
					t.Errorf("LedgerBalance = %v, want %v", account.LedgerBalance, expected)
				}
			}
		})
	}
}

func TestAccount_Credit(t *testing.T) {
	tests := []struct {
		name          string
		status        AccountStatus
		ledgerBalance string
		creditAmount  string
		expectError   bool
	}{
		{
			name:          "successful credit",
			status:        AccountStatusActive,
			ledgerBalance: "1000",
			creditAmount:  "200",
			expectError:   false,
		},
		{
			name:          "frozen account",
			status:        AccountStatusFrozen,
			ledgerBalance: "1000",
			creditAmount:  "200",
			expectError:   true,
		},
		{
			name:          "zero credit",
			status:        AccountStatusActive,
			ledgerBalance: "1000",
			creditAmount:  "0",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := &Account{
				Status:           tt.status,
				LedgerBalance:    decimal.RequireFromString(tt.ledgerBalance),
				PendingBalance:   decimal.Zero,
				AvailableBalance: decimal.RequireFromString(tt.ledgerBalance),
				ReservedBalance:  decimal.Zero,
			}
			amount := decimal.RequireFromString(tt.creditAmount)
			err := account.Credit(amount)

			if (err != nil) != tt.expectError {
				t.Errorf("Credit() error = %v, expectError %v", err, tt.expectError)
			}

			if err == nil {
				expected := decimal.RequireFromString(tt.ledgerBalance).Add(amount)
				if !account.LedgerBalance.Equal(expected) {
					t.Errorf("LedgerBalance = %v, want %v", account.LedgerBalance, expected)
				}
			}
		})
	}
}

func TestAccount_FreezeUnfreeze(t *testing.T) {
	t.Run("freeze active account", func(t *testing.T) {
		account := &Account{Status: AccountStatusActive}
		if err := account.Freeze(); err != nil {
			t.Errorf("Freeze() error = %v", err)
		}
		if account.Status != AccountStatusFrozen {
			t.Errorf("Status = %v, want %v", account.Status, AccountStatusFrozen)
		}
	})

	t.Run("unfreeze frozen account", func(t *testing.T) {
		account := &Account{Status: AccountStatusFrozen}
		if err := account.Unfreeze(); err != nil {
			t.Errorf("Unfreeze() error = %v", err)
		}
		if account.Status != AccountStatusActive {
			t.Errorf("Status = %v, want %v", account.Status, AccountStatusActive)
		}
	})

	t.Run("cannot unfreeze active account", func(t *testing.T) {
		account := &Account{Status: AccountStatusActive}
		if err := account.Unfreeze(); err == nil {
			t.Error("Unfreeze() expected error for active account")
		}
	})
}

func TestAccount_Close(t *testing.T) {
	t.Run("close account with zero balance", func(t *testing.T) {
		account := &Account{
			Status:          AccountStatusActive,
			LedgerBalance:   decimal.Zero,
			PendingBalance:  decimal.Zero,
			ReservedBalance: decimal.Zero,
		}
		if err := account.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
		if account.Status != AccountStatusClosed {
			t.Errorf("Status = %v, want %v", account.Status, AccountStatusClosed)
		}
	})

	t.Run("cannot close account with balance", func(t *testing.T) {
		account := &Account{
			Status:          AccountStatusActive,
			LedgerBalance:   decimal.NewFromInt(100),
			PendingBalance:  decimal.Zero,
			ReservedBalance: decimal.Zero,
		}
		if err := account.Close(); err == nil {
			t.Error("Close() expected error for account with balance")
		}
	})
}
