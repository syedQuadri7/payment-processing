package logging

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"
)

// Level represents log severity levels
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	default:
		return "unknown"
	}
}

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp     string         `json:"timestamp"`
	Level         string         `json:"level"`
	Message       string         `json:"message"`
	CorrelationID string         `json:"correlation_id,omitempty"`
	RequestID     string         `json:"request_id,omitempty"`
	WorkflowID    string         `json:"workflow_id,omitempty"`
	Provider      string         `json:"provider,omitempty"`
	Component     string         `json:"component,omitempty"`
	Fields        map[string]any `json:"fields,omitempty"`
	Error         string         `json:"error,omitempty"`
}

// Logger is a structured JSON logger
type Logger struct {
	mu        sync.Mutex
	output    io.Writer
	level     Level
	component string
}

// New creates a new Logger
func New(component string) *Logger {
	return &Logger{
		output:    os.Stdout,
		level:     LevelInfo,
		component: component,
	}
}

// NewWithOutput creates a new Logger with a custom output
func NewWithOutput(component string, output io.Writer) *Logger {
	return &Logger{
		output:    output,
		level:     LevelInfo,
		component: component,
	}
}

// SetLevel sets the minimum log level
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// Context keys for correlation
type contextKey string

const (
	correlationIDKey contextKey = "correlation_id"
	requestIDKey     contextKey = "request_id"
	workflowIDKey    contextKey = "workflow_id"
	providerKey      contextKey = "provider"
)

// WithCorrelationID adds a correlation ID to the context
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, correlationIDKey, correlationID)
}

// GetCorrelationID retrieves the correlation ID from the context
func GetCorrelationID(ctx context.Context) string {
	if v := ctx.Value(correlationIDKey); v != nil {
		return v.(string)
	}
	return ""
}

// WithRequestID adds a request ID to the context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// GetRequestID retrieves the request ID from the context
func GetRequestID(ctx context.Context) string {
	if v := ctx.Value(requestIDKey); v != nil {
		return v.(string)
	}
	return ""
}

// WithWorkflowID adds a workflow ID to the context
func WithWorkflowID(ctx context.Context, workflowID string) context.Context {
	return context.WithValue(ctx, workflowIDKey, workflowID)
}

// GetWorkflowID retrieves the workflow ID from the context
func GetWorkflowID(ctx context.Context) string {
	if v := ctx.Value(workflowIDKey); v != nil {
		return v.(string)
	}
	return ""
}

// WithProvider adds a provider to the context
func WithProvider(ctx context.Context, provider string) context.Context {
	return context.WithValue(ctx, providerKey, provider)
}

// GetProvider retrieves the provider from the context
func GetProvider(ctx context.Context) string {
	if v := ctx.Value(providerKey); v != nil {
		return v.(string)
	}
	return ""
}

// log writes a log entry
func (l *Logger) log(ctx context.Context, level Level, msg string, fields map[string]any, err error) {
	if level < l.level {
		return
	}

	entry := LogEntry{
		Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
		Level:         level.String(),
		Message:       msg,
		Component:     l.component,
		CorrelationID: GetCorrelationID(ctx),
		RequestID:     GetRequestID(ctx),
		WorkflowID:    GetWorkflowID(ctx),
		Provider:      GetProvider(ctx),
		Fields:        fields,
	}

	if err != nil {
		entry.Error = err.Error()
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	data, _ := json.Marshal(entry)
	l.output.Write(data)
	l.output.Write([]byte("\n"))
}

// Debug logs a debug message
func (l *Logger) Debug(ctx context.Context, msg string, fields ...any) {
	l.log(ctx, LevelDebug, msg, toMap(fields), nil)
}

// Info logs an info message
func (l *Logger) Info(ctx context.Context, msg string, fields ...any) {
	l.log(ctx, LevelInfo, msg, toMap(fields), nil)
}

// Warn logs a warning message
func (l *Logger) Warn(ctx context.Context, msg string, fields ...any) {
	l.log(ctx, LevelWarn, msg, toMap(fields), nil)
}

// Error logs an error message
func (l *Logger) Error(ctx context.Context, msg string, err error, fields ...any) {
	l.log(ctx, LevelError, msg, toMap(fields), err)
}

// WithFields returns a FieldLogger for chaining fields
func (l *Logger) WithFields(fields map[string]any) *FieldLogger {
	return &FieldLogger{
		logger: l,
		fields: fields,
	}
}

// FieldLogger is a logger with pre-set fields
type FieldLogger struct {
	logger *Logger
	fields map[string]any
}

// Debug logs a debug message with pre-set fields
func (fl *FieldLogger) Debug(ctx context.Context, msg string, fields ...any) {
	merged := mergeMaps(fl.fields, toMap(fields))
	fl.logger.log(ctx, LevelDebug, msg, merged, nil)
}

// Info logs an info message with pre-set fields
func (fl *FieldLogger) Info(ctx context.Context, msg string, fields ...any) {
	merged := mergeMaps(fl.fields, toMap(fields))
	fl.logger.log(ctx, LevelInfo, msg, merged, nil)
}

// Warn logs a warning message with pre-set fields
func (fl *FieldLogger) Warn(ctx context.Context, msg string, fields ...any) {
	merged := mergeMaps(fl.fields, toMap(fields))
	fl.logger.log(ctx, LevelWarn, msg, merged, nil)
}

// Error logs an error message with pre-set fields
func (fl *FieldLogger) Error(ctx context.Context, msg string, err error, fields ...any) {
	merged := mergeMaps(fl.fields, toMap(fields))
	fl.logger.log(ctx, LevelError, msg, merged, err)
}

// toMap converts key-value pairs to a map
func toMap(fields []any) map[string]any {
	if len(fields) == 0 {
		return nil
	}

	result := make(map[string]any)
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			result[key] = fields[i+1]
		}
	}
	return result
}

// mergeMaps merges two maps, with the second taking precedence
func mergeMaps(base, override map[string]any) map[string]any {
	if len(base) == 0 {
		return override
	}
	if len(override) == 0 {
		return base
	}

	result := make(map[string]any)
	for k, v := range base {
		result[k] = v
	}
	for k, v := range override {
		result[k] = v
	}
	return result
}
