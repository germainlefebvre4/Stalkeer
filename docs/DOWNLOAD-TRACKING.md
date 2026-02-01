# Download Tracking Enhancement Summary

## Overview

Enhanced the radarr and sonarr commands to properly track download information in the database using the new DownloadInfo model and StateManager.

## Changes Made

### 1. Enhanced Downloader.Download() Method

**Before:**
- Only updated ProcessedLine state
- Created DownloadInfo records only on failure
- No progress tracking during downloads
- No concurrency control

**After:**
- Creates/gets DownloadInfo record for every download
- Acquires database lock to prevent concurrent downloads
- Tracks download progress at configurable intervals
- Updates both DownloadInfo and ProcessedLine states
- Properly releases locks on completion or failure
- Persists download metadata (file size, path, completion time)

### 2. New Helper Methods

#### `getOrCreateDownloadInfo()`
- Gets existing DownloadInfo or creates new one
- Links DownloadInfo to ProcessedLine
- Ensures every download is tracked

#### `updateDownloadInfoCompleted()`
- Updates DownloadInfo with final details on success
- Sets download_path, file_size, completed_at
- Releases lock automatically
- Marks status as 'completed'

#### `updateProcessedLineState()`
- Updates ProcessedLine state for backward compatibility
- Maintains existing behavior for legacy code

### 3. State Tracking Flow

```
1. radarr/sonarr command calls dl.Download()
2. Download() creates/gets DownloadInfo record
3. Acquires lock on download (prevents duplicates)
4. Updates state to 'downloading'
5. During download:
   - Progress persisted every 10MB or 30 seconds
   - Both callbacks (user + internal) invoked
6. On completion:
   - Updates DownloadInfo: status='completed', file_size, download_path, completed_at
   - Updates ProcessedLine: state='downloaded'
   - Releases lock
7. On failure:
   - Updates DownloadInfo: status='failed', error_message
   - Updates ProcessedLine: state='failed'
   - Releases lock
```

### 4. Database Tracking Details

**DownloadInfo fields tracked:**
- `status` - Current download state (pending/downloading/completed/failed)
- `download_path` - Final file location
- `file_size` - Size in bytes
- `bytes_downloaded` - Progress for partial downloads
- `total_bytes` - Expected total size
- `started_at` - When download began
- `completed_at` - When download finished
- `locked_at` - Lock timestamp
- `locked_by` - Process that holds lock
- `retry_count` - Number of retry attempts
- `error_message` - Failure details

**ProcessedLine fields updated (backward compatibility):**
- `state` - Processing state (downloading/organizing/downloaded/failed)
- `download_info_id` - Link to DownloadInfo record

### 5. Benefits

1. **Complete Audit Trail**
   - Every download attempt recorded
   - Timestamps for all state transitions
   - Error messages preserved

2. **Resume Capability**
   - Downloads can be resumed after interruption
   - Progress tracking enables partial resume
   - Stale lock cleanup handles crashes

3. **Concurrency Control**
   - Database locks prevent duplicate downloads
   - Multiple instances can run safely
   - Lock timeout prevents deadlocks

4. **Progress Monitoring**
   - Real-time progress tracking
   - Periodic persistence (configurable intervals)
   - Statistics for bandwidth/time estimates

5. **Failure Analysis**
   - Retry count tracking
   - Error messages captured
   - Failed downloads identifiable

6. **Backward Compatibility**
   - ProcessedLine state still updated
   - Existing code continues to work
   - Gradual migration path

## Integration with radarr/sonarr Commands

No changes required to radarr/sonarr commands themselves - they continue to call:

```go
result, err := dl.Download(ctx, downloader.DownloadOptions{
    URL:             *processedLine.LineURL,
    BaseDestPath:    baseDestPath,
    TempDir:         cfg.Downloads.TempDir,
    ProcessedLineID: processedLine.ID,
    OnProgress:      func(downloaded, total int64) { /* ... */ },
})
```

The enhanced Download() method handles all tracking automatically.

## Database Schema

The existing DownloadInfo table (enhanced in Task 4.3) is used:

```sql
CREATE TABLE download_info (
    id SERIAL PRIMARY KEY,
    status VARCHAR(50) NOT NULL,
    download_path TEXT,
    file_size BIGINT,
    bytes_downloaded BIGINT DEFAULT 0,
    total_bytes BIGINT,
    resume_token VARCHAR(255),
    retry_count INTEGER DEFAULT 0 NOT NULL,
    last_retry_at TIMESTAMP,
    locked_at TIMESTAMP,
    locked_by VARCHAR(100),
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    error_message TEXT,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE INDEX idx_download_info_status ON download_info(status);
CREATE INDEX idx_download_info_locked_at ON download_info(locked_at);
CREATE INDEX idx_download_info_updated_at ON download_info(updated_at);
```

## Testing

- ✅ Build passes without errors
- ✅ All existing tests pass
- ✅ Downloader tests verify basic functionality
- ⏳ Integration tests with real downloads (manual verification recommended)

## Usage Examples

### Query Download Status

```sql
-- View all downloads
SELECT id, status, download_path, file_size, retry_count, created_at, completed_at
FROM download_info
ORDER BY created_at DESC;

-- View in-progress downloads
SELECT id, status, bytes_downloaded, total_bytes, 
       ROUND((bytes_downloaded::FLOAT / total_bytes) * 100, 1) as progress_pct
FROM download_info
WHERE status = 'downloading';

-- View failed downloads
SELECT id, retry_count, error_message, updated_at
FROM download_info
WHERE status = 'failed'
ORDER BY updated_at DESC;
```

### Resume Failed Downloads

```bash
# Resume all incomplete downloads
stalkeer resume-downloads

# Resume with radarr integration
stalkeer radarr --resume --limit 20
```

## Next Steps

Recommended enhancements:
1. Add integration tests for download tracking
2. Create admin UI for viewing download status
3. Add metrics/monitoring for download performance
4. Implement download queue visualization
5. Add notification system for failed downloads
