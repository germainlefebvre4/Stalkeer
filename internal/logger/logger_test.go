package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Output:    &buf,
		MinLevel:  LevelDebug,
		WithStack: true,
	})

	if logger.output != &buf {
		t.Error("expected output to be set")
	}
	if logger.minLevel != LevelDebug {
		t.Errorf("expected minLevel DEBUG, got %s", logger.minLevel)
	}
	if !logger.withStack {
		t.Error("expected withStack to be true")
	}
}

func TestDefault(t *testing.T) {
	logger := Default()

	if logger.minLevel != LevelInfo {
		t.Errorf("expected minLevel INFO, got %s", logger.minLevel)
	}
	if logger.withStack {
		t.Error("expected withStack to be false")
	}
}

func TestDebug(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Output:   &buf,
		MinLevel: LevelDebug,
	})

	logger.Debug("debug message")

	var entry Entry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to unmarshal log entry: %v", err)
	}

	if entry.Level != LevelDebug {
		t.Errorf("expected level DEBUG, got %s", entry.Level)
	}
	if entry.Message != "debug message" {
		t.Errorf("expected message 'debug message', got %s", entry.Message)
	}
}

func TestInfo(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Output:   &buf,
		MinLevel: LevelInfo,
	})

	logger.Info("info message")

	var entry Entry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to unmarshal log entry: %v", err)
	}

	if entry.Level != LevelInfo {
		t.Errorf("expected level INFO, got %s", entry.Level)
	}
	if entry.Message != "info message" {
		t.Errorf("expected message 'info message', got %s", entry.Message)
	}
}

func TestWarn(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Output:   &buf,
		MinLevel: LevelWarn,
	})

	logger.Warn("warning message")

	var entry Entry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to unmarshal log entry: %v", err)
	}

	if entry.Level != LevelWarn {
		t.Errorf("expected level WARN, got %s", entry.Level)
	}
	if entry.Message != "warning message" {
		t.Errorf("expected message 'warning message', got %s", entry.Message)
	}
}

func TestError(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Output:   &buf,
		MinLevel: LevelError,
	})

	testErr := errors.New("test error")
	logger.Error("error message", testErr)

	var entry Entry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to unmarshal log entry: %v", err)
	}

	if entry.Level != LevelError {
		t.Errorf("expected level ERROR, got %s", entry.Level)
	}
	if entry.Message != "error message" {
		t.Errorf("expected message 'error message', got %s", entry.Message)
	}
	if entry.Error != "test error" {
		t.Errorf("expected error 'test error', got %s", entry.Error)
	}
}

func TestErrorWithStack(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Output:    &buf,
		MinLevel:  LevelError,
		WithStack: true,
	})

	testErr := errors.New("test error")
	logger.Error("error with stack", testErr)

	var entry Entry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to unmarshal log entry: %v", err)
	}

	if len(entry.Stack) == 0 {
		t.Error("expected stack trace to be present")
	}
}

func TestMinLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Output:   &buf,
		MinLevel: LevelWarn,
	})

	logger.Debug("debug message")
	logger.Info("info message")

	if buf.Len() > 0 {
		t.Error("expected no output for DEBUG and INFO when minLevel is WARN")
	}

	logger.Warn("warning message")

	if buf.Len() == 0 {
		t.Error("expected output for WARN when minLevel is WARN")
	}
}

func TestWithFields(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Output:   &buf,
		MinLevel: LevelInfo,
	})

	fieldLogger := logger.WithFields(map[string]interface{}{
		"user_id": "123",
		"action":  "login",
	})

	fieldLogger.Info("user logged in")

	var entry Entry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to unmarshal log entry: %v", err)
	}

	if entry.Context["user_id"] != "123" {
		t.Errorf("expected user_id '123', got %v", entry.Context["user_id"])
	}
	if entry.Context["action"] != "login" {
		t.Errorf("expected action 'login', got %v", entry.Context["action"])
	}
}

func TestContextWithRequestID(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Output:   &buf,
		MinLevel: LevelInfo,
	})

	ctx := ContextWithRequestID(context.Background(), "req-123")
	logger.InfoContext(ctx, "request received")

	var entry Entry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to unmarshal log entry: %v", err)
	}

	if entry.Context["request_id"] != "req-123" {
		t.Errorf("expected request_id 'req-123', got %v", entry.Context["request_id"])
	}
}

func TestContextWithUserID(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Output:   &buf,
		MinLevel: LevelInfo,
	})

	ctx := ContextWithUserID(context.Background(), "user-456")
	logger.InfoContext(ctx, "user action")

	var entry Entry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to unmarshal log entry: %v", err)
	}

	if entry.Context["user_id"] != "user-456" {
		t.Errorf("expected user_id 'user-456', got %v", entry.Context["user_id"])
	}
}

func TestFieldLoggerDebug(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Output:   &buf,
		MinLevel: LevelDebug,
	})

	fieldLogger := logger.WithFields(map[string]interface{}{
		"component": "parser",
	})

	fieldLogger.Debug("parsing started")

	var entry Entry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to unmarshal log entry: %v", err)
	}

	if entry.Level != LevelDebug {
		t.Errorf("expected level DEBUG, got %s", entry.Level)
	}
	if entry.Context["component"] != "parser" {
		t.Errorf("expected component 'parser', got %v", entry.Context["component"])
	}
}

func TestFieldLoggerError(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Output:   &buf,
		MinLevel: LevelError,
	})

	fieldLogger := logger.WithFields(map[string]interface{}{
		"component": "database",
	})

	testErr := errors.New("connection failed")
	fieldLogger.Error("database error", testErr)

	var entry Entry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to unmarshal log entry: %v", err)
	}

	if entry.Level != LevelError {
		t.Errorf("expected level ERROR, got %s", entry.Level)
	}
	if entry.Error != "connection failed" {
		t.Errorf("expected error 'connection failed', got %s", entry.Error)
	}
	if entry.Context["component"] != "database" {
		t.Errorf("expected component 'database', got %v", entry.Context["component"])
	}
}

func TestJSONFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Output:   &buf,
		MinLevel: LevelInfo,
	})

	logger.Info("test message")

	output := strings.TrimSpace(buf.String())

	if !json.Valid([]byte(output)) {
		t.Errorf("expected valid JSON, got: %s", output)
	}
}

func TestMultipleContextValues(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Output:   &buf,
		MinLevel: LevelInfo,
	})

	ctx := context.Background()
	ctx = ContextWithRequestID(ctx, "req-123")
	ctx = ContextWithUserID(ctx, "user-456")

	logger.InfoContext(ctx, "multiple context values")

	var entry Entry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to unmarshal log entry: %v", err)
	}

	if entry.Context["request_id"] != "req-123" {
		t.Errorf("expected request_id 'req-123', got %v", entry.Context["request_id"])
	}
	if entry.Context["user_id"] != "user-456" {
		t.Errorf("expected user_id 'user-456', got %v", entry.Context["user_id"])
	}
}

func TestNewWithLevel(t *testing.T) {
	tests := []struct {
		level         string
		expectedLevel Level
		expectStack   bool
	}{
		{"debug", LevelDebug, true},
		{"info", LevelInfo, false},
		{"warn", LevelWarn, false},
		{"error", LevelError, false},
		{"invalid", LevelInfo, false}, // defaults to info
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			logger := NewWithLevel(tt.level)
			if logger.minLevel != tt.expectedLevel {
				t.Errorf("expected level %s, got %s", tt.expectedLevel, logger.minLevel)
			}
			if logger.withStack != tt.expectStack {
				t.Errorf("expected withStack %v, got %v", tt.expectStack, logger.withStack)
			}
		})
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
	}{
		{"debug", LevelDebug},
		{"info", LevelInfo},
		{"warn", LevelWarn},
		{"error", LevelError},
		{"invalid", LevelInfo},
		{"", LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level := parseLevel(tt.input)
			if level != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, level)
			}
		})
	}
}

func TestInitializeLoggers(t *testing.T) {
	// Reset loggers
	mu.Lock()
	appLogger = nil
	databaseLogger = nil
	mu.Unlock()

	InitializeLoggers("debug", "warn")

	appLog := AppLogger()
	dbLog := DatabaseLogger()

	if appLog.minLevel != LevelDebug {
		t.Errorf("expected app logger level DEBUG, got %s", appLog.minLevel)
	}

	if dbLog.minLevel != LevelWarn {
		t.Errorf("expected database logger level WARN, got %s", dbLog.minLevel)
	}
}

func TestSetAppLogger(t *testing.T) {
	customLogger := NewWithLevel("error")
	SetAppLogger(customLogger)

	retrieved := AppLogger()
	if retrieved != customLogger {
		t.Error("expected custom logger to be set")
	}

	// Cleanup
	mu.Lock()
	appLogger = nil
	mu.Unlock()
}

func TestSetDatabaseLogger(t *testing.T) {
	customLogger := NewWithLevel("debug")
	SetDatabaseLogger(customLogger)

	retrieved := DatabaseLogger()
	if retrieved != customLogger {
		t.Error("expected custom database logger to be set")
	}

	// Cleanup
	mu.Lock()
	databaseLogger = nil
	mu.Unlock()
}

func TestAppLogger_Singleton(t *testing.T) {
	// Reset
	mu.Lock()
	appLogger = nil
	mu.Unlock()

	logger1 := AppLogger()
	logger2 := AppLogger()

	if logger1 != logger2 {
		t.Error("expected AppLogger to return the same instance")
	}
}

func TestDatabaseLogger_Singleton(t *testing.T) {
	// Reset
	mu.Lock()
	databaseLogger = nil
	mu.Unlock()

	logger1 := DatabaseLogger()
	logger2 := DatabaseLogger()

	if logger1 != logger2 {
		t.Error("expected DatabaseLogger to return the same instance")
	}
}
