package server

import (
	"encoding/json"
	"net/http"
)

// ErrorType represents the category of an API error
type ErrorType string

const (
	ErrorTypeValidation     ErrorType = "validation_error"
	ErrorTypeAuthentication ErrorType = "authentication_error"
	ErrorTypeNotFound       ErrorType = "not_found"
	ErrorTypeConflict       ErrorType = "conflict"
	ErrorTypeBusinessRule   ErrorType = "business_rule_violation"
	ErrorTypeInternal       ErrorType = "internal_error"
	ErrorTypeRateLimit      ErrorType = "rate_limit_exceeded"
)

// APIError represents a structured API error response
type APIError struct {
	Type      ErrorType `json:"type"`
	Code      string    `json:"code"`
	Message   string    `json:"message"`
	Param     string    `json:"param,omitempty"`
	RequestID string    `json:"request_id,omitempty"`
}

func (e *APIError) Error() string {
	return e.Message
}

// HTTPStatus returns the appropriate HTTP status code for this error type
func (e *APIError) HTTPStatus() int {
	switch e.Type {
	case ErrorTypeValidation:
		return http.StatusBadRequest
	case ErrorTypeAuthentication:
		return http.StatusUnauthorized
	case ErrorTypeNotFound:
		return http.StatusNotFound
	case ErrorTypeConflict:
		return http.StatusConflict
	case ErrorTypeBusinessRule:
		return http.StatusUnprocessableEntity
	case ErrorTypeRateLimit:
		return http.StatusTooManyRequests
	default:
		return http.StatusInternalServerError
	}
}

// NewValidationError creates a validation error
func NewValidationError(param, message string) *APIError {
	return &APIError{
		Type:    ErrorTypeValidation,
		Code:    "invalid_" + param,
		Message: message,
		Param:   param,
	}
}

// NewAuthenticationError creates an authentication error
func NewAuthenticationError(message string) *APIError {
	return &APIError{
		Type:    ErrorTypeAuthentication,
		Code:    "authentication_required",
		Message: message,
	}
}

// NewNotFoundError creates a not found error
func NewNotFoundError(resource, id string) *APIError {
	return &APIError{
		Type:    ErrorTypeNotFound,
		Code:    resource + "_not_found",
		Message: resource + " not found: " + id,
	}
}

// NewConflictError creates a conflict error (e.g., idempotency conflict)
func NewConflictError(code, message string) *APIError {
	return &APIError{
		Type:    ErrorTypeConflict,
		Code:    code,
		Message: message,
	}
}

// NewBusinessRuleError creates a business rule violation error
func NewBusinessRuleError(code, message string) *APIError {
	return &APIError{
		Type:    ErrorTypeBusinessRule,
		Code:    code,
		Message: message,
	}
}

// NewInternalError creates an internal server error
func NewInternalError(message string) *APIError {
	return &APIError{
		Type:    ErrorTypeInternal,
		Code:    "internal_error",
		Message: message,
	}
}

// NewRateLimitError creates a rate limit exceeded error
func NewRateLimitError() *APIError {
	return &APIError{
		Type:    ErrorTypeRateLimit,
		Code:    "rate_limit_exceeded",
		Message: "Too many requests. Please retry after the rate limit window resets.",
	}
}

// NewMethodNotAllowedError creates a method not allowed error
func NewMethodNotAllowedError(method string) *APIError {
	return &APIError{
		Type:    ErrorTypeValidation,
		Code:    "method_not_allowed",
		Message: "Method " + method + " not allowed",
	}
}

// NewInvalidJSONError creates an invalid JSON error
func NewInvalidJSONError() *APIError {
	return &APIError{
		Type:    ErrorTypeValidation,
		Code:    "invalid_json",
		Message: "Invalid JSON payload",
	}
}

// NewMissingHeaderError creates a missing header error
func NewMissingHeaderError(header string) *APIError {
	return &APIError{
		Type:    ErrorTypeValidation,
		Code:    "missing_header",
		Message: "Missing required header: " + header,
		Param:   header,
	}
}

// NewInvalidStateTransitionError creates an invalid state transition error
// L2: Uses generic message to avoid leaking implementation details
func NewInvalidStateTransitionError(current, attempted string) *APIError {
	return &APIError{
		Type:    ErrorTypeBusinessRule,
		Code:    "invalid_state",
		Message: "This operation is not permitted in the current state",
	}
}

// APIErrorResponse wraps an APIError in a response envelope
type APIErrorResponse struct {
	Error *APIError `json:"error"`
}

// WriteError writes an API error as JSON response
func WriteError(w http.ResponseWriter, err *APIError, requestID string) {
	err.RequestID = requestID
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-Id", requestID)
	w.WriteHeader(err.HTTPStatus())
	json.NewEncoder(w).Encode(APIErrorResponse{Error: err})
}

// WriteJSON writes a successful JSON response
func WriteJSON(w http.ResponseWriter, status int, data interface{}, requestID string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-Id", requestID)
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// WriteJSONWithHeaders writes a successful JSON response with additional headers
func WriteJSONWithHeaders(w http.ResponseWriter, status int, data interface{}, headers map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	for k, v := range headers {
		w.Header().Set(k, v)
	}
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
