# Error Handling & Resilience

This document describes the error handling and resilience patterns implemented in Stalkeer.

## Overview

Stalkeer implements comprehensive error handling with:
- Custom error types with structured context
- Retry logic with exponential backoff
- Graceful shutdown mechanisms
- Circuit breaker pattern for external services
- Structured logging with context propagation

## Error Types

### Custom Error Package (`internal/errors`)

The application uses custom error types defined in `internal/errors/errors.go`:

```go
type AppError struct {
    Code    ErrorCode
    Message string
    Err     error
    Context map[string]interface{}
}
```

#### Error Codes

- **Validation Errors**: `CodeValidation`, `CodeInvalidInput`
- **Database Errors**: `CodeDatabase`, `CodeDatabaseConnection`, `CodeDatabaseQuery`, `CodeDatabaseTransaction`
- **Parse Errors**: `CodeParse`, `CodeInvalidFormat`, `CodeMalformedData`
- **Classification Errors**: `CodeClassification`, `CodeUnknownContentType`
- **External Service Errors**: `CodeExternalService`, `CodeServiceUnavailable`, `CodeServiceTimeout`, `CodeUnauthorized`, `CodeRateLimited`
- **Config Errors**: `CodeConfig`, `CodeMissingConfig`, `CodeInvalidConfig`
- **Internal Errors**: `CodeInternal`, `CodeUnknown`

#### Creating Errors

```go
// Simple error
err := errors.ValidationError("invalid input format")

// Wrapped error with context
err := errors.DatabaseError("failed to query items", originalErr)

// Error with additional context
err := errors.ExternalServiceError("radarr", "connection failed", originalErr).
    WithContext("attempt", 3).
    WithContext("url", "http://radarr:7878")
```

#### Checking Errors

```go
// Check if error is retryable
if errors.IsRetryable(err) {
    // Retry logic
}

// Get error code
code := errors.GetErrorCode(err)
if code == errors.CodeServiceTimeout {
    // Handle timeout specifically
}
```

## Retry Logic

### Retry Package (`internal/retry`)

Implements exponential backoff with jitter for transient failures.

#### Configuration

```go
cfg := retry.Config{
    MaxAttempts:       3,
    InitialBackoff:    100 * time.Millisecond,
    MaxBackoff:        30 * time.Second,
    BackoffMultiplier: 2.0,
    JitterFraction:    0.1,
}
```

#### Usage

```go
// Simple retry
err := retry.Do(ctx, cfg, func() error {
    return someOperation()
}, errors.IsRetryable)

// Retry with result
result, err := retry.DoWithResult(ctx, cfg, func() (Data, error) {
    return fetchData()
}, errors.IsRetryable)
```

#### Features

- Context-aware cancellation
- Exponential backoff with configurable multiplier
- Random jitter to prevent thundering herd
- Configurable retry predicate
- Generic support for functions returning values

## Graceful Shutdown

### Shutdown Package (`internal/shutdown`)

Manages graceful shutdown of application components.

#### Usage

```go
// Create handler with 30-second timeout
handler := shutdown.New(30 * time.Second)

// Register cleanup functions (executed in LIFO order)
handler.Register(func(ctx context.Context) error {
    log.Info("Shutting down HTTP server")
    return server.Shutdown(ctx)
})

handler.Register(func(ctx context.Context) error {
    log.Info("Closing database connection")
    return database.Close()
})

// Wait for shutdown signal (SIGINT, SIGTERM)
handler.Wait()
```

#### Features

- Signal handling (SIGINT, SIGTERM)
- Configurable shutdown timeout
- LIFO execution order (reverse of registration)
- Concurrent shutdown function execution
- Idempotent (safe to call multiple times)
- Shutdown channel for notification

## Circuit Breaker

### Circuit Breaker Package (`internal/circuitbreaker`)

Implements circuit breaker pattern to prevent cascading failures in external service calls.

#### States

- **Closed**: Normal operation, requests pass through
- **Open**: Too many failures, requests rejected immediately
- **Half-Open**: Testing recovery, limited requests allowed

#### Configuration

```go
cfg := circuitbreaker.Config{
    MaxFailures:         5,
    Timeout:             60 * time.Second,
    MaxHalfOpenRequests: 1,
    IsSuccessful: func(err error) bool {
        return err == nil
    },
}
```

#### Usage

```go
cb := circuitbreaker.New(cfg)

err := cb.Execute(func() error {
    return callExternalService()
})

if err == circuitbreaker.ErrOpenState {
    log.Warn("Circuit breaker is open, service unavailable")
}
```

#### Features

- Automatic state transitions
- Configurable failure threshold
- Configurable recovery timeout
- Custom success predicate
- Thread-safe operation
- Manual reset capability

## Integration Examples

### HTTP Server with Graceful Shutdown

```go
// Create shutdown handler
shutdownHandler := shutdown.New(30 * time.Second)

// Create server
server := api.NewServer()

// Register server shutdown
shutdownHandler.Register(func(ctx context.Context) error {
    return server.Shutdown(ctx)
})

// Register database cleanup
shutdownHandler.Register(func(ctx context.Context) error {
    return database.Close()
})

// Start server
go func() {
    if err := server.Run(port); err != nil && err != http.ErrServerClosed {
        log.Error("server error", err)
    }
}()

// Wait for shutdown
shutdownHandler.Wait()
```

### External API Call with Retry and Circuit Breaker

```go
cb := circuitbreaker.New(circuitbreaker.DefaultConfig())
retryCfg := retry.DefaultConfig()

result, err := retry.DoWithResult(ctx, retryCfg, func() (Data, error) {
    var data Data
    err := cb.Execute(func() error {
        var execErr error
        data, execErr = api.FetchData()
        return execErr
    })
    return data, err
}, errors.IsRetryable)

if err != nil {
    log.Error("failed to fetch data after retries", err)
}
```

### Database Operations with Error Handling

```go
items, err := db.GetItems(filter)
if err != nil {
    return errors.DatabaseError("failed to query items", err).
        WithContext("filter", filter)
}
```

## Best Practices

1. **Always wrap errors with context**
   ```go
   return errors.Wrap(err, errors.CodeDatabase, "failed to save item").
       WithContext("item_id", item.ID)
   ```

2. **Use appropriate error codes**
   - Helps with error categorization and monitoring
   - Enables proper retry logic

3. **Log errors at appropriate levels**
   - ERROR: Unexpected failures requiring attention
   - WARN: Recoverable errors or degraded performance
   - INFO: Normal operational messages

4. **Register shutdown handlers in reverse dependency order**
   - Shutdown HTTP server before database
   - Close connections before releasing resources

5. **Use circuit breakers for external services**
   - Prevents cascading failures
   - Provides fast failure when service is down

6. **Configure retry policies per operation**
   - Database: Shorter backoff, more attempts
   - External APIs: Longer backoff, fewer attempts
   - Critical operations: No retry, fail fast

## Testing

All error handling packages have comprehensive test coverage:

```bash
# Run all error handling tests
go test ./internal/errors -v
go test ./internal/retry -v
go test ./internal/shutdown -v
go test ./internal/circuitbreaker -v
```

### Test Coverage

- Custom error types: 100%
- Retry logic: 100%
- Shutdown handler: 100%
- Circuit breaker: 100%

## Future Enhancements

- [ ] Metrics collection for error rates and types
- [ ] Distributed tracing integration
- [ ] Alert rules based on error codes
- [ ] Error rate limiting per endpoint
- [ ] Automatic retry budget management
