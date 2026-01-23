package domain

import (
	"encoding/json"
	"net"
	"time"
)

// AuditEntityType represents the type of entity in the audit log
type AuditEntityType string

const (
	AuditEntityPaymentIntent     AuditEntityType = "PAYMENT_INTENT"
	AuditEntityPaymentMethod     AuditEntityType = "PAYMENT_METHOD"
	AuditEntityAccount           AuditEntityType = "ACCOUNT"
	AuditEntityAuthorizationHold AuditEntityType = "AUTHORIZATION_HOLD"
)

// AuditAction represents the type of action in the audit log
type AuditAction string

const (
	AuditActionCreate       AuditAction = "CREATE"
	AuditActionUpdate       AuditAction = "UPDATE"
	AuditActionDelete       AuditAction = "DELETE"
	AuditActionStatusChange AuditAction = "STATUS_CHANGE"
)

// AuditActorType represents the type of actor in the audit log
type AuditActorType string

const (
	AuditActorSystem   AuditActorType = "SYSTEM"
	AuditActorUser     AuditActorType = "USER"
	AuditActorWebhook  AuditActorType = "WEBHOOK"
	AuditActorAPI      AuditActorType = "API"
	AuditActorWorkflow AuditActorType = "WORKFLOW"
)

// AuditLogEntry represents an entry in the audit log
type AuditLogEntry struct {
	ID         string
	EntityType AuditEntityType
	EntityID   string
	Action     AuditAction
	ActorType  AuditActorType
	ActorID    *string
	OldValues  json.RawMessage
	NewValues  json.RawMessage
	Metadata   json.RawMessage
	IPAddress  net.IP
	UserAgent  *string
	CreatedAt  time.Time
}

// AuditMetadata represents common metadata fields for audit entries
type AuditMetadata struct {
	CorrelationID string `json:"correlation_id,omitempty"`
	Provider      string `json:"provider,omitempty"`
	WorkflowID    string `json:"workflow_id,omitempty"`
	RequestID     string `json:"request_id,omitempty"`
}
