package domain

import "time"

// ProcessedEvent tracks processed webhook events for idempotency
type ProcessedEvent struct {
	ID          string
	Provider    Provider
	EventID     string // Provider's event ID
	EventType   string
	ProcessedAt time.Time
}
