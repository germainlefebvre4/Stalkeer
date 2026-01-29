package errors

import (
	"errors"
	"testing"
)

func TestNew(t *testing.T) {
	err := New(CodeValidation, "test error")
	if err.Code != CodeValidation {
		t.Errorf("expected code %s, got %s", CodeValidation, err.Code)
	}
	if err.Message != "test error" {
		t.Errorf("expected message 'test error', got %s", err.Message)
	}
	if err.Err != nil {
		t.Errorf("expected nil wrapped error, got %v", err.Err)
	}
}

func TestWrap(t *testing.T) {
	originalErr := errors.New("original error")
	err := Wrap(originalErr, CodeDatabase, "database operation failed")

	if err.Code != CodeDatabase {
		t.Errorf("expected code %s, got %s", CodeDatabase, err.Code)
	}
	if err.Message != "database operation failed" {
		t.Errorf("expected message 'database operation failed', got %s", err.Message)
	}
	if err.Err != originalErr {
		t.Errorf("expected wrapped error to be original error")
	}
}

func TestAppErrorError(t *testing.T) {
	tests := []struct {
		name     string
		err      *AppError
		expected string
	}{
		{
			name:     "error without wrapped error",
			err:      New(CodeValidation, "validation failed"),
			expected: "[VALIDATION_ERROR] validation failed",
		},
		{
			name:     "error with wrapped error",
			err:      Wrap(errors.New("inner"), CodeDatabase, "db error"),
			expected: "[DATABASE_ERROR] db error: inner",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("Error() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAppErrorUnwrap(t *testing.T) {
	originalErr := errors.New("original")
	err := Wrap(originalErr, CodeDatabase, "wrapped")

	if unwrapped := err.Unwrap(); unwrapped != originalErr {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, originalErr)
	}
}

func TestAppErrorWithContext(t *testing.T) {
	err := New(CodeValidation, "test").
		WithContext("field", "email").
		WithContext("value", "invalid")

	if len(err.Context) != 2 {
		t.Errorf("expected 2 context items, got %d", len(err.Context))
	}
	if err.Context["field"] != "email" {
		t.Errorf("expected field context 'email', got %v", err.Context["field"])
	}
	if err.Context["value"] != "invalid" {
		t.Errorf("expected value context 'invalid', got %v", err.Context["value"])
	}
}

func TestValidationError(t *testing.T) {
	err := ValidationError("invalid input")
	if err.Code != CodeValidation {
		t.Errorf("expected code %s, got %s", CodeValidation, err.Code)
	}
}

func TestDatabaseError(t *testing.T) {
	originalErr := errors.New("connection failed")
	err := DatabaseError("failed to connect", originalErr)
	if err.Code != CodeDatabase {
		t.Errorf("expected code %s, got %s", CodeDatabase, err.Code)
	}
	if err.Err != originalErr {
		t.Errorf("expected wrapped error to be original error")
	}
}

func TestParseError(t *testing.T) {
	originalErr := errors.New("invalid syntax")
	err := ParseError("failed to parse", originalErr)
	if err.Code != CodeParse {
		t.Errorf("expected code %s, got %s", CodeParse, err.Code)
	}
}

func TestClassificationError(t *testing.T) {
	err := ClassificationError("unknown type")
	if err.Code != CodeClassification {
		t.Errorf("expected code %s, got %s", CodeClassification, err.Code)
	}
}

func TestExternalServiceError(t *testing.T) {
	originalErr := errors.New("timeout")
	err := ExternalServiceError("radarr", "service timeout", originalErr)
	if err.Code != CodeExternalService {
		t.Errorf("expected code %s, got %s", CodeExternalService, err.Code)
	}
	if err.Context["service"] != "radarr" {
		t.Errorf("expected service context 'radarr', got %v", err.Context["service"])
	}
}

func TestConfigError(t *testing.T) {
	t.Run("with wrapped error", func(t *testing.T) {
		originalErr := errors.New("file not found")
		err := ConfigError("config load failed", originalErr)
		if err.Code != CodeConfig {
			t.Errorf("expected code %s, got %s", CodeConfig, err.Code)
		}
		if err.Err != originalErr {
			t.Errorf("expected wrapped error to be original error")
		}
	})

	t.Run("without wrapped error", func(t *testing.T) {
		err := ConfigError("missing required field", nil)
		if err.Code != CodeConfig {
			t.Errorf("expected code %s, got %s", CodeConfig, err.Code)
		}
		if err.Err != nil {
			t.Errorf("expected nil wrapped error, got %v", err.Err)
		}
	})
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "retryable service timeout",
			err:      Wrap(errors.New("timeout"), CodeServiceTimeout, "timeout"),
			expected: true,
		},
		{
			name:     "retryable service unavailable",
			err:      Wrap(errors.New("unavailable"), CodeServiceUnavailable, "unavailable"),
			expected: true,
		},
		{
			name:     "retryable rate limited",
			err:      Wrap(errors.New("rate limit"), CodeRateLimited, "rate limited"),
			expected: true,
		},
		{
			name:     "retryable database connection",
			err:      Wrap(errors.New("connection"), CodeDatabaseConnection, "connection"),
			expected: true,
		},
		{
			name:     "non-retryable validation error",
			err:      ValidationError("invalid"),
			expected: false,
		},
		{
			name:     "non-retryable parse error",
			err:      ParseError("parse failed", errors.New("syntax")),
			expected: false,
		},
		{
			name:     "non-app error",
			err:      errors.New("standard error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryable(tt.err); got != tt.expected {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetErrorCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected ErrorCode
	}{
		{
			name:     "app error",
			err:      ValidationError("test"),
			expected: CodeValidation,
		},
		{
			name:     "wrapped app error",
			err:      DatabaseError("test", errors.New("inner")),
			expected: CodeDatabase,
		},
		{
			name:     "standard error",
			err:      errors.New("standard"),
			expected: CodeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetErrorCode(tt.err); got != tt.expected {
				t.Errorf("GetErrorCode() = %v, want %v", got, tt.expected)
			}
		})
	}
}
