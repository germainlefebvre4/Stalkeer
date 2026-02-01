package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/glefebvre/stalkeer/internal/database"
	"github.com/glefebvre/stalkeer/internal/errors"
	"github.com/glefebvre/stalkeer/internal/models"
	"github.com/glefebvre/stalkeer/internal/retry"
)

// DownloadOptions holds configuration for a download operation
type DownloadOptions struct {
	URL             string
	DestinationPath string
	ProcessedLineID uint
	OnProgress      func(downloaded, total int64)
	Timeout         time.Duration
	RetryAttempts   int
}

// DownloadResult contains information about a completed download
type DownloadResult struct {
	FilePath  string
	FileSize  int64
	Duration  time.Duration
	BytesRead int64
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
	if opts.DestinationPath == "" {
		return nil, errors.ValidationError("destination path cannot be empty")
	}

	// Update state to downloading if ProcessedLineID is provided
	if opts.ProcessedLineID > 0 {
		if err := d.updateDownloadState(opts.ProcessedLineID, models.StateDownloading, nil); err != nil {
			return nil, err
		}
	}

	// Create destination directory
	dir := filepath.Dir(opts.DestinationPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, errors.Wrap(err, errors.CodeInternal, "failed to create destination directory")
	}

	// Create temporary file for atomic write
	tempPath := opts.DestinationPath + ".tmp"
	defer os.Remove(tempPath) // Clean up on error

	// Perform download with retry
	var result *DownloadResult
	err := retry.Do(ctx, d.retryConfig, func() error {
		res, err := d.downloadFile(ctx, opts.URL, tempPath, opts.OnProgress)
		if err != nil {
			return err
		}
		result = res
		return nil
	}, errors.IsRetryable)

	if err != nil {
		if opts.ProcessedLineID > 0 {
			errMsg := err.Error()
			d.updateDownloadState(opts.ProcessedLineID, models.StateFailed, &errMsg)
		}
		return nil, errors.ExternalServiceError("download", "failed to download file", err)
	}

	// Atomic rename from temp to final path
	if err := os.Rename(tempPath, opts.DestinationPath); err != nil {
		return nil, errors.Wrap(err, errors.CodeInternal, "failed to rename downloaded file")
	}

	result.FilePath = opts.DestinationPath
	result.Duration = time.Since(startTime)

	// Update state to downloaded
	if opts.ProcessedLineID > 0 {
		if err := d.updateDownloadState(opts.ProcessedLineID, models.StateDownloaded, nil); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// downloadFile performs the actual HTTP download
func (d *Downloader) downloadFile(ctx context.Context, url, destPath string, onProgress func(int64, int64)) (*DownloadResult, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
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
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	return &DownloadResult{
		FileSize:  bytesRead,
		BytesRead: bytesRead,
	}, nil
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
