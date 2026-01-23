package handlers

import (
	"context"
	"net/http"
	"sync"
	"time"

	"go.temporal.io/sdk/client"

	"payment-processing/server"
	"payment-processing/server/middleware"
)

// HealthChecker defines the interface for health checking a component
type HealthChecker interface {
	Check(ctx context.Context) error
}

// DatabaseHealthChecker checks database connectivity
type DatabaseHealthChecker interface {
	Ping(ctx context.Context) error
}

// HealthHandler handles health check endpoints
type HealthHandler struct {
	version    string
	dbChecker  DatabaseHealthChecker
	temporal   client.Client
	mu         sync.RWMutex
	ready      bool
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(version string, dbChecker DatabaseHealthChecker, temporal client.Client) *HealthHandler {
	return &HealthHandler{
		version:   version,
		dbChecker: dbChecker,
		temporal:  temporal,
		ready:     true, // Assume ready initially
	}
}

// SetReady sets the readiness state
func (h *HealthHandler) SetReady(ready bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ready = ready
}

// Health handles GET /health - overall service health
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())

	if r.Method != http.MethodGet {
		server.WriteError(w, server.NewMethodNotAllowedError(r.Method), requestID)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	checks := make(map[string]server.CheckResult)
	overallStatus := "healthy"

	// Check database
	dbStatus := h.checkDatabase(ctx)
	checks["database"] = dbStatus
	if dbStatus.Status != "healthy" {
		overallStatus = "degraded"
	}

	// Check Temporal
	temporalStatus := h.checkTemporal(ctx)
	checks["temporal"] = temporalStatus
	if temporalStatus.Status != "healthy" {
		overallStatus = "degraded"
	}

	// If both are unhealthy, mark as unhealthy
	if dbStatus.Status == "unhealthy" && temporalStatus.Status == "unhealthy" {
		overallStatus = "unhealthy"
	}

	resp := server.HealthResponse{
		Status:    overallStatus,
		Version:   h.version,
		Checks:    checks,
		Timestamp: time.Now(),
	}

	status := http.StatusOK
	if overallStatus == "unhealthy" {
		status = http.StatusServiceUnavailable
	}

	server.WriteJSON(w, status, resp, requestID)
}

// Live handles GET /health/live - liveness probe
func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())

	if r.Method != http.MethodGet {
		server.WriteError(w, server.NewMethodNotAllowedError(r.Method), requestID)
		return
	}

	// Liveness probe should only fail if the process is truly stuck
	// It should always return 200 if the server can handle requests
	resp := server.HealthResponse{
		Status:    "alive",
		Timestamp: time.Now(),
	}

	server.WriteJSON(w, http.StatusOK, resp, requestID)
}

// Ready handles GET /health/ready - readiness probe
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())

	if r.Method != http.MethodGet {
		server.WriteError(w, server.NewMethodNotAllowedError(r.Method), requestID)
		return
	}

	h.mu.RLock()
	ready := h.ready
	h.mu.RUnlock()

	if !ready {
		resp := server.HealthResponse{
			Status:    "not_ready",
			Timestamp: time.Now(),
		}
		server.WriteJSON(w, http.StatusServiceUnavailable, resp, requestID)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	// Check critical dependencies for readiness
	checks := make(map[string]server.CheckResult)
	isReady := true

	// Database must be available for readiness
	dbStatus := h.checkDatabase(ctx)
	checks["database"] = dbStatus
	if dbStatus.Status != "healthy" {
		isReady = false
	}

	// Temporal must be available for readiness
	temporalStatus := h.checkTemporal(ctx)
	checks["temporal"] = temporalStatus
	if temporalStatus.Status != "healthy" {
		isReady = false
	}

	status := "ready"
	httpStatus := http.StatusOK
	if !isReady {
		status = "not_ready"
		httpStatus = http.StatusServiceUnavailable
	}

	resp := server.HealthResponse{
		Status:    status,
		Checks:    checks,
		Timestamp: time.Now(),
	}

	server.WriteJSON(w, httpStatus, resp, requestID)
}

// checkDatabase checks database health
func (h *HealthHandler) checkDatabase(ctx context.Context) server.CheckResult {
	if h.dbChecker == nil {
		return server.CheckResult{
			Status:  "unknown",
			Message: "database checker not configured",
		}
	}

	if err := h.dbChecker.Ping(ctx); err != nil {
		return server.CheckResult{
			Status:  "unhealthy",
			Message: err.Error(),
		}
	}

	return server.CheckResult{
		Status: "healthy",
	}
}

// checkTemporal checks Temporal connectivity
func (h *HealthHandler) checkTemporal(ctx context.Context) server.CheckResult {
	if h.temporal == nil {
		return server.CheckResult{
			Status:  "unknown",
			Message: "temporal client not configured",
		}
	}

	// Try to describe the task queue as a connectivity check
	// This is a lightweight operation that verifies connectivity
	_, err := h.temporal.DescribeTaskQueue(ctx, "payment-processing", 1) // workflow type
	if err != nil {
		return server.CheckResult{
			Status:  "unhealthy",
			Message: err.Error(),
		}
	}

	return server.CheckResult{
		Status: "healthy",
	}
}
