package errors

import (
	"errors"
	"fmt"
)

// ErrorCode represents a categorized error code
type ErrorCode string

const (
	// Validation errors
	CodeValidation   ErrorCode = "VALIDATION_ERROR"
	CodeInvalidInput ErrorCode = "INVALID_INPUT"

	// Database errors
	CodeDatabase            ErrorCode = "DATABASE_ERROR"
	CodeDatabaseConnection  ErrorCode = "DATABASE_CONNECTION_ERROR"
	CodeDatabaseQuery       ErrorCode = "DATABASE_QUERY_ERROR"
	CodeDatabaseTransaction ErrorCode = "DATABASE_TRANSACTION_ERROR"
	CodeNotFound            ErrorCode = "NOT_FOUND"

	// Parse errors
	CodeParse         ErrorCode = "PARSE_ERROR"
	CodeInvalidFormat ErrorCode = "INVALID_FORMAT"
	CodeMalformedData ErrorCode = "MALFORMED_DATA"

	// Classification errors
	CodeClassification     ErrorCode = "CLASSIFICATION_ERROR"
	CodeUnknownContentType ErrorCode = "UNKNOWN_CONTENT_TYPE"

	// External service errors
	CodeExternalService    ErrorCode = "EXTERNAL_SERVICE_ERROR"
	CodeServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
	CodeServiceTimeout     ErrorCode = "SERVICE_TIMEOUT"
	CodeUnauthorized       ErrorCode = "UNAUTHORIZED"
	CodeRateLimited        ErrorCode = "RATE_LIMITED"

	// Config errors
	CodeConfig        ErrorCode = "CONFIG_ERROR"
	CodeMissingConfig ErrorCode = "MISSING_CONFIG"
	CodeInvalidConfig ErrorCode = "INVALID_CONFIG"

	// Internal errors
	CodeInternal ErrorCode = "INTERNAL_ERROR"
	CodeUnknown  ErrorCode = "UNKNOWN_ERROR"
)

// AppError represents a structured application error
type AppError struct {
	Code    ErrorCode
	Message string
	Err     error
	Context map[string]interface{}
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap implements the errors.Unwrap interface
func (e *AppError) Unwrap() error {
	return e.Err
}

// WithContext adds context to the error
func (e *AppError) WithContext(key string, value interface{}) *AppError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// New creates a new AppError
func New(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

// Wrap wraps an existing error with additional context
func Wrap(err error, code ErrorCode, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// ValidationError creates a validation error
func ValidationError(message string) *AppError {
	return New(CodeValidation, message)
}

// DatabaseError creates a database error
func DatabaseError(message string, err error) *AppError {
	return Wrap(err, CodeDatabase, message)
}

// ParseError creates a parse error
func ParseError(message string, err error) *AppError {
	return Wrap(err, CodeParse, message)
}

// ClassificationError creates a classification error
func ClassificationError(message string) *AppError {
	return New(CodeClassification, message)
}

// ExternalServiceError creates an external service error
func ExternalServiceError(service, message string, err error) *AppError {
	return Wrap(err, CodeExternalService, message).
		WithContext("service", service)
}

// ConfigError creates a configuration error
func ConfigError(message string, err error) *AppError {
	if err != nil {
		return Wrap(err, CodeConfig, message)
	}
	return New(CodeConfig, message)
}

// IsRetryable determines if an error is retryable
func IsRetryable(err error) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		switch appErr.Code {
		case CodeServiceTimeout, CodeServiceUnavailable, CodeRateLimited,
			CodeDatabaseConnection:
			return true
		}
	}
	return false
}

// GetErrorCode extracts the error code from an error
func GetErrorCode(err error) ErrorCode {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return CodeUnknown
}

// IsValidationError checks if an error is a validation error
func IsValidationError(err error) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code == CodeValidation || appErr.Code == CodeInvalidInput
	}
	return false
}

// NotFoundError creates a not found error
func NotFoundError(resource, identifier string) *AppError {
	return New(CodeNotFound, fmt.Sprintf("%s not found: %s", resource, identifier))
}
