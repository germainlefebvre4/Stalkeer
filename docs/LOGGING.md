# Logging System

## Overview

Stalkeer uses a modularized logging system that allows independent control of application and database logging. This provides better visibility and tuning capabilities for production deployments.

## Configuration

### Modular Configuration (Recommended)

Configure application and database log levels independently:

```yaml
logging:
  format: json  # json or text
  
  app:
    level: info  # debug, info, warn, error
  database:
    level: warn  # debug, info, warn, error
```

### Legacy Configuration (Deprecated)

The legacy single log level configuration is still supported but will show a deprecation warning:

```yaml
logging:
  level: info  # debug, info, warn, error
  format: json
```

### Configuration Priority

Log levels are resolved in the following priority order:

1. Specific component level (`logging.app.level` or `logging.database.level`)
2. Legacy level (`logging.level`)
3. Default (`info`)

## Log Levels

### Application Log Levels

- **debug**: Detailed diagnostic information for development
- **info**: General informational messages about application flow
- **warn**: Warning messages about potential issues
- **error**: Error messages for failures and exceptions

### Database Log Levels

Database logging uses GORM's logger with the following mapping:

- **debug** → Shows all SQL queries with execution time
- **info** → Shows slow queries (>200ms) and errors
- **warn** → Shows slow queries and errors
- **error** → Shows only database errors

## Usage

### In Application Code

Use the `AppLogger()` function to get the application logger:

```go
import "github.com/glefebvre/stalkeer/internal/logger"

func MyFunction() {
    log := logger.AppLogger()
    log.Info("Processing started")
    
    if err := doSomething(); err != nil {
        log.Error("Failed to do something", err)
    }
}
```

### Initialization

Loggers are automatically initialized when the application starts. The main application initializes loggers after loading configuration:

```go
cfg := config.Get()
logger.InitializeLoggers(cfg.GetAppLogLevel(), cfg.GetDatabaseLogLevel())
```

### Testing

For testing, you can set custom loggers:

```go
import "github.com/glefebvre/stalkeer/internal/logger"

func TestMyFunction(t *testing.T) {
    // Create a test logger
    testLogger := logger.NewWithLevel("debug")
    logger.SetAppLogger(testLogger)
    
    // Your test code here
    
    // Cleanup
    logger.SetAppLogger(nil)
}
```

## Common Scenarios

### Production Deployment

Minimize logs while capturing important events:

```yaml
logging:
  format: json
  app:
    level: info
  database:
    level: error
```

### Development

Enable detailed logging for debugging:

```yaml
logging:
  format: json
  app:
    level: debug
  database:
    level: debug
```

### Performance Troubleshooting

Focus on database query performance:

```yaml
logging:
  format: json
  app:
    level: warn
  database:
    level: debug  # Shows all SQL queries
```

### Error Investigation

Minimize noise while debugging application errors:

```yaml
logging:
  format: json
  app:
    level: debug
  database:
    level: warn  # Only slow queries and errors
```

## Migration Guide

### From Legacy Configuration

If you're currently using:

```yaml
logging:
  level: info
  format: json
```

Migrate to:

```yaml
logging:
  format: json
  app:
    level: info
  database:
    level: warn  # or whatever level suits your needs
```

### Benefits of Migration

1. **Independent Control**: Tune application and database logging separately
2. **Better Performance**: Reduce database log overhead in production
3. **Improved Debugging**: Enable verbose database logs without flooding application logs
4. **Future-Proof**: New logging features will build on modular configuration

## Environment Variables

You can override configuration via environment variables:

```bash
# Legacy
export STALKEER_LOGGING_LEVEL=debug

# Modular (recommended)
export STALKEER_LOGGING_APP_LEVEL=debug
export STALKEER_LOGGING_DATABASE_LEVEL=warn
```

## Implementation Details

### Logger Singletons

The system uses singleton logger instances for efficiency:

- `AppLogger()` returns the application logger
- `DatabaseLogger()` returns the database logger

These are thread-safe and lazily initialized.

### GORM Integration

Database logging is integrated with GORM using a custom adapter (`logger.GormAdapter`) that:

- Maps application log levels to GORM log levels
- Formats SQL queries for readability
- Tracks slow query threshold (200ms)
- Handles query execution time logging

## Troubleshooting

### Deprecation Warning Appears

If you see:

```
Using deprecated 'logging.level' configuration. Please migrate to 'logging.app.level' and 'logging.database.level' for better control.
```

Update your `config.yml` to use the modular configuration format.

### Too Many Database Logs

If database logs are flooding your output, increase the database log level:

```yaml
logging:
  database:
    level: error  # Only show errors
```

### Missing Application Logs

Ensure the app log level is appropriate:

```yaml
logging:
  app:
    level: debug  # Show all logs
```

### Logs Not Appearing

1. Check that loggers are initialized after config loading
2. Verify log level configuration
3. Ensure you're using `logger.AppLogger()` not `logger.Default()`

## Future Enhancements

Potential future improvements to the logging system:

- Per-package log level configuration
- Dynamic log level changes via API
- Log output to separate files
- Structured logging fields standardization
- Log sampling and rate limiting
