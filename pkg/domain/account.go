package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// AccountType represents the type of account
type AccountType string

const (
	AccountTypeAsset     AccountType = "ASSET"
	AccountTypeLiability AccountType = "LIABILITY"
	AccountTypeEquity    AccountType = "EQUITY"
	AccountTypeRevenue   AccountType = "REVENUE"
	AccountTypeExpense   AccountType = "EXPENSE"
	AccountTypeClearing  AccountType = "CLEARING"
)

// AccountStatus represents the status of an account
type AccountStatus string

const (
	AccountStatusActive AccountStatus = "ACTIVE"
	AccountStatusFrozen AccountStatus = "FROZEN"
	AccountStatusClosed AccountStatus = "CLOSED"
)

// Account represents a ledger account for double-entry bookkeeping
type Account struct {
	ID               string
	Type             AccountType
	OwnerID          *string // Null for system accounts
	Name             string
	Currency         string
	LedgerBalance    decimal.Decimal
	PendingBalance   decimal.Decimal
	AvailableBalance decimal.Decimal
	ReservedBalance  decimal.Decimal
	DailyLimit       *decimal.Decimal
	Status           AccountStatus
	Version          int // Optimistic locking version
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// IsSystemAccount returns true if this is an internal system account
func (a *Account) IsSystemAccount() bool {
	return a.OwnerID == nil
}

// CalculateAvailableBalance computes available balance from ledger and pending
func (a *Account) CalculateAvailableBalance() decimal.Decimal {
	return a.LedgerBalance.Sub(a.PendingBalance).Sub(a.ReservedBalance)
}

// CanDebit checks if the account can be debited for the given amount
func (a *Account) CanDebit(amount decimal.Decimal) bool {
	if a.Status != AccountStatusActive {
		return false
	}
	return a.AvailableBalance.GreaterThanOrEqual(amount)
}

// IsNormalDebit returns true if this account type normally has a debit balance
func (a *Account) IsNormalDebit() bool {
	return a.Type == AccountTypeAsset || a.Type == AccountTypeExpense
}

// IsNormalCredit returns true if this account type normally has a credit balance
func (a *Account) IsNormalCredit() bool {
	return a.Type == AccountTypeLiability || a.Type == AccountTypeEquity || a.Type == AccountTypeRevenue
}

// PlaceHold increases the pending balance (for authorization)
// This decreases available balance but doesn't affect ledger balance
func (a *Account) PlaceHold(amount decimal.Decimal) error {
	if a.Status != AccountStatusActive {
		return ErrAccountNotActive
	}
	if amount.LessThanOrEqual(decimal.Zero) {
		return ErrInvalidHoldAmount
	}
	if !a.CanDebit(amount) {
		return ErrInsufficientFunds
	}

	a.PendingBalance = a.PendingBalance.Add(amount)
	a.AvailableBalance = a.CalculateAvailableBalance()
	a.UpdatedAt = time.Now()
	a.Version++

	return nil
}

// ReleaseHold decreases the pending balance (for void or expiration)
// This increases available balance
func (a *Account) ReleaseHold(amount decimal.Decimal) error {
	if amount.LessThanOrEqual(decimal.Zero) {
		return ErrInvalidHoldAmount
	}
	if amount.GreaterThan(a.PendingBalance) {
		return ErrHoldExceedsPending
	}

	a.PendingBalance = a.PendingBalance.Sub(amount)
	a.AvailableBalance = a.CalculateAvailableBalance()
	a.UpdatedAt = time.Now()
	a.Version++

	return nil
}

// Debit decreases the ledger balance (for capture)
// Also releases the corresponding pending balance
func (a *Account) Debit(amount decimal.Decimal) error {
	if a.Status != AccountStatusActive {
		return ErrAccountNotActive
	}
	if amount.LessThanOrEqual(decimal.Zero) {
		return ErrInvalidDebitAmount
	}

	a.LedgerBalance = a.LedgerBalance.Sub(amount)

	// Release pending if there's a corresponding hold
	if a.PendingBalance.GreaterThanOrEqual(amount) {
		a.PendingBalance = a.PendingBalance.Sub(amount)
	}

	a.AvailableBalance = a.CalculateAvailableBalance()
	a.UpdatedAt = time.Now()
	a.Version++

	return nil
}

// Credit increases the ledger balance (for refund or deposit)
func (a *Account) Credit(amount decimal.Decimal) error {
	if a.Status != AccountStatusActive {
		return ErrAccountNotActive
	}
	if amount.LessThanOrEqual(decimal.Zero) {
		return ErrInvalidCreditAmount
	}

	a.LedgerBalance = a.LedgerBalance.Add(amount)
	a.AvailableBalance = a.CalculateAvailableBalance()
	a.UpdatedAt = time.Now()
	a.Version++

	return nil
}

// Reserve increases the reserved balance (for scheduled payments)
func (a *Account) Reserve(amount decimal.Decimal) error {
	if a.Status != AccountStatusActive {
		return ErrAccountNotActive
	}
	if amount.LessThanOrEqual(decimal.Zero) {
		return ErrInvalidReserveAmount
	}
	if !a.CanDebit(amount) {
		return ErrInsufficientFunds
	}

	a.ReservedBalance = a.ReservedBalance.Add(amount)
	a.AvailableBalance = a.CalculateAvailableBalance()
	a.UpdatedAt = time.Now()
	a.Version++

	return nil
}

// ReleaseReserve decreases the reserved balance
func (a *Account) ReleaseReserve(amount decimal.Decimal) error {
	if amount.LessThanOrEqual(decimal.Zero) {
		return ErrInvalidReserveAmount
	}
	if amount.GreaterThan(a.ReservedBalance) {
		return ErrReserveExceedsBalance
	}

	a.ReservedBalance = a.ReservedBalance.Sub(amount)
	a.AvailableBalance = a.CalculateAvailableBalance()
	a.UpdatedAt = time.Now()
	a.Version++

	return nil
}

// Freeze freezes the account, preventing new transactions
func (a *Account) Freeze() error {
	if a.Status == AccountStatusClosed {
		return ErrAccountClosed
	}
	a.Status = AccountStatusFrozen
	a.UpdatedAt = time.Now()
	a.Version++
	return nil
}

// Unfreeze unfreezes a frozen account
func (a *Account) Unfreeze() error {
	if a.Status != AccountStatusFrozen {
		return ErrAccountNotFrozen
	}
	a.Status = AccountStatusActive
	a.UpdatedAt = time.Now()
	a.Version++
	return nil
}

// Close closes the account
func (a *Account) Close() error {
	if a.Status == AccountStatusClosed {
		return ErrAccountAlreadyClosed
	}
	// Accounts with non-zero balances cannot be closed
	if !a.LedgerBalance.IsZero() || !a.PendingBalance.IsZero() || !a.ReservedBalance.IsZero() {
		return ErrAccountHasBalance
	}
	a.Status = AccountStatusClosed
	a.UpdatedAt = time.Now()
	a.Version++
	return nil
}

// ExceedsDailyLimit checks if the given amount would exceed the daily limit
func (a *Account) ExceedsDailyLimit(amount decimal.Decimal) bool {
	if a.DailyLimit == nil {
		return false
	}
	return amount.GreaterThan(*a.DailyLimit)
}

// Account errors
var (
	ErrAccountNotActive     = &AccountError{Message: "account is not active"}
	ErrAccountNotFrozen     = &AccountError{Message: "account is not frozen"}
	ErrAccountClosed        = &AccountError{Message: "account is closed"}
	ErrAccountAlreadyClosed = &AccountError{Message: "account is already closed"}
	ErrAccountHasBalance    = &AccountError{Message: "account has non-zero balance"}
	ErrInsufficientFunds    = &AccountError{Message: "insufficient funds"}
	ErrInvalidHoldAmount    = &AccountError{Message: "invalid hold amount"}
	ErrInvalidDebitAmount   = &AccountError{Message: "invalid debit amount"}
	ErrInvalidCreditAmount  = &AccountError{Message: "invalid credit amount"}
	ErrInvalidReserveAmount = &AccountError{Message: "invalid reserve amount"}
	ErrHoldExceedsPending   = &AccountError{Message: "hold amount exceeds pending balance"}
	ErrReserveExceedsBalance = &AccountError{Message: "reserve amount exceeds reserved balance"}
)

// AccountError represents an account operation error
type AccountError struct {
	Message string
}

func (e *AccountError) Error() string {
	return e.Message
}
