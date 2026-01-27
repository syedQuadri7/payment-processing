package outbox

import (
	"context"
	"log"
	"sync"
	"time"

	"payment-processing/shared/domain"
)

// EventHandler is called for each event polled from the outbox
type EventHandler func(ctx context.Context, event *domain.OutboxEvent) error

// ConsumerConfig holds configuration for the outbox consumer
type ConsumerConfig struct {
	// PollInterval is how often to poll for new events
	PollInterval time.Duration

	// BatchSize is the maximum number of events to fetch per poll
	BatchSize int

	// RetentionPeriod is how long processed events remain in the table
	// before cleanup (used when CDC is the primary consumer)
	RetentionPeriod time.Duration

	// CleanupInterval is how often to run cleanup of old events
	CleanupInterval time.Duration
}

// DefaultConsumerConfig returns sensible defaults for the consumer
func DefaultConsumerConfig() ConsumerConfig {
	return ConsumerConfig{
		PollInterval:    1 * time.Second,
		BatchSize:       100,
		RetentionPeriod: 24 * time.Hour,
		CleanupInterval: 1 * time.Hour,
	}
}

// OutboxRepository defines the operations needed by the consumer
type OutboxRepository interface {
	GetUnpublished(ctx context.Context, limit int) ([]*domain.OutboxEvent, error)
	MarkPublished(ctx context.Context, ids []string) error
	DeleteOlderThan(ctx context.Context, before time.Time) (int64, error)
}

// Consumer polls the outbox table and processes events
type Consumer struct {
	repo    OutboxRepository
	config  ConsumerConfig
	handler EventHandler

	mu      sync.Mutex
	running bool
	stopCh  chan struct{}
	doneCh  chan struct{}

	// Metrics
	eventsProcessed int64
	eventsFailed    int64
	lastPollTime    time.Time
}

// NewConsumer creates a new outbox consumer
func NewConsumer(repo OutboxRepository, handler EventHandler, config ConsumerConfig) *Consumer {
	return &Consumer{
		repo:    repo,
		config:  config,
		handler: handler,
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
	}
}

// Start begins polling the outbox table
func (c *Consumer) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return nil
	}
	c.running = true
	c.mu.Unlock()

	log.Printf("Outbox consumer starting (poll_interval=%v, batch_size=%d)",
		c.config.PollInterval, c.config.BatchSize)

	go c.pollLoop(ctx)
	go c.cleanupLoop(ctx)

	return nil
}

// Stop stops the consumer gracefully
func (c *Consumer) Stop() {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return
	}
	c.running = false
	c.mu.Unlock()

	close(c.stopCh)
	<-c.doneCh

	log.Printf("Outbox consumer stopped (processed=%d, failed=%d)",
		c.eventsProcessed, c.eventsFailed)
}

// Stats returns current consumer statistics
func (c *Consumer) Stats() (processed, failed int64, lastPoll time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.eventsProcessed, c.eventsFailed, c.lastPollTime
}

// pollLoop continuously polls for new events
func (c *Consumer) pollLoop(ctx context.Context) {
	defer close(c.doneCh)

	ticker := time.NewTicker(c.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.poll(ctx)
		}
	}
}

// poll fetches and processes a batch of events
func (c *Consumer) poll(ctx context.Context) {
	c.mu.Lock()
	c.lastPollTime = time.Now()
	c.mu.Unlock()

	events, err := c.repo.GetUnpublished(ctx, c.config.BatchSize)
	if err != nil {
		log.Printf("Outbox poll error: %v", err)
		return
	}

	if len(events) == 0 {
		return
	}

	log.Printf("Outbox poll: processing %d events", len(events))

	var successIDs []string
	for _, event := range events {
		if err := c.processEvent(ctx, event); err != nil {
			log.Printf("Outbox event %s failed: %v", event.ID, err)
			c.mu.Lock()
			c.eventsFailed++
			c.mu.Unlock()
			// Continue processing other events
			continue
		}
		successIDs = append(successIDs, event.ID)
		c.mu.Lock()
		c.eventsProcessed++
		c.mu.Unlock()
	}

	// Mark successful events as published (delete them)
	if len(successIDs) > 0 {
		if err := c.repo.MarkPublished(ctx, successIDs); err != nil {
			log.Printf("Outbox mark published error: %v", err)
		}
	}
}

// processEvent handles a single event
func (c *Consumer) processEvent(ctx context.Context, event *domain.OutboxEvent) error {
	if c.handler == nil {
		// No handler configured - just log the event
		log.Printf("Outbox event: type=%s aggregate=%s/%s",
			event.EventType, event.AggregateType, event.AggregateID)
		return nil
	}

	return c.handler(ctx, event)
}

// cleanupLoop periodically removes old events
func (c *Consumer) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(c.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.cleanup(ctx)
		}
	}
}

// cleanup removes events older than the retention period
func (c *Consumer) cleanup(ctx context.Context) {
	cutoff := time.Now().Add(-c.config.RetentionPeriod)
	deleted, err := c.repo.DeleteOlderThan(ctx, cutoff)
	if err != nil {
		log.Printf("Outbox cleanup error: %v", err)
		return
	}
	if deleted > 0 {
		log.Printf("Outbox cleanup: deleted %d old events", deleted)
	}
}
