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
