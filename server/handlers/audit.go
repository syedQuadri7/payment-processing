package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"payment-processing/pkg/domain"
	"payment-processing/server"
	"payment-processing/server/middleware"
)

// AuditLogRepository defines the interface for audit log queries
type AuditLogRepository interface {
	GetByEntity(ctx context.Context, entityType domain.AuditEntityType, entityID string) ([]*domain.AuditLogEntry, error)
	GetByActor(ctx context.Context, actorType domain.AuditActorType, actorID string, limit int) ([]*domain.AuditLogEntry, error)
	GetByTimeRange(ctx context.Context, start, end time.Time, limit int) ([]*domain.AuditLogEntry, error)
	GetByAction(ctx context.Context, action domain.AuditAction, limit int) ([]*domain.AuditLogEntry, error)
}

// AuditHandler handles audit log endpoints
type AuditHandler struct {
	repo AuditLogRepository
}

// NewAuditHandler creates a new audit handler
func NewAuditHandler(repo AuditLogRepository) *AuditHandler {
	return &AuditHandler{repo: repo}
}

// AuditEntryResponse is the response for a single audit entry
type AuditEntryResponse struct {
	ID            string         `json:"id"`
	EntityType    string         `json:"entity_type"`
	EntityID      string         `json:"entity_id"`
	Action        string         `json:"action"`
	ActorType     string         `json:"actor_type"`
	ActorID       *string        `json:"actor_id,omitempty"`
	OldValues     map[string]any `json:"old_values,omitempty"`
	NewValues     map[string]any `json:"new_values,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	IPAddress     *string        `json:"ip_address,omitempty"`
	UserAgent     *string        `json:"user_agent,omitempty"`
	CreatedAt     string         `json:"created_at"`
}

// AuditListResponse is the response for listing audit entries
type AuditListResponse struct {
	Entries []*AuditEntryResponse `json:"entries"`
	Count   int                   `json:"count"`
}

// GetByEntity handles GET /audit/entity/{type}/{id}
func (h *AuditHandler) GetByEntity(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())

	entityType := chi.URLParam(r, "type")
	entityID := chi.URLParam(r, "id")

	if entityType == "" || entityID == "" {
		server.WriteError(w, server.NewValidationError("path", "entity type and ID are required"), requestID)
		return
	}

	entries, err := h.repo.GetByEntity(r.Context(), domain.AuditEntityType(entityType), entityID)
	if err != nil {
		server.WriteError(w, server.NewInternalError("Error querying audit log"), requestID)
		return
	}

	response := h.toListResponse(entries)
	server.WriteJSON(w, http.StatusOK, response, requestID)
}

// GetByActor handles GET /audit/actor/{type}/{id}
func (h *AuditHandler) GetByActor(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())

	actorType := chi.URLParam(r, "type")
	actorID := chi.URLParam(r, "id")

	if actorType == "" || actorID == "" {
		server.WriteError(w, server.NewValidationError("path", "actor type and ID are required"), requestID)
		return
	}

	limit := parseLimit(r.URL.Query().Get("limit"), 100)

	entries, err := h.repo.GetByActor(r.Context(), domain.AuditActorType(actorType), actorID, limit)
	if err != nil {
		server.WriteError(w, server.NewInternalError("Error querying audit log"), requestID)
		return
	}

	response := h.toListResponse(entries)
	server.WriteJSON(w, http.StatusOK, response, requestID)
}

// GetByTimeRange handles GET /audit/range
func (h *AuditHandler) GetByTimeRange(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())

	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	if startStr == "" || endStr == "" {
		server.WriteError(w, server.NewValidationError("query", "start and end parameters are required"), requestID)
		return
	}

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		server.WriteError(w, server.NewValidationError("start", "invalid RFC3339 timestamp"), requestID)
		return
	}

	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		server.WriteError(w, server.NewValidationError("end", "invalid RFC3339 timestamp"), requestID)
		return
	}

	limit := parseLimit(r.URL.Query().Get("limit"), 100)

	entries, err := h.repo.GetByTimeRange(r.Context(), start, end, limit)
	if err != nil {
		server.WriteError(w, server.NewInternalError("Error querying audit log"), requestID)
		return
	}

	response := h.toListResponse(entries)
	server.WriteJSON(w, http.StatusOK, response, requestID)
}

// GetByAction handles GET /audit/action/{action}
func (h *AuditHandler) GetByAction(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())

	action := chi.URLParam(r, "action")
	if action == "" {
		server.WriteError(w, server.NewValidationError("path", "action is required"), requestID)
		return
	}

	limit := parseLimit(r.URL.Query().Get("limit"), 100)

	entries, err := h.repo.GetByAction(r.Context(), domain.AuditAction(action), limit)
	if err != nil {
		server.WriteError(w, server.NewInternalError("Error querying audit log"), requestID)
		return
	}

	response := h.toListResponse(entries)
	server.WriteJSON(w, http.StatusOK, response, requestID)
}

// toListResponse converts domain entries to response format
func (h *AuditHandler) toListResponse(entries []*domain.AuditLogEntry) *AuditListResponse {
	responses := make([]*AuditEntryResponse, len(entries))
	for i, entry := range entries {
		responses[i] = h.toEntryResponse(entry)
	}
	return &AuditListResponse{
		Entries: responses,
		Count:   len(responses),
	}
}

// toEntryResponse converts a domain entry to response format
func (h *AuditHandler) toEntryResponse(entry *domain.AuditLogEntry) *AuditEntryResponse {
	resp := &AuditEntryResponse{
		ID:         entry.ID,
		EntityType: string(entry.EntityType),
		EntityID:   entry.EntityID,
		Action:     string(entry.Action),
		ActorType:  string(entry.ActorType),
		ActorID:    entry.ActorID,
		UserAgent:  entry.UserAgent,
		CreatedAt:  entry.CreatedAt.Format(time.RFC3339Nano),
	}

	if entry.IPAddress != nil {
		ip := entry.IPAddress.String()
		resp.IPAddress = &ip
	}

	// Parse JSON fields if present
	if len(entry.OldValues) > 0 {
		var oldValues map[string]any
		if err := entry.OldValues.UnmarshalJSON(entry.OldValues); err == nil {
			resp.OldValues = oldValues
		}
	}
	if len(entry.NewValues) > 0 {
		var newValues map[string]any
		if err := entry.NewValues.UnmarshalJSON(entry.NewValues); err == nil {
			resp.NewValues = newValues
		}
	}
	if len(entry.Metadata) > 0 {
		var metadata map[string]any
		if err := entry.Metadata.UnmarshalJSON(entry.Metadata); err == nil {
			resp.Metadata = metadata
		}
	}

	return resp
}

// parseLimit parses a limit parameter with a default value
func parseLimit(s string, defaultLimit int) int {
	if s == "" {
		return defaultLimit
	}
	limit, err := strconv.Atoi(s)
	if err != nil || limit <= 0 {
		return defaultLimit
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}
