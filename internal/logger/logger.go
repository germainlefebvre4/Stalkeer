package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"time"
)

// Level represents the severity level of a log entry
type Level string

const (
	LevelDebug Level = "DEBUG"
	LevelInfo  Level = "INFO"
	LevelWarn  Level = "WARN"
	LevelError Level = "ERROR"
)

// contextKey is the type used for context keys
type contextKey string

const (
	requestIDKey contextKey = "request_id"
	userIDKey    contextKey = "user_id"
)

// Package-level logger instances
var (
	appLogger      *Logger
	databaseLogger *Logger
	appOnce        sync.Once
	dbOnce         sync.Once
	mu             sync.RWMutex
)

// Entry represents a single log entry
type Entry struct {
	Timestamp string                 `json:"timestamp"`
	Level     Level                  `json:"level"`
	Message   string                 `json:"message"`
	Context   map[string]interface{} `json:"context,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Stack     []string               `json:"stack,omitempty"`
}

// Logger provides structured logging functionality
type Logger struct {
	output    io.Writer
	minLevel  Level
	withStack bool
}

// Config holds logger configuration
type Config struct {
	Output    io.Writer
	MinLevel  Level
	WithStack bool
}

// New creates a new logger with the given configuration
func New(cfg Config) *Logger {
	if cfg.Output == nil {
		cfg.Output = os.Stdout
	}
	if cfg.MinLevel == "" {
		cfg.MinLevel = LevelInfo
	}

	return &Logger{
		output:    cfg.Output,
		minLevel:  cfg.MinLevel,
		withStack: cfg.WithStack,
	}
}

// Default creates a logger with default configuration
func Default() *Logger {
	return New(Config{
		Output:    os.Stdout,
		MinLevel:  LevelInfo,
		WithStack: false,
	})
}

// NewWithLevel creates a new logger with a specific log level string
func NewWithLevel(level string) *Logger {
	logLevel := parseLevel(level)
	return New(Config{
		Output:    os.Stdout,
		MinLevel:  logLevel,
		WithStack: logLevel == LevelDebug,
	})
}

// AppLogger returns the singleton application logger instance
func AppLogger() *Logger {
	mu.RLock()
	if appLogger != nil {
		mu.RUnlock()
		return appLogger
	}
	mu.RUnlock()

	appOnce.Do(func() {
		mu.Lock()
		defer mu.Unlock()
		appLogger = Default()
	})

	mu.RLock()
	defer mu.RUnlock()
	return appLogger
}

// DatabaseLogger returns the singleton database logger instance
func DatabaseLogger() *Logger {
	mu.RLock()
	if databaseLogger != nil {
		mu.RUnlock()
		return databaseLogger
	}
	mu.RUnlock()

	dbOnce.Do(func() {
		mu.Lock()
		defer mu.Unlock()
		databaseLogger = Default()
	})

	mu.RLock()
	defer mu.RUnlock()
	return databaseLogger
}

// SetAppLogger sets the application logger (primarily for testing)
func SetAppLogger(logger *Logger) {
	mu.Lock()
	defer mu.Unlock()
	appLogger = logger
}

// SetDatabaseLogger sets the database logger (primarily for testing)
func SetDatabaseLogger(logger *Logger) {
	mu.Lock()
	defer mu.Unlock()
	databaseLogger = logger
}

// InitializeLoggers initializes both app and database loggers with specified levels
func InitializeLoggers(appLevel, dbLevel string) {
	mu.Lock()
	defer mu.Unlock()

	appLogger = NewWithLevel(appLevel)
	databaseLogger = NewWithLevel(dbLevel)
}

// parseLevel converts a string log level to a Level type
func parseLevel(level string) Level {
	switch level {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

// Debug logs a debug message
func (l *Logger) Debug(msg string) {
	l.log(LevelDebug, msg, nil, nil)
}

// DebugContext logs a debug message with context
func (l *Logger) DebugContext(ctx context.Context, msg string) {
	l.logContext(ctx, LevelDebug, msg, nil, nil)
}

// Info logs an info message
func (l *Logger) Info(msg string) {
	l.log(LevelInfo, msg, nil, nil)
}

// InfoContext logs an info message with context
func (l *Logger) InfoContext(ctx context.Context, msg string) {
	l.logContext(ctx, LevelInfo, msg, nil, nil)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string) {
	l.log(LevelWarn, msg, nil, nil)
}

// WarnContext logs a warning message with context
func (l *Logger) WarnContext(ctx context.Context, msg string) {
	l.logContext(ctx, LevelWarn, msg, nil, nil)
}

// Error logs an error message
func (l *Logger) Error(msg string, err error) {
	l.log(LevelError, msg, nil, err)
}

// ErrorContext logs an error message with context
func (l *Logger) ErrorContext(ctx context.Context, msg string, err error) {
	l.logContext(ctx, LevelError, msg, nil, err)
}

// WithFields returns a new logger with additional fields
func (l *Logger) WithFields(fields map[string]interface{}) *FieldLogger {
	return &FieldLogger{
		logger: l,
		fields: fields,
	}
}

// log performs the actual logging
func (l *Logger) log(level Level, msg string, context map[string]interface{}, err error) {
	if !l.shouldLog(level) {
		return
	}

	entry := Entry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     level,
		Message:   msg,
		Context:   context,
	}

	if err != nil {
		entry.Error = err.Error()
		if l.withStack && level == LevelError {
			entry.Stack = getStackTrace()
		}
	}

	data, _ := json.Marshal(entry)
	fmt.Fprintln(l.output, string(data))
}

// logContext logs with context values
func (l *Logger) logContext(ctx context.Context, level Level, msg string, fields map[string]interface{}, err error) {
	if !l.shouldLog(level) {
		return
	}

	context := make(map[string]interface{})

	// Add context values
	if requestID := ctx.Value(requestIDKey); requestID != nil {
		context["request_id"] = requestID
	}
	if userID := ctx.Value(userIDKey); userID != nil {
		context["user_id"] = userID
	}

	// Merge additional fields
	for k, v := range fields {
		context[k] = v
	}

	l.log(level, msg, context, err)
}

// shouldLog checks if the log level should be logged
func (l *Logger) shouldLog(level Level) bool {
	levels := map[Level]int{
		LevelDebug: 0,
		LevelInfo:  1,
		LevelWarn:  2,
		LevelError: 3,
	}

	return levels[level] >= levels[l.minLevel]
}

// getStackTrace captures the current stack trace
func getStackTrace() []string {
	const maxDepth = 32
	var pcs [maxDepth]uintptr
	n := runtime.Callers(3, pcs[:])

	frames := runtime.CallersFrames(pcs[:n])
	stack := make([]string, 0, n)

	for {
		frame, more := frames.Next()
		stack = append(stack, fmt.Sprintf("%s:%d %s", frame.File, frame.Line, frame.Function))
		if !more {
			break
		}
	}

	return stack
}

// FieldLogger is a logger with pre-set fields
type FieldLogger struct {
	logger *Logger
	fields map[string]interface{}
}

// Debug logs a debug message with fields
func (fl *FieldLogger) Debug(msg string) {
	fl.logger.log(LevelDebug, msg, fl.fields, nil)
}

// DebugContext logs a debug message with fields and context
func (fl *FieldLogger) DebugContext(ctx context.Context, msg string) {
	fl.logger.logContext(ctx, LevelDebug, msg, fl.fields, nil)
}

// Info logs an info message with fields
func (fl *FieldLogger) Info(msg string) {
	fl.logger.log(LevelInfo, msg, fl.fields, nil)
}

// InfoContext logs an info message with fields and context
func (fl *FieldLogger) InfoContext(ctx context.Context, msg string) {
	fl.logger.logContext(ctx, LevelInfo, msg, fl.fields, nil)
}

// Warn logs a warning message with fields
func (fl *FieldLogger) Warn(msg string) {
	fl.logger.log(LevelWarn, msg, fl.fields, nil)
}

// WarnContext logs a warning message with fields and context
func (fl *FieldLogger) WarnContext(ctx context.Context, msg string) {
	fl.logger.logContext(ctx, LevelWarn, msg, fl.fields, nil)
}

// Error logs an error message with fields
func (fl *FieldLogger) Error(msg string, err error) {
	fl.logger.log(LevelError, msg, fl.fields, err)
}

// ErrorContext logs an error message with fields and context
func (fl *FieldLogger) ErrorContext(ctx context.Context, msg string, err error) {
	fl.logger.logContext(ctx, LevelError, msg, fl.fields, err)
}

// ContextWithRequestID adds a request ID to the context
func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// ContextWithUserID adds a user ID to the context
func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}
