package outbox

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"payment-processing/pkg/domain"
)

// mockOutboxRepo implements OutboxRepository for testing
type mockOutboxRepo struct {
	mu       sync.Mutex
	events   []*domain.OutboxEvent
	deleted  []string
	cleanups []time.Time
}

func newMockOutboxRepo() *mockOutboxRepo {
	return &mockOutboxRepo{
		events:  make([]*domain.OutboxEvent, 0),
		deleted: make([]string, 0),
	}
}

func (m *mockOutboxRepo) addEvent(event *domain.OutboxEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
}

func (m *mockOutboxRepo) GetUnpublished(ctx context.Context, limit int) ([]*domain.OutboxEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.events) == 0 {
		return nil, nil
	}

	// Return up to limit events
	n := len(m.events)
	if n > limit {
		n = limit
	}
	result := make([]*domain.OutboxEvent, n)
	copy(result, m.events[:n])
	return result, nil
}

func (m *mockOutboxRepo) MarkPublished(ctx context.Context, ids []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.deleted = append(m.deleted, ids...)

	// Remove events from the list
	remaining := make([]*domain.OutboxEvent, 0)
	deleteSet := make(map[string]bool)
	for _, id := range ids {
		deleteSet[id] = true
	}
	for _, e := range m.events {
		if !deleteSet[e.ID] {
			remaining = append(remaining, e)
		}
	}
	m.events = remaining

	return nil
}

func (m *mockOutboxRepo) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanups = append(m.cleanups, before)

	// Remove old events
	remaining := make([]*domain.OutboxEvent, 0)
	deleted := int64(0)
	for _, e := range m.events {
		if e.CreatedAt.Before(before) {
			deleted++
		} else {
			remaining = append(remaining, e)
		}
	}
	m.events = remaining

	return deleted, nil
}

func TestConsumer_ProcessesEvents(t *testing.T) {
	repo := newMockOutboxRepo()

	// Add test events
	payload, _ := json.Marshal(map[string]string{"key": "value"})
	repo.addEvent(&domain.OutboxEvent{
		ID:            "event-1",
		AggregateType: domain.AggregateTypePaymentIntent,
		AggregateID:   "pi_123",
		EventType:     domain.OutboxEventPaymentAuthorized,
		Payload:       payload,
		CreatedAt:     time.Now(),
	})

	var processed []string
	var mu sync.Mutex
	handler := func(ctx context.Context, event *domain.OutboxEvent) error {
		mu.Lock()
		processed = append(processed, event.ID)
		mu.Unlock()
		return nil
	}

	config := DefaultConsumerConfig()
	config.PollInterval = 10 * time.Millisecond
	consumer := NewConsumer(repo, handler, config)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if err := consumer.Start(ctx); err != nil {
		t.Fatalf("Failed to start consumer: %v", err)
	}

	// Wait for processing
	time.Sleep(50 * time.Millisecond)
	consumer.Stop()

	mu.Lock()
	if len(processed) != 1 {
		t.Errorf("Expected 1 processed event, got %d", len(processed))
	}
	if len(processed) > 0 && processed[0] != "event-1" {
		t.Errorf("Expected event-1, got %s", processed[0])
	}
	mu.Unlock()

	// Verify event was marked as published
	repo.mu.Lock()
	if len(repo.deleted) != 1 || repo.deleted[0] != "event-1" {
		t.Errorf("Expected event-1 to be marked published, got %v", repo.deleted)
	}
	repo.mu.Unlock()
}

func TestConsumer_BatchProcessing(t *testing.T) {
	repo := newMockOutboxRepo()

	// Add multiple events
	for i := 0; i < 5; i++ {
		payload, _ := json.Marshal(map[string]int{"index": i})
		repo.addEvent(&domain.OutboxEvent{
			ID:            "event-" + string(rune('a'+i)),
			AggregateType: domain.AggregateTypePaymentIntent,
			AggregateID:   "pi_123",
			EventType:     domain.OutboxEventPaymentAuthorized,
			Payload:       payload,
			CreatedAt:     time.Now(),
		})
	}

	var processedCount int
	var mu sync.Mutex
	handler := func(ctx context.Context, event *domain.OutboxEvent) error {
		mu.Lock()
		processedCount++
		mu.Unlock()
		return nil
	}

	config := DefaultConsumerConfig()
	config.PollInterval = 10 * time.Millisecond
	config.BatchSize = 3 // Process 3 at a time
	consumer := NewConsumer(repo, handler, config)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	if err := consumer.Start(ctx); err != nil {
		t.Fatalf("Failed to start consumer: %v", err)
	}

	// Wait for all processing
	time.Sleep(100 * time.Millisecond)
	consumer.Stop()

	mu.Lock()
	if processedCount != 5 {
		t.Errorf("Expected 5 processed events, got %d", processedCount)
	}
	mu.Unlock()
}

func TestConsumer_Stats(t *testing.T) {
	repo := newMockOutboxRepo()

	payload, _ := json.Marshal(map[string]string{"key": "value"})
	repo.addEvent(&domain.OutboxEvent{
		ID:            "event-1",
		AggregateType: domain.AggregateTypePaymentIntent,
		AggregateID:   "pi_123",
		EventType:     domain.OutboxEventPaymentAuthorized,
		Payload:       payload,
		CreatedAt:     time.Now(),
	})

	handler := func(ctx context.Context, event *domain.OutboxEvent) error {
		return nil
	}

	config := DefaultConsumerConfig()
	config.PollInterval = 10 * time.Millisecond
	consumer := NewConsumer(repo, handler, config)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	consumer.Start(ctx)
	time.Sleep(50 * time.Millisecond)
	consumer.Stop()

	processed, failed, lastPoll := consumer.Stats()
	if processed != 1 {
		t.Errorf("Expected 1 processed, got %d", processed)
	}
	if failed != 0 {
		t.Errorf("Expected 0 failed, got %d", failed)
	}
	if lastPoll.IsZero() {
		t.Error("Expected lastPoll to be set")
	}
}

func TestIdempotentHandler_DeduplicatesEvents(t *testing.T) {
	var processedCount int
	inner := func(ctx context.Context, event *domain.OutboxEvent) error {
		processedCount++
		return nil
	}

	handler := NewIdempotentHandler(inner)

	event := &domain.OutboxEvent{
		ID:            "event-1",
		AggregateType: domain.AggregateTypePaymentIntent,
		AggregateID:   "pi_123",
		EventType:     domain.OutboxEventPaymentAuthorized,
	}

	ctx := context.Background()

	// Process the same event twice
	if err := handler.Handle(ctx, event); err != nil {
		t.Fatalf("First handle failed: %v", err)
	}
	if err := handler.Handle(ctx, event); err != nil {
		t.Fatalf("Second handle failed: %v", err)
	}

	if processedCount != 1 {
		t.Errorf("Expected 1 processing, got %d", processedCount)
	}
	if handler.ProcessedCount() != 1 {
		t.Errorf("Expected ProcessedCount=1, got %d", handler.ProcessedCount())
	}
}

func TestPaymentEventHandler_RoutesEvents(t *testing.T) {
	var authorizedCalled, capturedCalled bool

	handler := &PaymentEventHandler{
		OnPaymentAuthorized: func(ctx context.Context, payload *domain.PaymentEventPayload) error {
			authorizedCalled = true
			return nil
		},
		OnPaymentCaptured: func(ctx context.Context, payload *domain.PaymentEventPayload) error {
			capturedCalled = true
			return nil
		},
	}

	ctx := context.Background()
	payload, _ := json.Marshal(domain.PaymentEventPayload{
		EventID:   "evt_123",
		EventType: "payment.authorized",
		PaymentID: "pi_123",
	})

	// Test authorized event
	authEvent := &domain.OutboxEvent{
		ID:            "event-1",
		AggregateType: domain.AggregateTypePaymentIntent,
		AggregateID:   "pi_123",
		EventType:     domain.OutboxEventPaymentAuthorized,
		Payload:       payload,
	}
	if err := handler.Handle(ctx, authEvent); err != nil {
		t.Fatalf("Handle authorized failed: %v", err)
	}

	// Test captured event
	captureEvent := &domain.OutboxEvent{
		ID:            "event-2",
		AggregateType: domain.AggregateTypePaymentIntent,
		AggregateID:   "pi_123",
		EventType:     domain.OutboxEventPaymentCaptured,
		Payload:       payload,
	}
	if err := handler.Handle(ctx, captureEvent); err != nil {
		t.Fatalf("Handle captured failed: %v", err)
	}

	if !authorizedCalled {
		t.Error("Expected OnPaymentAuthorized to be called")
	}
	if !capturedCalled {
		t.Error("Expected OnPaymentCaptured to be called")
	}
}

func TestChainHandlers(t *testing.T) {
	var order []int

	h1 := func(ctx context.Context, event *domain.OutboxEvent) error {
		order = append(order, 1)
		return nil
	}
	h2 := func(ctx context.Context, event *domain.OutboxEvent) error {
		order = append(order, 2)
		return nil
	}
	h3 := func(ctx context.Context, event *domain.OutboxEvent) error {
		order = append(order, 3)
		return nil
	}

	chain := ChainHandlers(h1, h2, h3)

	event := &domain.OutboxEvent{ID: "test"}
	if err := chain(context.Background(), event); err != nil {
		t.Fatalf("Chain failed: %v", err)
	}

	if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 3 {
		t.Errorf("Expected [1,2,3], got %v", order)
	}
}
