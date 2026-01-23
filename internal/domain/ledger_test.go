package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestJournalEntry_IsBalanced(t *testing.T) {
	tests := []struct {
		name     string
		entries  []LedgerEntry
		expected bool
	}{
		{
			name:     "empty journal is balanced",
			entries:  []LedgerEntry{},
			expected: true,
		},
		{
			name: "single debit and credit balanced",
			entries: []LedgerEntry{
				{Amount: decimal.RequireFromString("100"), Direction: EntryDirectionDebit},
				{Amount: decimal.RequireFromString("100"), Direction: EntryDirectionCredit},
			},
			expected: true,
		},
		{
			name: "multiple entries balanced",
			entries: []LedgerEntry{
				{Amount: decimal.RequireFromString("100"), Direction: EntryDirectionDebit},
				{Amount: decimal.RequireFromString("50"), Direction: EntryDirectionDebit},
				{Amount: decimal.RequireFromString("150"), Direction: EntryDirectionCredit},
			},
			expected: true,
		},
		{
			name: "unbalanced - more debits",
			entries: []LedgerEntry{
				{Amount: decimal.RequireFromString("100"), Direction: EntryDirectionDebit},
				{Amount: decimal.RequireFromString("50"), Direction: EntryDirectionCredit},
			},
			expected: false,
		},
		{
			name: "unbalanced - more credits",
			entries: []LedgerEntry{
				{Amount: decimal.RequireFromString("50"), Direction: EntryDirectionDebit},
				{Amount: decimal.RequireFromString("100"), Direction: EntryDirectionCredit},
			},
			expected: false,
		},
		{
			name: "complex balanced entry",
			entries: []LedgerEntry{
				{Amount: decimal.RequireFromString("1000.50"), Direction: EntryDirectionDebit},
				{Amount: decimal.RequireFromString("500.25"), Direction: EntryDirectionCredit},
				{Amount: decimal.RequireFromString("500.25"), Direction: EntryDirectionCredit},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			je := &JournalEntry{Entries: tt.entries}
			if got := je.IsBalanced(); got != tt.expected {
				t.Errorf("IsBalanced() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestJournalEntry_TotalDebits(t *testing.T) {
	tests := []struct {
		name     string
		entries  []LedgerEntry
		expected string
	}{
		{
			name:     "empty entries",
			entries:  []LedgerEntry{},
			expected: "0",
		},
		{
			name: "single debit",
			entries: []LedgerEntry{
				{Amount: decimal.RequireFromString("100"), Direction: EntryDirectionDebit},
			},
			expected: "100",
		},
		{
			name: "multiple debits",
			entries: []LedgerEntry{
				{Amount: decimal.RequireFromString("100"), Direction: EntryDirectionDebit},
				{Amount: decimal.RequireFromString("50.75"), Direction: EntryDirectionDebit},
			},
			expected: "150.75",
		},
		{
			name: "mixed entries",
			entries: []LedgerEntry{
				{Amount: decimal.RequireFromString("100"), Direction: EntryDirectionDebit},
				{Amount: decimal.RequireFromString("200"), Direction: EntryDirectionCredit},
				{Amount: decimal.RequireFromString("50"), Direction: EntryDirectionDebit},
			},
			expected: "150",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			je := &JournalEntry{Entries: tt.entries}
			expected := decimal.RequireFromString(tt.expected)
			if got := je.TotalDebits(); !got.Equal(expected) {
				t.Errorf("TotalDebits() = %v, want %v", got, expected)
			}
		})
	}
}

func TestJournalEntry_TotalCredits(t *testing.T) {
	tests := []struct {
		name     string
		entries  []LedgerEntry
		expected string
	}{
		{
			name:     "empty entries",
			entries:  []LedgerEntry{},
			expected: "0",
		},
		{
			name: "single credit",
			entries: []LedgerEntry{
				{Amount: decimal.RequireFromString("100"), Direction: EntryDirectionCredit},
			},
			expected: "100",
		},
		{
			name: "multiple credits",
			entries: []LedgerEntry{
				{Amount: decimal.RequireFromString("100"), Direction: EntryDirectionCredit},
				{Amount: decimal.RequireFromString("50.25"), Direction: EntryDirectionCredit},
			},
			expected: "150.25",
		},
		{
			name: "mixed entries",
			entries: []LedgerEntry{
				{Amount: decimal.RequireFromString("100"), Direction: EntryDirectionDebit},
				{Amount: decimal.RequireFromString("200"), Direction: EntryDirectionCredit},
				{Amount: decimal.RequireFromString("50"), Direction: EntryDirectionCredit},
			},
			expected: "250",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			je := &JournalEntry{Entries: tt.entries}
			expected := decimal.RequireFromString(tt.expected)
			if got := je.TotalCredits(); !got.Equal(expected) {
				t.Errorf("TotalCredits() = %v, want %v", got, expected)
			}
		})
	}
}
