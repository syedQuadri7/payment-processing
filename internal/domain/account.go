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
