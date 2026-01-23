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

func TestJournalEntry_Validate(t *testing.T) {
	tests := []struct {
		name        string
		entry       JournalEntry
		expectError bool
	}{
		{
			name: "valid balanced entry",
			entry: JournalEntry{
				Description: "Test entry",
				Entries: []LedgerEntry{
					{AccountID: "acc-1", Amount: decimal.RequireFromString("100"), Direction: EntryDirectionDebit},
					{AccountID: "acc-2", Amount: decimal.RequireFromString("100"), Direction: EntryDirectionCredit},
				},
			},
			expectError: false,
		},
		{
			name: "missing description",
			entry: JournalEntry{
				Description: "",
				Entries: []LedgerEntry{
					{AccountID: "acc-1", Amount: decimal.RequireFromString("100"), Direction: EntryDirectionDebit},
					{AccountID: "acc-2", Amount: decimal.RequireFromString("100"), Direction: EntryDirectionCredit},
				},
			},
			expectError: true,
		},
		{
			name: "no entries",
			entry: JournalEntry{
				Description: "Test",
				Entries:     []LedgerEntry{},
			},
			expectError: true,
		},
		{
			name: "only one entry",
			entry: JournalEntry{
				Description: "Test",
				Entries: []LedgerEntry{
					{AccountID: "acc-1", Amount: decimal.RequireFromString("100"), Direction: EntryDirectionDebit},
				},
			},
			expectError: true,
		},
		{
			name: "unbalanced entry",
			entry: JournalEntry{
				Description: "Test",
				Entries: []LedgerEntry{
					{AccountID: "acc-1", Amount: decimal.RequireFromString("100"), Direction: EntryDirectionDebit},
					{AccountID: "acc-2", Amount: decimal.RequireFromString("50"), Direction: EntryDirectionCredit},
				},
			},
			expectError: true,
		},
		{
			name: "zero amount entry",
			entry: JournalEntry{
				Description: "Test",
				Entries: []LedgerEntry{
					{AccountID: "acc-1", Amount: decimal.Zero, Direction: EntryDirectionDebit},
					{AccountID: "acc-2", Amount: decimal.Zero, Direction: EntryDirectionCredit},
				},
			},
			expectError: true,
		},
		{
			name: "missing account ID",
			entry: JournalEntry{
				Description: "Test",
				Entries: []LedgerEntry{
					{AccountID: "", Amount: decimal.RequireFromString("100"), Direction: EntryDirectionDebit},
					{AccountID: "acc-2", Amount: decimal.RequireFromString("100"), Direction: EntryDirectionCredit},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.entry.Validate()
			if (err != nil) != tt.expectError {
				t.Errorf("Validate() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestJournalEntryBuilder(t *testing.T) {
	t.Run("build valid entry", func(t *testing.T) {
		entry, err := NewJournalEntry("Test capture").
			WithReference(ReferenceTypePaymentIntent, "pi-123").
			Debit("customer-account", decimal.RequireFromString("100")).
			Credit("clearing-account", decimal.RequireFromString("100")).
			Build()

		if err != nil {
			t.Fatalf("Build() error = %v", err)
		}
		if entry.Description != "Test capture" {
			t.Errorf("Description = %v, want %v", entry.Description, "Test capture")
		}
		if *entry.ReferenceType != ReferenceTypePaymentIntent {
			t.Errorf("ReferenceType = %v, want %v", *entry.ReferenceType, ReferenceTypePaymentIntent)
		}
		if len(entry.Entries) != 2 {
			t.Errorf("Entries count = %v, want 2", len(entry.Entries))
		}
	})

	t.Run("build unbalanced entry fails", func(t *testing.T) {
		_, err := NewJournalEntry("Test").
			Debit("acc-1", decimal.RequireFromString("100")).
			Credit("acc-2", decimal.RequireFromString("50")).
			Build()

		if err == nil {
			t.Error("Build() expected error for unbalanced entry")
		}
	})
}

func TestNewCaptureJournalEntry(t *testing.T) {
	entry, err := NewCaptureJournalEntry("pi-123", "customer-acc", "clearing-acc", decimal.RequireFromString("100"))
	if err != nil {
		t.Fatalf("NewCaptureJournalEntry() error = %v", err)
	}

	if !entry.IsBalanced() {
		t.Error("Expected balanced entry")
	}
	if *entry.ReferenceType != ReferenceTypePaymentIntent {
		t.Errorf("ReferenceType = %v, want %v", *entry.ReferenceType, ReferenceTypePaymentIntent)
	}
}

func TestNewRefundJournalEntry(t *testing.T) {
	entry, err := NewRefundJournalEntry("pi-123", "merchant-acc", "customer-acc", decimal.RequireFromString("50"))
	if err != nil {
		t.Fatalf("NewRefundJournalEntry() error = %v", err)
	}

	if !entry.IsBalanced() {
		t.Error("Expected balanced entry")
	}
	if *entry.ReferenceType != ReferenceTypeRefund {
		t.Errorf("ReferenceType = %v, want %v", *entry.ReferenceType, ReferenceTypeRefund)
	}
}
