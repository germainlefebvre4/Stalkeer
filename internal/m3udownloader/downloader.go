package m3udownloader

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/glefebvre/stalkeer/internal/circuitbreaker"
	"github.com/glefebvre/stalkeer/internal/config"
	"github.com/glefebvre/stalkeer/internal/logger"
	"github.com/glefebvre/stalkeer/internal/retry"
)

var (
	// ErrInvalidM3U is returned when downloaded content is not valid M3U
	ErrInvalidM3U = fmt.Errorf("invalid M3U file format")

	// ErrFileSizeExceeded is returned when file size exceeds limit
	ErrFileSizeExceeded = fmt.Errorf("file size exceeds maximum limit")

	// ErrInvalidContentType is returned when content type is not M3U
	ErrInvalidContentType = fmt.Errorf("invalid content type")
)

// Downloader handles M3U playlist downloads
type Downloader struct {
	cfg            *config.M3UDownloadConfig
	logger         *logger.Logger
	httpClient     *http.Client
	retryConfig    retry.Config
	circuitBreaker *circuitbreaker.CircuitBreaker
	archiveManager *ArchiveManager
}

// NewDownloader creates a new M3U downloader
func NewDownloader(cfg *config.M3UDownloadConfig, log *logger.Logger) *Downloader {
	// Create HTTP client with timeout
	httpClient := &http.Client{
		Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Limit redirects to 10
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	// Configure retry logic
	retryConfig := retry.Config{
		MaxAttempts:       cfg.RetryAttempts,
		InitialBackoff:    1 * time.Second,
		MaxBackoff:        30 * time.Second,
		BackoffMultiplier: 2.0,
		JitterFraction:    0.1,
	}

	// Configure circuit breaker
	cbConfig := circuitbreaker.Config{
		MaxFailures:         5,
		Timeout:             60 * time.Second,
		MaxHalfOpenRequests: 1,
		IsSuccessful: func(err error) bool {
			return err == nil
		},
	}

	return &Downloader{
		cfg:            cfg,
		logger:         log,
		httpClient:     httpClient,
		retryConfig:    retryConfig,
		circuitBreaker: circuitbreaker.New(cbConfig),
		archiveManager: NewArchiveManager(cfg.ArchiveDir, log),
	}
}

// Download downloads M3U playlist from URL and saves to destPath atomically
func (d *Downloader) Download(ctx context.Context, url, destPath string) error {
	d.logger.WithFields(map[string]interface{}{
		"url":      url,
		"destPath": destPath,
	}).Info("Starting M3U download")

	// Execute download with circuit breaker
	err := d.circuitBreaker.Execute(func() error {
		return d.downloadWithRetry(ctx, url, destPath)
	})

	if err != nil {
		d.logger.WithFields(map[string]interface{}{
			"url":   url,
			"error": err,
		}).Error("M3U download failed", err)
		return err
	}

	d.logger.WithFields(map[string]interface{}{
		"url":      url,
		"destPath": destPath,
	}).Info("M3U download completed successfully")

	return nil
}

// downloadWithRetry downloads with retry logic
func (d *Downloader) downloadWithRetry(ctx context.Context, url, destPath string) error {
	return retry.Do(ctx, d.retryConfig, func() error {
		return d.downloadOnce(ctx, url, destPath)
	}, d.isRetryableError)
}

// downloadOnce performs a single download attempt
func (d *Downloader) downloadOnce(ctx context.Context, url, destPath string) error {
	// Create temporary file for atomic write
	tempFile, err := os.CreateTemp(filepath.Dir(destPath), ".m3u_download_*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	defer func() {
		tempFile.Close()
		os.Remove(tempPath) // Clean up temp file if operation fails
	}()

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication if configured
	if d.cfg.AuthUsername != "" && d.cfg.AuthPassword != "" {
		req.SetBasicAuth(d.cfg.AuthUsername, d.cfg.AuthPassword)
	}

	// Perform request
	resp, err := d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
	}

	// Validate content type (if provided)
	contentType := resp.Header.Get("Content-Type")
	if contentType != "" && !d.isValidContentType(contentType) {
		d.logger.WithFields(map[string]interface{}{
			"content_type": contentType,
		}).Warn("Unexpected content type, proceeding anyway")
	}

	// Check content length if provided
	if resp.ContentLength > 0 {
		maxSize := d.cfg.MaxFileSizeMB * 1024 * 1024
		if resp.ContentLength > maxSize {
			return fmt.Errorf("%w: %d bytes exceeds %d MB limit", ErrFileSizeExceeded, resp.ContentLength, d.cfg.MaxFileSizeMB)
		}
	}

	// Read response body with size limit
	var buf bytes.Buffer
	maxSize := d.cfg.MaxFileSizeMB * 1024 * 1024
	limitedReader := io.LimitReader(resp.Body, maxSize+1) // +1 to detect overflow

	written, err := io.Copy(&buf, limitedReader)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Check if size limit exceeded
	if written > maxSize {
		return fmt.Errorf("%w: download exceeds %d MB limit", ErrFileSizeExceeded, d.cfg.MaxFileSizeMB)
	}

	// Log progress for large files
	if written > 10*1024*1024 { // > 10 MB
		d.logger.WithFields(map[string]interface{}{
			"size_mb": float64(written) / (1024 * 1024),
		}).Info("Download progress")
	}

	// Validate M3U content
	data := buf.Bytes()
	if err := d.validateM3UContent(data); err != nil {
		return err
	}

	// Write to temp file
	if _, err := tempFile.Write(data); err != nil {
		return fmt.Errorf("failed to write to temp file: %w", err)
	}

	// Sync to ensure data is written to disk
	if err := tempFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	// Close temp file before rename
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Ensure destination directory exists
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Atomic rename to destination
	if err := os.Rename(tempPath, destPath); err != nil {
		return fmt.Errorf("failed to rename temp file to destination: %w", err)
	}

	d.logger.WithFields(map[string]interface{}{
		"size_bytes": written,
		"size_mb":    float64(written) / (1024 * 1024),
	}).Info("M3U file saved successfully")

	return nil
}

// validateM3UContent checks if the content is a valid M3U file
func (d *Downloader) validateM3UContent(data []byte) error {
	// Check minimum size
	if len(data) == 0 {
		return fmt.Errorf("%w: empty file", ErrInvalidM3U)
	}

	// Check for M3U header
	content := string(data)
	lines := strings.Split(content, "\n")

	// First non-empty line should be #EXTM3U
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "#EXTM3U") {
			return fmt.Errorf("%w: missing #EXTM3U header", ErrInvalidM3U)
		}
		break
	}

	return nil
}

// isValidContentType checks if the content type is valid for M3U
func (d *Downloader) isValidContentType(contentType string) bool {
	validTypes := []string{
		"application/vnd.apple.mpegurl",
		"application/x-mpegurl",
		"audio/x-mpegurl",
		"audio/mpegurl",
		"text/plain",
		"application/octet-stream",
	}

	// Extract main type (before semicolon)
	ct := strings.Split(contentType, ";")[0]
	ct = strings.TrimSpace(strings.ToLower(ct))

	for _, valid := range validTypes {
		if ct == valid {
			return true
		}
	}

	return false
}

// isRetryableError determines if an error should trigger a retry
func (d *Downloader) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Don't retry validation errors
	if err == ErrInvalidM3U || err == ErrFileSizeExceeded || err == ErrInvalidContentType {
		return false
	}

	// Don't retry 4xx errors (client errors)
	errStr := err.Error()
	if strings.Contains(errStr, "HTTP error: 4") {
		return false
	}

	// Retry network errors, timeouts, and 5xx errors
	return true
}

// DownloadAndArchive downloads M3U file and creates an archive copy
func (d *Downloader) DownloadAndArchive(ctx context.Context, url, destPath string) error {
	// Download to destPath
	if err := d.Download(ctx, url, destPath); err != nil {
		return err
	}

	// Create archive copy
	archivePath, err := d.archiveManager.ArchiveFile(destPath)
	if err != nil {
		d.logger.WithFields(map[string]interface{}{
			"error": err,
		}).Warn("Failed to create archive copy, continuing anyway")
		// Don't fail the entire operation if archiving fails
	} else {
		d.logger.WithFields(map[string]interface{}{
			"archive_path": archivePath,
		}).Info("Archive copy created")
	}

	// Rotate archives
	if err := d.archiveManager.RotateArchive(d.cfg.RetentionCount); err != nil {
		d.logger.WithFields(map[string]interface{}{
			"error": err,
		}).Warn("Failed to rotate archives")
		// Don't fail the entire operation if rotation fails
	}

	return nil
}

// GetArchiveManager returns the archive manager
func (d *Downloader) GetArchiveManager() *ArchiveManager {
	return d.archiveManager
}
