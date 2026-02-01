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
	"github.com/glefebvre/stalkeer/internal/logger"
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
	FilePath     string
	TempPath     string // Temp path used (for debugging)
	FileSize     int64
	Extension    string
	Duration     time.Duration
	BytesRead    int64
	MoveDuration time.Duration
}

// Downloader handles media file downloads
type Downloader struct {
	httpClient    *http.Client
	retryConfig   retry.Config
	stateManager  *StateManager
	resumeSupport *ResumeSupport
}

// New creates a new Downloader instance
func New(timeout time.Duration, retryAttempts int) *Downloader {
	if timeout == 0 {
		timeout = 600 * time.Second // 10 minutes default
	}

	if retryAttempts == 0 {
		retryAttempts = 3
	}

	stateManager := NewStateManager(DefaultStateManagerConfig())
	resumeSupport := NewResumeSupport(stateManager)

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
		stateManager:  stateManager,
		resumeSupport: resumeSupport,
	}
}

// GetStateManager returns the state manager instance
func (d *Downloader) GetStateManager() *StateManager {
	return d.stateManager
}

// Download downloads a file from the given URL to the destination path
func (d *Downloader) Download(ctx context.Context, opts DownloadOptions) (*DownloadResult, error) {
	startTime := time.Now()
	log := logger.AppLogger()

	// Validate inputs
	if opts.URL == "" {
		return nil, errors.ValidationError("download URL cannot be empty")
	}
	if opts.BaseDestPath == "" {
		return nil, errors.ValidationError("base destination path cannot be empty")
	}

	// Create or get DownloadInfo record and acquire lock
	var downloadInfoID uint
	if opts.ProcessedLineID > 0 {
		// Create or get DownloadInfo record
		dlInfo, err := d.getOrCreateDownloadInfo(ctx, opts.ProcessedLineID, opts.URL)
		if err != nil {
			return nil, err
		}
		downloadInfoID = dlInfo.ID

		// Acquire lock to prevent concurrent downloads
		if err := d.stateManager.AcquireLock(ctx, downloadInfoID); err != nil {
			log.WithFields(map[string]interface{}{
				"download_id": downloadInfoID,
				"error":       err,
			}).Warn("failed to acquire download lock, skipping")
			return nil, errors.ValidationError("download is locked by another process")
		}
		defer func() {
			// Always release lock on exit (success or failure)
			if err := d.stateManager.ReleaseLock(ctx, downloadInfoID); err != nil {
				log.WithFields(map[string]interface{}{
					"download_id": downloadInfoID,
					"error":       err,
				}).Error("failed to release download lock", err)
			}
		}()

		// Update state to downloading
		if err := d.stateManager.UpdateState(ctx, downloadInfoID, models.DownloadStatusDownloading, nil); err != nil {
			return nil, err
		}

		// Also update ProcessedLine state for backward compatibility
		if err := d.updateProcessedLineState(opts.ProcessedLineID, models.StateDownloading); err != nil {
			log.WithFields(map[string]interface{}{
				"error": err,
			}).Warn("failed to update processed line state")
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
	var lastPersistedBytes int64
	var lastPersistTime time.Time = time.Now()

	err := retry.Do(ctx, d.retryConfig, func() error {
		res, ct, err := d.downloadFile(ctx, opts.URL, tempPath, func(downloaded, total int64) {
			// Call user's progress callback
			if opts.OnProgress != nil {
				opts.OnProgress(downloaded, total)
			}

			// Persist progress at intervals if we have a download info record
			if downloadInfoID > 0 {
				bytesSinceLastPersist := downloaded - lastPersistedBytes
				timeSinceLastPersist := time.Since(lastPersistTime)

				if d.stateManager.ShouldPersistProgress(bytesSinceLastPersist, timeSinceLastPersist) {
					if err := d.stateManager.UpdateProgress(ctx, downloadInfoID, downloaded, total); err != nil {
						log.WithFields(map[string]interface{}{
							"download_id": downloadInfoID,
							"error":       err,
						}).Warn("failed to persist download progress")
					}
					lastPersistedBytes = downloaded
					lastPersistTime = time.Now()
				}
			}
		})
		if err != nil {
			return err
		}
		result = res
		contentType = ct
		return nil
	}, errors.IsRetryable)

	if err != nil {
		// Update download info on failure
		if downloadInfoID > 0 {
			errMsg := err.Error()
			if updateErr := d.stateManager.UpdateState(ctx, downloadInfoID, models.DownloadStatusFailed, &errMsg); updateErr != nil {
				log.WithFields(map[string]interface{}{
					"error": updateErr,
				}).Error("failed to update download state to failed", updateErr)
			}

			// Update ProcessedLine state for backward compatibility
			if updateErr := d.updateProcessedLineState(opts.ProcessedLineID, models.StateFailed); updateErr != nil {
				log.WithFields(map[string]interface{}{
					"error": updateErr,
				}).Warn("failed to update processed line state to failed")
			}
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
	if downloadInfoID > 0 {
		if err := d.stateManager.UpdateState(ctx, downloadInfoID, models.DownloadStatusDownloading, nil); err != nil {
			log.WithFields(map[string]interface{}{
				"error": err,
			}).Warn("failed to update state to organizing")
		}

		// Update ProcessedLine state for backward compatibility
		if err := d.updateProcessedLineState(opts.ProcessedLineID, models.StateOrganizing); err != nil {
			log.WithFields(map[string]interface{}{
				"error": err,
			}).Warn("failed to update processed line state to organizing")
		}
	}

	// Move file to final destination
	moveStart := time.Now()
	if err := moveFile(tempPath, finalDestPath); err != nil {
		if downloadInfoID > 0 {
			errMsg := err.Error()
			if updateErr := d.stateManager.UpdateState(ctx, downloadInfoID, models.DownloadStatusFailed, &errMsg); updateErr != nil {
				log.WithFields(map[string]interface{}{
					"error": updateErr,
				}).Error("failed to update download state to failed", updateErr)
			}

			// Update ProcessedLine state for backward compatibility
			if updateErr := d.updateProcessedLineState(opts.ProcessedLineID, models.StateFailed); updateErr != nil {
				log.WithFields(map[string]interface{}{
					"error": updateErr,
				}).Warn("failed to update processed line state to failed")
			}
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

	// Update state to completed
	if downloadInfoID > 0 {
		// Update download info with final details
		if err := d.updateDownloadInfoCompleted(ctx, downloadInfoID, finalDestPath, result.FileSize); err != nil {
			log.WithFields(map[string]interface{}{
				"error": err,
			}).Error("failed to update download info to completed", err)
		}

		// Update ProcessedLine state for backward compatibility
		if err := d.updateProcessedLineState(opts.ProcessedLineID, models.StateDownloaded); err != nil {
			log.WithFields(map[string]interface{}{
				"error": err,
			}).Warn("failed to update processed line state to downloaded")
		}
	}

	return result, nil
}

// downloadFile performs the actual HTTP download
func (d *Downloader) downloadFile(ctx context.Context, url, destPath string, onProgress func(int64, int64)) (*DownloadResult, string, error) {
	return d.downloadFileWithResume(ctx, url, destPath, 0, onProgress)
}

// downloadFileWithResume performs HTTP download with optional resume support
func (d *Downloader) downloadFileWithResume(ctx context.Context, url, destPath string, startByte int64, onProgress func(int64, int64)) (*DownloadResult, string, error) {
	var req *http.Request
	var err error

	// Create request with optional Range header
	if startByte > 0 {
		req, err = d.resumeSupport.BuildResumeRequest(ctx, url, startByte)
		if err != nil {
			return nil, "", err
		}
		logger.AppLogger().WithFields(map[string]interface{}{
			"url":        url,
			"start_byte": startByte,
		}).Debug("attempting to resume download")
	} else {
		req, err = http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, "", fmt.Errorf("failed to create request: %w", err)
		}
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	// Handle resume response if we requested a range
	if startByte > 0 {
		if err := d.resumeSupport.HandleResumeResponse(resp, startByte); err != nil {
			// If resume not supported, we'll restart from beginning
			if errors.IsValidationError(err) {
				logger.AppLogger().Warn("resume not supported, restarting download from beginning")
				return d.downloadFileWithResume(ctx, url, destPath, 0, onProgress)
			}
			return nil, "", err
		}
	} else {
		// Normal download - check status
		if resp.StatusCode != http.StatusOK {
			return nil, "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}
	}

	// Get content type for extension detection
	contentType := resp.Header.Get("Content-Type")

	// Open file for writing (append mode if resuming)
	var out *os.File
	if startByte > 0 {
		out, err = os.OpenFile(destPath, os.O_WRONLY|os.O_APPEND, 0644)
	} else {
		out, err = os.Create(destPath)
	}
	if err != nil {
		return nil, "", fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	// Download with progress tracking
	var bytesRead int64
	contentLength := resp.ContentLength
	if startByte > 0 {
		// For resumed downloads, ContentLength is remaining bytes
		contentLength += startByte
	}

	if onProgress != nil && contentLength > 0 {
		// Use TeeReader to track progress
		reader := &progressReader{
			reader:     resp.Body,
			total:      contentLength,
			downloaded: startByte, // Start from existing progress
			onProgress: onProgress,
		}
		bytesRead, err = io.Copy(out, reader)
	} else {
		bytesRead, err = io.Copy(out, resp.Body)
	}

	if err != nil {
		return nil, "", fmt.Errorf("failed to write file: %w", err)
	}

	totalBytes := startByte + bytesRead

	return &DownloadResult{
		FileSize:  totalBytes,
		BytesRead: totalBytes,
	}, contentType, nil
}

// getOrCreateDownloadInfo gets or creates a DownloadInfo record for a ProcessedLine
func (d *Downloader) getOrCreateDownloadInfo(ctx context.Context, processedLineID uint, url string) (*models.DownloadInfo, error) {
	db := database.Get()
	if db == nil {
		return nil, errors.New(errors.CodeInternal, "database not initialized")
	}

	// Get ProcessedLine
	var processedLine models.ProcessedLine
	if err := db.First(&processedLine, processedLineID).Error; err != nil {
		return nil, errors.DatabaseError("failed to fetch processed line", err)
	}

	// Check if DownloadInfo already exists
	if processedLine.DownloadInfoID != nil {
		var downloadInfo models.DownloadInfo
		if err := db.First(&downloadInfo, *processedLine.DownloadInfoID).Error; err != nil {
			return nil, errors.DatabaseError("failed to fetch download info", err)
		}
		return &downloadInfo, nil
	}

	// Create new DownloadInfo
	downloadInfo := &models.DownloadInfo{
		Status:    string(models.DownloadStatusPending),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := db.Create(downloadInfo).Error; err != nil {
		return nil, errors.DatabaseError("failed to create download info", err)
	}

	// Link to ProcessedLine
	downloadInfoID := downloadInfo.ID
	if err := db.Model(&models.ProcessedLine{}).
		Where("id = ?", processedLineID).
		Update("download_info_id", &downloadInfoID).Error; err != nil {
		return nil, errors.DatabaseError("failed to link download info to processed line", err)
	}

	return downloadInfo, nil
}

// updateDownloadInfoCompleted updates DownloadInfo to completed status with final details
func (d *Downloader) updateDownloadInfoCompleted(ctx context.Context, downloadInfoID uint, filePath string, fileSize int64) error {
	db := database.Get()
	if db == nil {
		return errors.New(errors.CodeInternal, "database not initialized")
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":        string(models.DownloadStatusCompleted),
		"download_path": filePath,
		"file_size":     fileSize,
		"completed_at":  now,
		"updated_at":    now,
		"locked_at":     nil, // Release lock
		"locked_by":     nil,
	}

	// Update DownloadInfo with all completion details
	if err := db.Model(&models.DownloadInfo{}).
		Where("id = ?", downloadInfoID).
		Updates(updates).Error; err != nil {
		return errors.DatabaseError("failed to update download info to completed", err)
	}

	return nil
}

// updateProcessedLineState updates the ProcessedLine state (for backward compatibility)
func (d *Downloader) updateProcessedLineState(processedLineID uint, state models.ProcessingState) error {
	db := database.Get()
	if db == nil {
		return errors.New(errors.CodeInternal, "database not initialized")
	}

	updates := map[string]interface{}{
		"state":      state,
		"updated_at": time.Now(),
	}

	if err := db.Model(&models.ProcessedLine{}).
		Where("id = ?", processedLineID).
		Updates(updates).Error; err != nil {
		return errors.DatabaseError("failed to update processed line state", err)
	}

	return nil
}

// updateDownloadState updates the download state in the database (DEPRECATED - kept for compatibility)
// Use StateManager methods instead
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
		"video/x-matroska":      ".mkv",
		"video/mp4":             ".mp4",
		"video/x-msvideo":       ".avi",
		"video/quicktime":       ".mov",
		"video/x-flv":           ".flv",
		"video/webm":            ".webm",
		"video/mpeg":            ".mpg",
		"video/3gpp":            ".3gp",
		"video/x-ms-wmv":        ".wmv",
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
