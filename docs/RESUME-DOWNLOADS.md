# Resume Downloads Feature

## Overview

The resume downloads feature provides robust state management for download operations, allowing downloads to be resumed after interruptions from system crashes, network failures, or application restarts.

## Architecture

### Components

1. **DownloadInfo Model** - Extended database model with comprehensive state tracking
2. **StateManager** - Manages download state transitions and locking
3. **ResumeSupport** - Handles HTTP range requests and partial file validation
4. **ResumeHelper** - Shared functionality for resuming downloads
5. **resume-downloads Command** - CLI command to manually trigger resume operations

### Database Schema

The `DownloadInfo` model has been extended with the following fields:

- `bytes_downloaded` - Progress tracking for partial downloads
- `total_bytes` - Expected total file size
- `resume_token` - Server-specific resume identifier (ETag, etc.)
- `retry_count` - Number of retry attempts
- `last_retry_at` - Timestamp of last retry
- `locked_at` - Lock timestamp to prevent concurrent downloads
- `locked_by` - Instance/process that acquired the lock

### State Machine

Download states:
- `pending` - Download queued but not started
- `downloading` - Download in progress
- `paused` - Download paused (can be resumed)
- `completed` - Download finished successfully
- `failed` - Download failed
- `retrying` - Download being retried after failure

## Usage

### CLI Commands

#### Resume Downloads Command

```bash
# Resume all incomplete downloads
stalkeer resume-downloads

# Preview what would be resumed (dry-run)
stalkeer resume-downloads --dry-run --verbose

# Resume up to 10 downloads with 5 concurrent workers
stalkeer resume-downloads --limit 10 --parallel 5

# Resume only failed downloads, skip pending
stalkeer resume-downloads --max-retries 3

# Clean stale locks before resuming
stalkeer resume-downloads --clean-stale-locks
```

#### Integration with Radarr/Sonarr

```bash
# Resume incomplete movie downloads before fetching new ones
stalkeer radarr --resume --limit 20

# Resume incomplete TV show downloads before fetching new episodes
stalkeer sonarr --resume --limit 20 --verbose
```

### Configuration

Add these settings to `config.yml`:

```yaml
downloads:
  # Resume downloads settings
  resume_enabled: true  # Enable resumable downloads
  progress_interval_mb: 10  # Persist progress every N megabytes
  progress_interval_seconds: 30  # Persist progress every N seconds
  lock_timeout_minutes: 5  # Consider locks older than this stale
  max_retry_attempts: 5  # Maximum retry attempts before giving up
```

## How It Works

### Download Lifecycle

1. **Initialization**: Download record created in database with `pending` status
2. **Lock Acquisition**: Process acquires lock to prevent duplicate downloads
3. **State Transition**: Status changes to `downloading` with timestamp
4. **Progress Tracking**: Progress persisted at intervals (every 10MB or 30 seconds)
5. **Completion**: Status changes to `completed`, lock released
6. **Failure**: Status changes to `failed`, retry count incremented

### Resume Process

1. **Query Database**: Find downloads in incomplete states (pending, downloading, paused, failed)
2. **Filter**: Exclude downloads exceeding max retry limit or locked by active processes
3. **Cleanup Stale Locks**: Remove locks older than timeout (default 5 minutes)
4. **Validate Partial Files**: Check if partial downloads exist and are valid
5. **Attempt Resume**: Use HTTP range requests where supported
6. **Fallback**: Restart download from beginning if resume not supported

### HTTP Range Request Support

The system automatically detects if servers support HTTP range requests:

1. Send HEAD request to check for `Accept-Ranges: bytes` header
2. If supported, send GET request with `Range: bytes=START-` header
3. Server responds with `206 Partial Content` and remaining data
4. If not supported, restart download from beginning

### Lock Mechanism

Prevents multiple processes from downloading the same file:

- Lock acquired before download starts
- Lock includes timestamp and instance identifier
- Stale locks (older than timeout) automatically cleaned up
- Lock released on completion or failure

## Error Handling

### Retryable Errors

- Network timeouts
- Server unavailable (5xx errors)
- Rate limiting (429)
- Connection errors

### Non-Retryable Errors

- Invalid URL (4xx errors except 429)
- File not found (404)
- Corrupted partial files
- Disk space errors

### Retry Strategy

- Initial retry: 2 seconds delay
- Exponential backoff with max 30 seconds
- Jitter to prevent thundering herd
- Maximum retry attempts configurable (default 5)

## Monitoring and Debugging

### Logging

Enable verbose logging for detailed information:

```yaml
logging:
  app:
    level: debug  # Enable debug logging
```

Log messages include:
- Download state transitions
- Lock acquisition/release
- Progress persistence
- Retry attempts
- Resume operations

### Statistics

The `resume-downloads` command provides statistics:

```
Resume operation completed:
  Total: 15
  Resumed: 12
  Failed: 2
  Skipped: 1
  Duration: 2m34s
```

### Common Issues

**Issue**: Downloads not resuming
- Check database for download records
- Verify locks aren't stale
- Check server supports range requests

**Issue**: Downloads restarting from beginning
- Server may not support HTTP range requests (fallback behavior)
- Partial file may be corrupted (automatic cleanup)

**Issue**: Stale locks preventing downloads
- Run with `--clean-stale-locks` flag
- Adjust `lock_timeout_minutes` configuration

## Best Practices

1. **Regular Resume Operations**: Schedule periodic `resume-downloads` runs to ensure all media is downloaded
2. **Monitor Retry Counts**: High retry counts may indicate persistent issues with specific URLs
3. **Disk Space**: Ensure adequate disk space for partial downloads (stored in temp directory)
4. **Timeout Configuration**: Adjust lock timeout based on average download times
5. **Concurrent Downloads**: Balance `max_parallel` setting with network bandwidth and server limits

## Troubleshooting

### Check Download State

Query database to inspect download records:

```sql
SELECT id, status, retry_count, bytes_downloaded, total_bytes, error_message
FROM download_info
WHERE status != 'completed'
ORDER BY updated_at DESC;
```

### Manual Lock Cleanup

If needed, manually clean stale locks:

```sql
UPDATE download_info
SET locked_at = NULL, locked_by = NULL
WHERE locked_at < NOW() - INTERVAL '5 minutes';
```

### Reset Failed Downloads

Reset downloads to retry:

```sql
UPDATE download_info
SET status = 'pending', retry_count = 0, error_message = NULL
WHERE status = 'failed' AND retry_count >= 5;
```

## Future Enhancements

Planned improvements:
- Checksum verification for partial downloads
- Distributed download coordination (multiple instances)
- Bandwidth throttling
- Download prioritization
- Web UI for download management
- Notification system for completion/failure

## Testing

Run tests for resume functionality:

```bash
# Run all downloader tests
go test ./internal/downloader/... -v

# Run specific test
go test ./internal/downloader -run TestStateManager -v

# Run with coverage
go test ./internal/downloader/... -cover
```

## API Integration

While the resume feature is primarily CLI-driven, the StateManager can be used programmatically:

```go
import "github.com/glefebvre/stalkeer/internal/downloader"

// Create state manager
sm := downloader.NewStateManager(downloader.DefaultStateManagerConfig())

// Get incomplete downloads
downloads, err := sm.GetIncompleteDownloads(ctx, maxRetries, limit)

// Acquire lock
err = sm.AcquireLock(ctx, downloadID)

// Update state
err = sm.UpdateState(ctx, downloadID, models.DownloadStatusDownloading, nil)

// Update progress
err = sm.UpdateProgress(ctx, downloadID, bytesDownloaded, totalBytes)

// Release lock
err = sm.ReleaseLock(ctx, downloadID)
```
