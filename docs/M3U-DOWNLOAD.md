# M3U Playlist Download and Archive Management

This feature enables automatic downloading of M3U playlist files from remote sources with built-in archiving and rotation capabilities.

## Overview

The M3U Download feature provides:

- **Automated Downloads**: Download M3U playlists from HTTP/HTTPS URLs
- **Atomic Operations**: Safe, atomic file updates prevent corruption
- **Validation**: Verify M3U format before accepting downloads
- **Archive Management**: Automatic timestamped archiving of playlist versions
- **Rotation**: Keep only the N most recent archives to manage disk space
- **Error Handling**: Retry mechanism with circuit breaker for reliability
- **Security**: File size limits, content validation, and HTTPS support

## Configuration

Add the following to your `config.yml`:

```yaml
m3u:
  file_path: /path/to/playlist.m3u  # Required: Where to save the M3U file
  
  download:
    enabled: false  # Enable M3U download feature
    url: "https://example.com/playlist.m3u"  # Remote M3U URL
    archive_dir: ./m3u_playlist  # Directory for archived files
    retention_count: 5  # Number of archives to keep
    max_file_size_mb: 500  # Maximum file size limit
    timeout_seconds: 300  # Download timeout (5 minutes)
    retry_attempts: 3  # Number of retry attempts
    
    # Optional: HTTP Basic Authentication
    auth_username: ""
    auth_password: ""
    
    # Optional: Scheduled downloads (future feature)
    schedule_enabled: false
    interval_hours: 24
```

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | boolean | `false` | Enable/disable M3U download feature |
| `url` | string | `""` | Remote M3U playlist URL (HTTP/HTTPS) |
| `archive_dir` | string | `./m3u_playlist` | Directory to store archived M3U files |
| `retention_count` | integer | `5` | Number of archived files to retain |
| `max_file_size_mb` | integer | `500` | Maximum allowed file size in megabytes |
| `timeout_seconds` | integer | `300` | Download timeout in seconds |
| `retry_attempts` | integer | `3` | Number of retry attempts on failure |
| `auth_username` | string | `""` | HTTP Basic Auth username (optional) |
| `auth_password` | string | `""` | HTTP Basic Auth password (optional) |

### Environment Variables

You can also configure via environment variables:

```bash
export STALKEER_M3U_FILE_PATH=/path/to/playlist.m3u
export STALKEER_M3U_DOWNLOAD_ENABLED=true
export STALKEER_M3U_DOWNLOAD_URL=https://example.com/playlist.m3u
export STALKEER_M3U_DOWNLOAD_ARCHIVE_DIR=./m3u_playlist
export STALKEER_M3U_DOWNLOAD_RETENTION_COUNT=5
```

## Usage

### Download M3U Playlist

Download the M3U playlist from the configured URL:

```bash
stalkeer m3u-download
```

This will:
1. Download the M3U file from the configured URL
2. Validate the M3U format
3. Save atomically to `m3u.file_path`
4. Create a timestamped archive copy
5. Rotate old archives based on retention settings

**With custom URL:**

```bash
stalkeer m3u-download --url https://example.com/custom-playlist.m3u
```

**Without archiving:**

```bash
stalkeer m3u-download --no-archive
```

### List Archived Playlists

View all archived M3U files:

```bash
stalkeer m3u-list-archives
```

Output example:
```
Archived M3U files (./m3u_playlist):

Filename                                 Size         Modified
--------------------------------------------------------------------------------
playlist_20260203_230154.054749.m3u      40.96 MB     2026-02-03 23:01:54
playlist_20260203_225902.373353.m3u      40.96 MB     2026-02-03 22:59:02
playlist_20260202_180000.123456.m3u      40.95 MB     2026-02-02 18:00:00
playlist_20260201_120000.000000.m3u      40.94 MB     2026-02-01 12:00:00
playlist_20260131_160000.000000.m3u      40.93 MB     2026-01-31 16:00:00

Total: 5 archived files
```

### Clean Up Old Archives

Manually trigger archive rotation:

```bash
# Use configured retention count
stalkeer m3u-cleanup-archives

# Keep only 3 most recent
stalkeer m3u-cleanup-archives --retention 3
```

## How It Works

### Download Workflow

1. **Request**: HTTP GET request to the configured URL
2. **Validation**: 
   - Check HTTP status code (200 OK)
   - Verify content type (if provided)
   - Enforce file size limits
3. **Content Validation**: Verify M3U format (`#EXTM3U` header)
4. **Atomic Write**: 
   - Download to temporary file
   - Validate content
   - Atomic rename to `m3u.file_path`
5. **Archive**: Create timestamped copy in archive directory
6. **Rotation**: Delete archives beyond retention count

### Archive Filename Format

Archives use the following naming convention:

```
playlist_YYYYMMDD_HHMMSS.ffffff.m3u
```

Example: `playlist_20260203_230154.054749.m3u`

- `YYYYMMDD`: Date (2026-02-03)
- `HHMMSS`: Time (23:01:54)
- `ffffff`: Microseconds (054749)

### Error Handling

The download system includes multiple layers of error handling:

1. **Retry Mechanism**: Automatically retries transient failures
   - Configurable retry attempts
   - Exponential backoff with jitter
   - Retries network errors and 5xx HTTP errors
   - Skips 4xx errors (client errors)

2. **Circuit Breaker**: Prevents repeated failures
   - Opens after 5 consecutive failures
   - Waits 60 seconds before retry
   - Protects against unreliable remote servers

3. **Validation**: Prevents corruption
   - M3U format validation
   - File size enforcement
   - Atomic file operations

4. **Preservation**: Original file safety
   - Downloads to temporary file first
   - Original `m3u.file_path` unchanged on failure
   - Only replaced after successful validation

## Security Considerations

### File Size Limits

The `max_file_size_mb` setting prevents disk exhaustion attacks:

```yaml
m3u:
  download:
    max_file_size_mb: 500  # Reject files larger than 500MB
```

### HTTPS Support

Use HTTPS URLs for secure downloads:

```yaml
m3u:
  download:
    url: "https://example.com/playlist.m3u"  # HTTPS recommended
```

### Authentication

Support for HTTP Basic Authentication:

```yaml
m3u:
  download:
    url: "https://example.com/playlist.m3u"
    auth_username: "myuser"
    auth_password: "mypassword"
```

**Security Note**: Store credentials in environment variables instead of config files:

```bash
export STALKEER_M3U_DOWNLOAD_AUTH_USERNAME=myuser
export STALKEER_M3U_DOWNLOAD_AUTH_PASSWORD=mypassword
```

## Troubleshooting

### Download Fails with Timeout

**Problem**: Downloads timeout for large files

**Solution**: Increase timeout setting:

```yaml
m3u:
  download:
    timeout_seconds: 600  # 10 minutes
```

### Invalid M3U Format Error

**Problem**: Downloaded file fails validation

**Solution**: Check the remote URL returns a valid M3U file:

```bash
curl -I https://example.com/playlist.m3u
# Should return Content-Type: application/vnd.apple.mpegurl or similar
```

### File Size Exceeded Error

**Problem**: File larger than configured limit

**Solution**: Increase the limit or verify URL is correct:

```yaml
m3u:
  download:
    max_file_size_mb: 1000  # Increase limit
```

### Authentication Failures

**Problem**: 401 Unauthorized errors

**Solution**: Verify credentials are correct:

```bash
# Test with curl
curl -u username:password https://example.com/playlist.m3u
```

### Archive Directory Permission Error

**Problem**: Cannot create archive directory

**Solution**: Ensure write permissions:

```bash
mkdir -p ./m3u_playlist
chmod 755 ./m3u_playlist
```

### Circuit Breaker Open

**Problem**: "circuit breaker is open" error

**Solution**: Wait 60 seconds or fix the remote URL. The circuit breaker automatically resets after the timeout period.

## Integration with Process Command

After downloading an M3U playlist, process it:

```bash
# Download latest playlist
stalkeer m3u-download

# Process the downloaded playlist
stalkeer process
```

## Best Practices

1. **Regular Downloads**: Set up a cron job for periodic downloads
   ```bash
   # Download playlist daily at 2 AM
   0 2 * * * /usr/local/bin/stalkeer m3u-download
   ```

2. **Monitor Archives**: Check archive count periodically
   ```bash
   stalkeer m3u-list-archives
   ```

3. **Retention Policy**: Adjust based on your needs
   - Keep 5-10 archives for debugging
   - Keep 1-2 for minimal storage

4. **Backup Archive Directory**: Include in backups
   ```bash
   tar -czf m3u-archives-backup.tar.gz ./m3u_playlist
   ```

5. **Use HTTPS**: Always prefer HTTPS URLs for security

6. **Environment Variables**: Store sensitive data in environment variables

## Future Enhancements

Planned features for future releases:

- **Scheduled Downloads**: Automatic periodic downloads
- **Content Hashing**: Skip redundant downloads when content unchanged
- **Webhook Notifications**: Notify on successful/failed downloads
- **S3 Archive Storage**: Store archives in cloud storage
- **Download Metrics**: Track download history and statistics

## Related Documentation

- [Configuration Management](DEVELOPMENT.md#configuration)
- [Error Handling](ERROR-HANDLING.md)
- [Logging System](LOGGING.md)

## Support

For issues or questions:

1. Check the [Troubleshooting](#troubleshooting) section
2. Review logs for detailed error messages
3. Open an issue on GitHub
