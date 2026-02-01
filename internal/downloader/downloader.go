package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/glefebvre/stalkeer/internal/database"
	"github.com/glefebvre/stalkeer/internal/errors"
	"github.com/glefebvre/stalkeer/internal/models"
	"github.com/glefebvre/stalkeer/internal/retry"
	"github.com/google/uuid"
)

// DownloadOptions holds configuration for a download operation
type DownloadOptions struct {
	URL             string
	BaseDestPath    string // Path without extension
	ProcessedLineID uint
	OnProgress      func(downloaded, total int64)
	Timeout         time.Duration
	RetryAttempts   int
	TempDir         string // Optional temp directory (empty = use OS temp)
}

// DownloadResult contains information about a completed download
type DownloadResult struct {
	FilePath      string
	TempPath      string // Temp path used (for debugging)
	FileSize      int64
	Extension     string
	Duration      time.Duration
	BytesRead     int64
	MoveDuration  time.Duration
}

// Downloader handles media file downloads
type Downloader struct {
	httpClient  *http.Client
	retryConfig retry.Config
}

// New creates a new Downloader instance
func New(timeout time.Duration, retryAttempts int) *Downloader {
	if timeout == 0 {
		timeout = 300 * time.Second // 5 minutes default
	}

	if retryAttempts == 0 {
		retryAttempts = 3
	}

	return &Downloader{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		retryConfig: retry.Config{
			MaxAttempts:       retryAttempts,
			InitialBackoff:    2 * time.Second,
			MaxBackoff:        30 * time.Second,
			BackoffMultiplier: 2.0,
			JitterFraction:    0.1,
		},
	}
}

// Download downloads a file from the given URL to the destination path
func (d *Downloader) Download(ctx context.Context, opts DownloadOptions) (*DownloadResult, error) {
	startTime := time.Now()

	// Validate inputs
	if opts.URL == "" {
		return nil, errors.ValidationError("download URL cannot be empty")
	}
	if opts.BaseDestPath == "" {
		return nil, errors.ValidationError("base destination path cannot be empty")
	}

	// Update state to downloading if ProcessedLineID is provided
	if opts.ProcessedLineID > 0 {
		if err := d.updateDownloadState(opts.ProcessedLineID, models.StateDownloading, nil); err != nil {
			return nil, err
		}
	}

	// Create unique temp directory
	tempDir := opts.TempDir
	if tempDir == "" {
		tempDir = os.TempDir()
	}
	tempDownloadDir := filepath.Join(tempDir, fmt.Sprintf("stalkeer-download-%s", uuid.New().String()))
	if err := os.MkdirAll(tempDownloadDir, 0755); err != nil {
		return nil, errors.Wrap(err, errors.CodeInternal, "failed to create temp directory")
	}
	defer os.RemoveAll(tempDownloadDir) // Clean up temp dir

	// Create temporary file
	tempPath := filepath.Join(tempDownloadDir, "download.tmp")

	// Perform download with retry
	var result *DownloadResult
	var contentType string
	err := retry.Do(ctx, d.retryConfig, func() error {
		res, ct, err := d.downloadFile(ctx, opts.URL, tempPath, opts.OnProgress)
		if err != nil {
			return err
		}
		result = res
		contentType = ct
		return nil
	}, errors.IsRetryable)

	if err != nil {
		if opts.ProcessedLineID > 0 {
			errMsg := err.Error()
			d.updateDownloadState(opts.ProcessedLineID, models.StateFailed, &errMsg)
		}
		return nil, errors.ExternalServiceError("download", "failed to download file", err)
	}

	// Detect file extension
	ext := detectFileExtension(opts.URL, contentType)
	result.Extension = ext

	// Construct final destination path with extension
	finalDestPath := opts.BaseDestPath + ext

	// Create destination directory
	destDir := filepath.Dir(finalDestPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return nil, errors.Wrap(err, errors.CodeInternal, "failed to create destination directory")
	}

	// Update state to organizing
	if opts.ProcessedLineID > 0 {
		if err := d.updateDownloadState(opts.ProcessedLineID, models.StateOrganizing, nil); err != nil {
			return nil, err
		}
	}

	// Move file to final destination
	moveStart := time.Now()
	if err := moveFile(tempPath, finalDestPath); err != nil {
		if opts.ProcessedLineID > 0 {
			errMsg := err.Error()
			d.updateDownloadState(opts.ProcessedLineID, models.StateFailed, &errMsg)
		}
		return nil, errors.Wrap(err, errors.CodeInternal, "failed to move file to destination")
	}

	// Set proper file permissions
	if err := os.Chmod(finalDestPath, 0644); err != nil {
		return nil, errors.Wrap(err, errors.CodeInternal, "failed to set file permissions")
	}

	result.FilePath = finalDestPath
	result.TempPath = tempPath
	result.Duration = time.Since(startTime)
	result.MoveDuration = time.Since(moveStart)

	// Update state to downloaded
	if opts.ProcessedLineID > 0 {
		if err := d.updateDownloadState(opts.ProcessedLineID, models.StateDownloaded, nil); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// downloadFile performs the actual HTTP download
func (d *Downloader) downloadFile(ctx context.Context, url, destPath string, onProgress func(int64, int64)) (*DownloadResult, string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Get content type for extension detection
	contentType := resp.Header.Get("Content-Type")

	out, err := os.Create(destPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	// Download with progress tracking
	var bytesRead int64
	contentLength := resp.ContentLength

	if onProgress != nil && contentLength > 0 {
		// Use TeeReader to track progress
		reader := &progressReader{
			reader:     resp.Body,
			total:      contentLength,
			downloaded: 0,
			onProgress: onProgress,
		}
		bytesRead, err = io.Copy(out, reader)
	} else {
		bytesRead, err = io.Copy(out, resp.Body)
	}

	if err != nil {
		return nil, "", fmt.Errorf("failed to write file: %w", err)
	}

	return &DownloadResult{
		FileSize:  bytesRead,
		BytesRead: bytesRead,
	}, contentType, nil
}

// updateDownloadState updates the download state in the database
func (d *Downloader) updateDownloadState(processedLineID uint, state models.ProcessingState, errorMsg *string) error {
	db := database.Get()
	if db == nil {
		return errors.New(errors.CodeInternal, "database not initialized")
	}

	updates := map[string]interface{}{
		"state":      state,
		"updated_at": time.Now(),
	}

	var processedLine models.ProcessedLine
	if err := db.First(&processedLine, processedLineID).Error; err != nil {
		return errors.DatabaseError("failed to fetch processed line", err)
	}

	// Create or update DownloadInfo if error occurred
	if errorMsg != nil && state == models.StateFailed {
		downloadInfo := models.DownloadInfo{
			Status:       "failed",
			ErrorMessage: errorMsg,
			UpdatedAt:    time.Now(),
		}

		if processedLine.DownloadInfoID != nil {
			// Update existing
			if err := db.Model(&models.DownloadInfo{}).
				Where("id = ?", *processedLine.DownloadInfoID).
				Updates(&downloadInfo).Error; err != nil {
				return errors.DatabaseError("failed to update download info", err)
			}
		} else {
			// Create new
			downloadInfo.CreatedAt = time.Now()
			if err := db.Create(&downloadInfo).Error; err != nil {
				return errors.DatabaseError("failed to create download info", err)
			}
			downloadInfoID := downloadInfo.ID
			updates["download_info_id"] = &downloadInfoID
		}
	}

	if err := db.Model(&models.ProcessedLine{}).
		Where("id = ?", processedLineID).
		Updates(updates).Error; err != nil {
		return errors.DatabaseError("failed to update processed line state", err)
	}

	return nil
}

// progressReader wraps an io.Reader to report progress
type progressReader struct {
	reader     io.Reader
	total      int64
	downloaded int64
	onProgress func(downloaded, total int64)
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.downloaded += int64(n)
	if pr.onProgress != nil {
		pr.onProgress(pr.downloaded, pr.total)
	}
	return n, err
}

// detectFileExtension detects file extension from URL or Content-Type header
func detectFileExtension(url string, contentType string) string {
// 1. Try URL path
if ext := filepath.Ext(url); ext != "" {
// Clean up query parameters if present
if idx := strings.Index(ext, "?"); idx != -1 {
ext = ext[:idx]
}
if ext != "" {
return ext
}
}

// 2. Try Content-Type mapping
extMap := map[string]string{
"video/x-matroska":  ".mkv",
"video/mp4":         ".mp4",
"video/x-msvideo":   ".avi",
"video/quicktime":   ".mov",
"video/x-flv":       ".flv",
"video/webm":        ".webm",
"video/mpeg":        ".mpg",
"video/3gpp":        ".3gp",
"video/x-ms-wmv":    ".wmv",
"application/x-mpegURL": ".m3u8",
}

// Clean content type (remove charset, etc.)
if idx := strings.Index(contentType, ";"); idx != -1 {
contentType = strings.TrimSpace(contentType[:idx])
}

if ext, ok := extMap[contentType]; ok {
return ext
}

// 3. Default to .mkv
return ".mkv"
}

// moveFile moves a file from src to dst, trying rename first, then copy+verify+delete
func moveFile(src, dst string) error {
// Try rename first (fast, atomic)
if err := os.Rename(src, dst); err == nil {
return nil
}

// Fallback: copy + verify + delete (needed for cross-filesystem moves)
if err := copyFile(src, dst); err != nil {
return fmt.Errorf("copy failed: %w", err)
}

// Verify file sizes match
srcInfo, err := os.Stat(src)
if err != nil {
os.Remove(dst) // Clean up partial copy
return fmt.Errorf("failed to stat source: %w", err)
}

dstInfo, err := os.Stat(dst)
if err != nil {
os.Remove(dst)
return fmt.Errorf("failed to stat destination: %w", err)
}

if srcInfo.Size() != dstInfo.Size() {
os.Remove(dst)
return fmt.Errorf("file size mismatch after copy: src=%d dst=%d", srcInfo.Size(), dstInfo.Size())
}

// Remove source only after successful copy and verification
return os.Remove(src)
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
srcFile, err := os.Open(src)
if err != nil {
return fmt.Errorf("failed to open source: %w", err)
}
defer srcFile.Close()

dstFile, err := os.Create(dst)
if err != nil {
return fmt.Errorf("failed to create destination: %w", err)
}
defer dstFile.Close()

_, err = io.Copy(dstFile, srcFile)
if err != nil {
return fmt.Errorf("failed to copy data: %w", err)
}

// Sync to ensure data is written to disk
if err := dstFile.Sync(); err != nil {
return fmt.Errorf("failed to sync: %w", err)
}

return nil
}
