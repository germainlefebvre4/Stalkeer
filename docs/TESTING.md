# Testing Guide

This document provides guidelines for testing the Stalkeer application.

## Overview

Stalkeer uses Go's built-in testing framework along with table-driven tests for comprehensive coverage. Tests are organized alongside the code they test, following Go conventions.

## Running Tests

### Run all tests
```bash
go test ./...
```

### Run tests with coverage
```bash
go test -cover ./...
```

### Run tests with detailed coverage report
```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Run tests with race detection
```bash
go test -race ./...
```

### Run specific package tests
```bash
go test ./internal/config
go test ./internal/models
```

## Test Structure

### Test Files

All test files follow the `_test.go` naming convention and are placed in the same package as the code they test.

### Test Database Setup

The `internal/testing` package provides helpers for setting up test databases:

```go
import (
    "testing"
    "github.com/glefebvre/stalkeer/internal/testing"
)

func TestSomething(t *testing.T) {
    db := testing.TestDB(t)
    defer testing.CleanupDB(t, db)
    
    // Your test logic here
}
```

### Fixtures

Use the provided fixture builders to create test data:

```go
// Create a movie playlist item
item := testing.CreatePlaylistItem(db)

// Create a TV show with custom properties
tvShow := testing.CreatePlaylistItem(db, 
    testing.WithTVShow(),
    testing.WithGroupTitle("TV Shows"),
)

// Create a filter configuration
filter := testing.CreateFilterConfig(db, testing.WithRuntimeFilter())

// Create a processing log
log := testing.CreateProcessingLog(db, 
    testing.WithStatus(models.StatusCompleted),
)
```

### Table-Driven Tests

Use table-driven tests for testing multiple scenarios:

```go
func TestSomething(t *testing.T) {
    tests := []testing.TableTest[InputType]{
        {
            Name:     "valid input",
            Input:    validInput,
            Expected: expectedOutput,
            WantErr:  false,
        },
        {
            Name:     "invalid input",
            Input:    invalidInput,
            Expected: nil,
            WantErr:  true,
        },
    }
    
    testing.RunTableTests(t, tests, func(t *testing.T, tc testing.TableTest[InputType]) {
        result, err := FunctionToTest(tc.Input)
        
        if tc.WantErr {
            testing.AssertNotNil(t, err, "expected error")
            return
        }
        
        testing.AssertNoError(t, err, "unexpected error")
        testing.AssertEqual(t, tc.Expected, result, "result mismatch")
    })
}
```

## Test Helpers

The `internal/testing` package provides several assertion helpers:

- `AssertNoError(t, err, message)` - Fails if error is not nil
- `AssertEqual(t, expected, actual, message)` - Fails if values don't match
- `AssertNotNil(t, value, message)` - Fails if value is nil
- `AssertCount(t, db, model, expected, message)` - Verifies record count

## Coverage Goals

- **Overall Coverage**: Aim for 80%+ coverage across the codebase
- **Critical Paths**: 90%+ coverage for core business logic
- **New Code**: All new features should include tests

## Best Practices

1. **Test Isolation**: Each test should be independent and not rely on other tests
2. **Clean Up**: Always clean up test data and resources
3. **Descriptive Names**: Use clear, descriptive test names that explain what is being tested
4. **Table-Driven**: Use table-driven tests for multiple similar test cases
5. **Error Cases**: Test both success and failure scenarios
6. **Edge Cases**: Include tests for boundary conditions and edge cases
7. **Mock External Dependencies**: Use mocks/stubs for external services

## Testing Database Migrations

To test database migrations:

```go
func TestMigrations(t *testing.T) {
    db := testing.TestDB(t)
    
    // Verify tables were created
    var tableCount int64
    db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&tableCount)
    
    if tableCount < 3 {
        t.Errorf("expected at least 3 tables, got %d", tableCount)
    }
}
```

## Integration Tests

For integration tests that require a full PostgreSQL database:

1. Use Docker Compose to start PostgreSQL:
   ```bash
   docker-compose up -d postgres
   ```

2. Set environment variables:
   ```bash
   export STALKEER_DATABASE_HOST=localhost
   export STALKEER_DATABASE_PORT=5432
   export STALKEER_DATABASE_USER=stalkeer
   export STALKEER_DATABASE_PASSWORD=stalkeer
   export STALKEER_DATABASE_DBNAME=stalkeer_test
   export STALKEER_M3U_FILE_PATH=/tmp/test.m3u
   ```

3. Run integration tests:
   ```bash
   go test -tags=integration ./...
   ```

## Continuous Integration

Tests are automatically run on every push and pull request via GitHub Actions. The CI pipeline:

- Runs all tests with race detection
- Generates code coverage reports
- Uploads coverage to Codecov
- Runs linting checks

## Troubleshooting

### Tests failing with "database not found"

Make sure to use the test database helpers:
```go
db := testing.TestDB(t)
```

### Tests interfering with each other

Always clean up between tests:
```go
defer testing.CleanupDB(t, db)
```

### Slow tests

Use `t.Parallel()` for tests that can run concurrently:
```go
func TestSomething(t *testing.T) {
    t.Parallel()
    // Test logic
}
```
