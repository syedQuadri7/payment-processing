package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// ReferenceType represents the type of entity that originated a journal entry
type ReferenceType string

const (
	ReferenceTypePaymentIntent ReferenceType = "PAYMENT_INTENT"
	ReferenceTypeRefund        ReferenceType = "REFUND"
	ReferenceTypeAdjustment    ReferenceType = "ADJUSTMENT"
	ReferenceTypeFee           ReferenceType = "FEE"
	ReferenceTypeSettlement    ReferenceType = "SETTLEMENT"
)

// EntryDirection represents the direction of a ledger entry
type EntryDirection string

const (
	EntryDirectionDebit  EntryDirection = "DEBIT"
	EntryDirectionCredit EntryDirection = "CREDIT"
)

// JournalEntry groups related ledger entries into a logical transaction
// Every journal entry must balance (total debits = total credits)
type JournalEntry struct {
	ID            string
	Description   string
	ReferenceType *ReferenceType
	ReferenceID   *string
	PostedAt      time.Time
	CreatedAt     time.Time
	Entries       []LedgerEntry // Associated ledger entries
}

// LedgerEntry represents an individual debit or credit entry
// This is an append-only record - never updated or deleted
type LedgerEntry struct {
	ID             string
	JournalEntryID string
	AccountID      string
	Amount         decimal.Decimal
	Direction      EntryDirection
	BalanceAfter   decimal.Decimal // Running balance after this entry
	CreatedAt      time.Time
}

// IsBalanced checks if the journal entry is balanced (debits = credits)
func (je *JournalEntry) IsBalanced() bool {
	totalDebits := decimal.Zero
	totalCredits := decimal.Zero

	for _, entry := range je.Entries {
		if entry.Direction == EntryDirectionDebit {
			totalDebits = totalDebits.Add(entry.Amount)
		} else {
			totalCredits = totalCredits.Add(entry.Amount)
		}
	}

	return totalDebits.Equal(totalCredits)
}

// TotalDebits returns the sum of all debit entries
func (je *JournalEntry) TotalDebits() decimal.Decimal {
	total := decimal.Zero
	for _, entry := range je.Entries {
		if entry.Direction == EntryDirectionDebit {
			total = total.Add(entry.Amount)
		}
	}
	return total
}

// TotalCredits returns the sum of all credit entries
func (je *JournalEntry) TotalCredits() decimal.Decimal {
	total := decimal.Zero
	for _, entry := range je.Entries {
		if entry.Direction == EntryDirectionCredit {
			total = total.Add(entry.Amount)
		}
	}
	return total
}

// Imbalance returns the difference between debits and credits
// Returns zero if balanced, positive if more debits, negative if more credits
func (je *JournalEntry) Imbalance() decimal.Decimal {
	return je.TotalDebits().Sub(je.TotalCredits())
}

// Validate checks if the journal entry is valid
func (je *JournalEntry) Validate() error {
	if je.Description == "" {
		return ErrMissingDescription
	}
	if len(je.Entries) == 0 {
		return ErrNoEntries
	}
	if len(je.Entries) < 2 {
		return ErrMinimumTwoEntries
	}
	if !je.IsBalanced() {
		return &UnbalancedJournalError{
			Debits:  je.TotalDebits(),
			Credits: je.TotalCredits(),
		}
	}
	// Validate each entry
	for i, entry := range je.Entries {
		if entry.Amount.LessThanOrEqual(decimal.Zero) {
			return &InvalidEntryError{Index: i, Message: "amount must be positive"}
		}
		if entry.AccountID == "" {
			return &InvalidEntryError{Index: i, Message: "account_id is required"}
		}
		if entry.Direction != EntryDirectionDebit && entry.Direction != EntryDirectionCredit {
			return &InvalidEntryError{Index: i, Message: "direction must be DEBIT or CREDIT"}
		}
	}
	return nil
}

// AddDebit adds a debit entry to the journal
func (je *JournalEntry) AddDebit(accountID string, amount decimal.Decimal) {
	je.Entries = append(je.Entries, LedgerEntry{
		AccountID: accountID,
		Amount:    amount,
		Direction: EntryDirectionDebit,
		CreatedAt: time.Now(),
	})
}

// AddCredit adds a credit entry to the journal
func (je *JournalEntry) AddCredit(accountID string, amount decimal.Decimal) {
	je.Entries = append(je.Entries, LedgerEntry{
		AccountID: accountID,
		Amount:    amount,
		Direction: EntryDirectionCredit,
		CreatedAt: time.Now(),
	})
}

// JournalEntryBuilder helps construct valid journal entries
type JournalEntryBuilder struct {
	entry JournalEntry
}

// NewJournalEntry creates a new journal entry builder
func NewJournalEntry(description string) *JournalEntryBuilder {
	return &JournalEntryBuilder{
		entry: JournalEntry{
			Description: description,
			PostedAt:    time.Now(),
			CreatedAt:   time.Now(),
			Entries:     make([]LedgerEntry, 0),
		},
	}
}

// WithReference sets the reference type and ID
func (b *JournalEntryBuilder) WithReference(refType ReferenceType, refID string) *JournalEntryBuilder {
	b.entry.ReferenceType = &refType
	b.entry.ReferenceID = &refID
	return b
}

// Debit adds a debit entry
func (b *JournalEntryBuilder) Debit(accountID string, amount decimal.Decimal) *JournalEntryBuilder {
	b.entry.AddDebit(accountID, amount)
	return b
}

// Credit adds a credit entry
func (b *JournalEntryBuilder) Credit(accountID string, amount decimal.Decimal) *JournalEntryBuilder {
	b.entry.AddCredit(accountID, amount)
	return b
}

// Build validates and returns the journal entry
func (b *JournalEntryBuilder) Build() (*JournalEntry, error) {
	if err := b.entry.Validate(); err != nil {
		return nil, err
	}
	return &b.entry, nil
}

// MustBuild validates and returns the journal entry, panicking on error
func (b *JournalEntryBuilder) MustBuild() *JournalEntry {
	entry, err := b.Build()
	if err != nil {
		panic(err)
	}
	return entry
}

// NewCaptureJournalEntry creates a journal entry for a payment capture
func NewCaptureJournalEntry(paymentIntentID, customerAccountID, clearingAccountID string, amount decimal.Decimal) (*JournalEntry, error) {
	return NewJournalEntry("Capture payment "+paymentIntentID).
		WithReference(ReferenceTypePaymentIntent, paymentIntentID).
		Debit(customerAccountID, amount).
		Credit(clearingAccountID, amount).
		Build()
}

// NewRefundJournalEntry creates a journal entry for a refund
func NewRefundJournalEntry(paymentIntentID, merchantAccountID, customerAccountID string, amount decimal.Decimal) (*JournalEntry, error) {
	return NewJournalEntry("Refund for "+paymentIntentID).
		WithReference(ReferenceTypeRefund, paymentIntentID).
		Debit(merchantAccountID, amount).
		Credit(customerAccountID, amount).
		Build()
}

// NewSettlementJournalEntry creates a journal entry for settlement
func NewSettlementJournalEntry(batchID, clearingAccountID, merchantAccountID string, amount decimal.Decimal) (*JournalEntry, error) {
	return NewJournalEntry("Settlement batch "+batchID).
		WithReference(ReferenceTypeSettlement, batchID).
		Debit(clearingAccountID, amount).
		Credit(merchantAccountID, amount).
		Build()
}

// Ledger errors
var (
	ErrMissingDescription = &LedgerError{Message: "description is required"}
	ErrNoEntries          = &LedgerError{Message: "journal entry must have at least one entry"}
	ErrMinimumTwoEntries  = &LedgerError{Message: "journal entry must have at least two entries for double-entry"}
)

// LedgerError represents a general ledger error
type LedgerError struct {
	Message string
}

func (e *LedgerError) Error() string {
	return e.Message
}

// UnbalancedJournalError indicates a journal entry that doesn't balance
type UnbalancedJournalError struct {
	Debits  decimal.Decimal
	Credits decimal.Decimal
}

func (e *UnbalancedJournalError) Error() string {
	return "journal entry is not balanced: debits=" + e.Debits.String() + " credits=" + e.Credits.String()
}

// InvalidEntryError indicates an invalid ledger entry
type InvalidEntryError struct {
	Index   int
	Message string
}

func (e *InvalidEntryError) Error() string {
	return "invalid entry at index " + string(rune('0'+e.Index)) + ": " + e.Message
}
