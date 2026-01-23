package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestLogger_Info(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithOutput("test", &buf)

	ctx := context.Background()
	logger.Info(ctx, "test message", "key", "value")

	var entry LogEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to unmarshal log entry: %v", err)
	}

	if entry.Level != "info" {
		t.Errorf("Expected level 'info', got '%s'", entry.Level)
	}
	if entry.Message != "test message" {
		t.Errorf("Expected message 'test message', got '%s'", entry.Message)
	}
	if entry.Component != "test" {
		t.Errorf("Expected component 'test', got '%s'", entry.Component)
	}
	if entry.Fields["key"] != "value" {
		t.Errorf("Expected field key=value, got %v", entry.Fields)
	}
}

func TestLogger_WithCorrelationID(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithOutput("test", &buf)

	ctx := WithCorrelationID(context.Background(), "corr-123")
	logger.Info(ctx, "test message")

	var entry LogEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to unmarshal log entry: %v", err)
	}

	if entry.CorrelationID != "corr-123" {
		t.Errorf("Expected correlation_id 'corr-123', got '%s'", entry.CorrelationID)
	}
}

func TestLogger_WithRequestID(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithOutput("test", &buf)

	ctx := WithRequestID(context.Background(), "req-456")
	logger.Info(ctx, "test message")

	var entry LogEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to unmarshal log entry: %v", err)
	}

	if entry.RequestID != "req-456" {
		t.Errorf("Expected request_id 'req-456', got '%s'", entry.RequestID)
	}
}

func TestLogger_Error(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithOutput("test", &buf)

	ctx := context.Background()
	logger.Error(ctx, "error occurred", errors.New("test error"), "key", "value")

	var entry LogEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to unmarshal log entry: %v", err)
	}

	if entry.Level != "error" {
		t.Errorf("Expected level 'error', got '%s'", entry.Level)
	}
	if entry.Error != "test error" {
		t.Errorf("Expected error 'test error', got '%s'", entry.Error)
	}
}

func TestLogger_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithOutput("test", &buf)
	logger.SetLevel(LevelWarn)

	ctx := context.Background()
	logger.Debug(ctx, "debug message")
	logger.Info(ctx, "info message")

	if buf.Len() > 0 {
		t.Error("Expected no output for debug/info when level is warn")
	}

	logger.Warn(ctx, "warn message")
	if buf.Len() == 0 {
		t.Error("Expected output for warn when level is warn")
	}
}

func TestLogger_WithFields(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithOutput("test", &buf)

	fl := logger.WithFields(map[string]any{
		"service": "payment",
		"version": "1.0",
	})

	ctx := context.Background()
	fl.Info(ctx, "test message", "extra", "field")

	var entry LogEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to unmarshal log entry: %v", err)
	}

	if entry.Fields["service"] != "payment" {
		t.Errorf("Expected field service=payment, got %v", entry.Fields["service"])
	}
	if entry.Fields["version"] != "1.0" {
		t.Errorf("Expected field version=1.0, got %v", entry.Fields["version"])
	}
	if entry.Fields["extra"] != "field" {
		t.Errorf("Expected field extra=field, got %v", entry.Fields["extra"])
	}
}

func TestLogger_AllContextValues(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithOutput("test", &buf)

	ctx := context.Background()
	ctx = WithCorrelationID(ctx, "corr-123")
	ctx = WithRequestID(ctx, "req-456")
	ctx = WithWorkflowID(ctx, "wf-789")
	ctx = WithProvider(ctx, "stripe")

	logger.Info(ctx, "test message")

	var entry LogEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to unmarshal log entry: %v", err)
	}

	if entry.CorrelationID != "corr-123" {
		t.Errorf("Expected correlation_id 'corr-123', got '%s'", entry.CorrelationID)
	}
	if entry.RequestID != "req-456" {
		t.Errorf("Expected request_id 'req-456', got '%s'", entry.RequestID)
	}
	if entry.WorkflowID != "wf-789" {
		t.Errorf("Expected workflow_id 'wf-789', got '%s'", entry.WorkflowID)
	}
	if entry.Provider != "stripe" {
		t.Errorf("Expected provider 'stripe', got '%s'", entry.Provider)
	}
}

func TestGetContextValues(t *testing.T) {
	ctx := context.Background()

	// Test empty context
	if GetCorrelationID(ctx) != "" {
		t.Error("Expected empty correlation ID for empty context")
	}
	if GetRequestID(ctx) != "" {
		t.Error("Expected empty request ID for empty context")
	}

	// Test with values
	ctx = WithCorrelationID(ctx, "corr-123")
	ctx = WithRequestID(ctx, "req-456")

	if GetCorrelationID(ctx) != "corr-123" {
		t.Errorf("Expected correlation ID 'corr-123', got '%s'", GetCorrelationID(ctx))
	}
	if GetRequestID(ctx) != "req-456" {
		t.Errorf("Expected request ID 'req-456', got '%s'", GetRequestID(ctx))
	}
}
