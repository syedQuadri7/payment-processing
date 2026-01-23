package state

import (
	"context"
	"sync"
	"time"
)

// MemoryStore implements Store using in-memory maps.
type MemoryStore struct {
	mu       sync.RWMutex
	payments map[string]*PaymentState
	webhooks map[string]*WebhookRecord
	refunds  map[string]*RefundRecord

	// Index for provider ID lookups
	providerIndex map[string]string // "provider:providerID" -> id
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		payments:      make(map[string]*PaymentState),
		webhooks:      make(map[string]*WebhookRecord),
		refunds:       make(map[string]*RefundRecord),
		providerIndex: make(map[string]string),
	}
}

// CreatePayment stores a new payment.
func (s *MemoryStore) CreatePayment(ctx context.Context, payment *PaymentState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.payments[payment.ID]; exists {
		return ErrAlreadyExists
	}

	now := time.Now()
	payment.CreatedAt = now
	payment.UpdatedAt = now

	// Copy to avoid external mutations
	p := *payment
	s.payments[payment.ID] = &p

	// Index by provider ID
	if payment.ProviderID != "" {
		key := payment.Provider + ":" + payment.ProviderID
		s.providerIndex[key] = payment.ID
	}

	return nil
}

// GetPayment retrieves a payment by ID.
func (s *MemoryStore) GetPayment(ctx context.Context, id string) (*PaymentState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	payment, exists := s.payments[id]
	if !exists {
		return nil, ErrNotFound
	}

	// Return a copy
	p := *payment
	return &p, nil
}

// GetPaymentByProviderID retrieves a payment by provider and provider-specific ID.
func (s *MemoryStore) GetPaymentByProviderID(ctx context.Context, provider, providerID string) (*PaymentState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := provider + ":" + providerID
	id, exists := s.providerIndex[key]
	if !exists {
		return nil, ErrNotFound
	}

	payment, exists := s.payments[id]
	if !exists {
		return nil, ErrNotFound
	}

	p := *payment
	return &p, nil
}

// UpdatePayment updates an existing payment.
func (s *MemoryStore) UpdatePayment(ctx context.Context, payment *PaymentState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, exists := s.payments[payment.ID]
	if !exists {
		return ErrNotFound
	}

	payment.UpdatedAt = time.Now()
	payment.CreatedAt = existing.CreatedAt

	// Update provider index if changed
	if existing.ProviderID != payment.ProviderID {
		oldKey := existing.Provider + ":" + existing.ProviderID
		delete(s.providerIndex, oldKey)
		if payment.ProviderID != "" {
			newKey := payment.Provider + ":" + payment.ProviderID
			s.providerIndex[newKey] = payment.ID
		}
	}

	p := *payment
	s.payments[payment.ID] = &p
	return nil
}

// ListPayments returns payments matching the given options.
func (s *MemoryStore) ListPayments(ctx context.Context, opts ListOptions) ([]*PaymentState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*PaymentState
	for _, payment := range s.payments {
		// Apply filters
		if opts.Provider != "" && payment.Provider != opts.Provider {
			continue
		}
		if opts.Status != "" && string(payment.Status) != opts.Status {
			continue
		}

		p := *payment
		result = append(result, &p)
	}

	// Apply pagination
	if opts.Offset > 0 && opts.Offset < len(result) {
		result = result[opts.Offset:]
	} else if opts.Offset >= len(result) {
		return []*PaymentState{}, nil
	}

	if opts.Limit > 0 && opts.Limit < len(result) {
		result = result[:opts.Limit]
	}

	return result, nil
}

// DeletePayment removes a payment.
func (s *MemoryStore) DeletePayment(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	payment, exists := s.payments[id]
	if !exists {
		return ErrNotFound
	}

	// Remove from provider index
	if payment.ProviderID != "" {
		key := payment.Provider + ":" + payment.ProviderID
		delete(s.providerIndex, key)
	}

	delete(s.payments, id)
	return nil
}

// CreateWebhook stores a new webhook record.
func (s *MemoryStore) CreateWebhook(ctx context.Context, webhook *WebhookRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.webhooks[webhook.ID]; exists {
		return ErrAlreadyExists
	}

	webhook.CreatedAt = time.Now()
	w := *webhook
	s.webhooks[webhook.ID] = &w
	return nil
}

// GetWebhook retrieves a webhook by ID.
func (s *MemoryStore) GetWebhook(ctx context.Context, id string) (*WebhookRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	webhook, exists := s.webhooks[id]
	if !exists {
		return nil, ErrNotFound
	}

	w := *webhook
	return &w, nil
}

// UpdateWebhook updates an existing webhook.
func (s *MemoryStore) UpdateWebhook(ctx context.Context, webhook *WebhookRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.webhooks[webhook.ID]; !exists {
		return ErrNotFound
	}

	w := *webhook
	s.webhooks[webhook.ID] = &w
	return nil
}

// ListWebhooks returns webhooks matching the given options.
func (s *MemoryStore) ListWebhooks(ctx context.Context, opts ListOptions) ([]*WebhookRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*WebhookRecord
	for _, webhook := range s.webhooks {
		if opts.Status != "" && string(webhook.Status) != opts.Status {
			continue
		}

		w := *webhook
		result = append(result, &w)
	}

	// Apply pagination
	if opts.Offset > 0 && opts.Offset < len(result) {
		result = result[opts.Offset:]
	} else if opts.Offset >= len(result) {
		return []*WebhookRecord{}, nil
	}

	if opts.Limit > 0 && opts.Limit < len(result) {
		result = result[:opts.Limit]
	}

	return result, nil
}

// GetPendingWebhooks returns all webhooks pending delivery.
func (s *MemoryStore) GetPendingWebhooks(ctx context.Context) ([]*WebhookRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*WebhookRecord
	for _, webhook := range s.webhooks {
		if webhook.Status == WebhookPending || webhook.Status == WebhookRetrying {
			w := *webhook
			result = append(result, &w)
		}
	}
	return result, nil
}

// CreateRefund stores a new refund record.
func (s *MemoryStore) CreateRefund(ctx context.Context, refund *RefundRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.refunds[refund.ID]; exists {
		return ErrAlreadyExists
	}

	refund.CreatedAt = time.Now()
	r := *refund
	s.refunds[refund.ID] = &r
	return nil
}

// GetRefund retrieves a refund by ID.
func (s *MemoryStore) GetRefund(ctx context.Context, id string) (*RefundRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	refund, exists := s.refunds[id]
	if !exists {
		return nil, ErrNotFound
	}

	r := *refund
	return &r, nil
}

// ListRefundsByPayment returns all refunds for a payment.
func (s *MemoryStore) ListRefundsByPayment(ctx context.Context, paymentID string) ([]*RefundRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*RefundRecord
	for _, refund := range s.refunds {
		if refund.PaymentID == paymentID {
			r := *refund
			result = append(result, &r)
		}
	}
	return result, nil
}

// Reset clears all state.
func (s *MemoryStore) Reset(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.payments = make(map[string]*PaymentState)
	s.webhooks = make(map[string]*WebhookRecord)
	s.refunds = make(map[string]*RefundRecord)
	s.providerIndex = make(map[string]string)
	return nil
}

// Stats returns statistics about the store.
func (s *MemoryStore) Stats(ctx context.Context) (*StoreStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &StoreStats{
		TotalPayments:      len(s.payments),
		TotalWebhooks:      len(s.webhooks),
		TotalRefunds:       len(s.refunds),
		PaymentsByProvider: make(map[string]int),
		PaymentsByStatus:   make(map[string]int),
		WebhooksByStatus:   make(map[string]int),
	}

	for _, p := range s.payments {
		stats.PaymentsByProvider[p.Provider]++
		stats.PaymentsByStatus[string(p.Status)]++
	}

	for _, w := range s.webhooks {
		stats.WebhooksByStatus[string(w.Status)]++
	}

	return stats, nil
}

// Ensure MemoryStore implements Store
var _ Store = (*MemoryStore)(nil)
