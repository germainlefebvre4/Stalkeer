package downloader

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/glefebvre/stalkeer/internal/database"
	"github.com/glefebvre/stalkeer/internal/errors"
	"github.com/glefebvre/stalkeer/internal/logger"
	"github.com/glefebvre/stalkeer/internal/models"
	"gorm.io/gorm"
)

// StateManager handles download state transitions and locking
type StateManager struct {
	db               *gorm.DB
	lockTimeout      time.Duration
	instanceID       string
	progressInterval struct {
		bytes   int64
		seconds time.Duration
	}
}

// StateManagerConfig holds configuration for state manager
type StateManagerConfig struct {
	LockTimeoutMinutes      int
	ProgressIntervalMB      int64
	ProgressIntervalSeconds int
}

// DefaultStateManagerConfig returns default configuration
func DefaultStateManagerConfig() StateManagerConfig {
	return StateManagerConfig{
		LockTimeoutMinutes:      5,
		ProgressIntervalMB:      10,
		ProgressIntervalSeconds: 30,
	}
}

// NewStateManager creates a new download state manager
func NewStateManager(config StateManagerConfig) *StateManager {
	if config.LockTimeoutMinutes == 0 {
		config = DefaultStateManagerConfig()
	}

	hostname, _ := os.Hostname()
	instanceID := fmt.Sprintf("%s-%d", hostname, os.Getpid())

	return &StateManager{
		db:          database.GetDB(),
		lockTimeout: time.Duration(config.LockTimeoutMinutes) * time.Minute,
		instanceID:  instanceID,
		progressInterval: struct {
			bytes   int64
			seconds time.Duration
		}{
			bytes:   config.ProgressIntervalMB * 1024 * 1024,
			seconds: time.Duration(config.ProgressIntervalSeconds) * time.Second,
		},
	}
}

// AcquireLock attempts to acquire a lock on a download record
func (sm *StateManager) AcquireLock(ctx context.Context, downloadID uint) error {
	log := logger.AppLogger()

	// First, clean up any stale locks
	if err := sm.CleanupStaleLocks(ctx); err != nil {
		log.WithFields(map[string]interface{}{
			"error": err,
		}).Warn("failed to cleanup stale locks (continuing anyway)")
	}

	// Attempt to acquire lock using optimistic locking
	now := time.Now()
	result := sm.db.WithContext(ctx).
		Model(&models.DownloadInfo{}).
		Where("id = ? AND (locked_at IS NULL OR locked_at < ?)", downloadID, now.Add(-sm.lockTimeout)).
		Updates(map[string]interface{}{
			"locked_at": now,
			"locked_by": sm.instanceID,
		})

	if result.Error != nil {
		return errors.Wrap(result.Error, errors.CodeInternal, "failed to acquire download lock")
	}

	if result.RowsAffected == 0 {
		return errors.ValidationError("download is locked by another process")
	}

	log.WithFields(map[string]interface{}{
		"download_id": downloadID,
		"instance_id": sm.instanceID,
	}).Debug("acquired download lock")

	return nil
}

// ReleaseLock releases a lock on a download record
func (sm *StateManager) ReleaseLock(ctx context.Context, downloadID uint) error {
	log := logger.AppLogger()

	result := sm.db.WithContext(ctx).
		Model(&models.DownloadInfo{}).
		Where("id = ? AND locked_by = ?", downloadID, sm.instanceID).
		Updates(map[string]interface{}{
			"locked_at": nil,
			"locked_by": nil,
		})

	if result.Error != nil {
		return errors.Wrap(result.Error, errors.CodeInternal, "failed to release download lock")
	}

	log.WithFields(map[string]interface{}{
		"download_id": downloadID,
		"instance_id": sm.instanceID,
	}).Debug("released download lock")

	return nil
}

// CleanupStaleLocks removes locks older than the timeout
func (sm *StateManager) CleanupStaleLocks(ctx context.Context) error {
	log := logger.AppLogger()

	cutoffTime := time.Now().Add(-sm.lockTimeout)
	result := sm.db.WithContext(ctx).
		Model(&models.DownloadInfo{}).
		Where("locked_at < ?", cutoffTime).
		Updates(map[string]interface{}{
			"locked_at": nil,
			"locked_by": nil,
		})

	if result.Error != nil {
		return errors.Wrap(result.Error, errors.CodeInternal, "failed to cleanup stale locks")
	}

	if result.RowsAffected > 0 {
		log.WithFields(map[string]interface{}{
			"count":       result.RowsAffected,
			"cutoff_time": cutoffTime,
		}).Info("cleaned up stale download locks")
	}

	return nil
}

// UpdateState transitions download to a new state
func (sm *StateManager) UpdateState(ctx context.Context, downloadID uint, newStatus models.DownloadStatus, errorMsg *string) error {
	log := logger.AppLogger()

	updates := map[string]interface{}{
		"status": string(newStatus),
	}

	// Set timestamps based on state
	now := time.Now()
	switch newStatus {
	case models.DownloadStatusDownloading:
		updates["started_at"] = now
	case models.DownloadStatusCompleted:
		updates["completed_at"] = now
		// Release lock on completion
		updates["locked_at"] = nil
		updates["locked_by"] = nil
	case models.DownloadStatusFailed:
		updates["completed_at"] = now
		// Release lock on failure
		updates["locked_at"] = nil
		updates["locked_by"] = nil
		if errorMsg != nil {
			updates["error_message"] = *errorMsg
		}
	case models.DownloadStatusRetrying:
		updates["retry_count"] = gorm.Expr("retry_count + 1")
		updates["last_retry_at"] = now
	}

	result := sm.db.WithContext(ctx).
		Model(&models.DownloadInfo{}).
		Where("id = ?", downloadID).
		Updates(updates)

	if result.Error != nil {
		return errors.Wrap(result.Error, errors.CodeInternal, "failed to update download state")
	}

	log.WithFields(map[string]interface{}{
		"download_id": downloadID,
		"new_status":  string(newStatus),
	}).Debug("updated download state")

	return nil
}

// UpdateProgress updates download progress (bytes downloaded)
func (sm *StateManager) UpdateProgress(ctx context.Context, downloadID uint, bytesDownloaded, totalBytes int64) error {
	updates := map[string]interface{}{
		"bytes_downloaded": bytesDownloaded,
		"total_bytes":      totalBytes,
	}

	result := sm.db.WithContext(ctx).
		Model(&models.DownloadInfo{}).
		Where("id = ?", downloadID).
		Updates(updates)

	if result.Error != nil {
		// Log error but don't fail the download
		logger.AppLogger().WithFields(map[string]interface{}{
			"download_id":      downloadID,
			"bytes_downloaded": bytesDownloaded,
			"error":            result.Error,
		}).Warn("failed to update download progress")
		return nil
	}

	return nil
}

// ShouldPersistProgress determines if progress should be persisted based on interval
func (sm *StateManager) ShouldPersistProgress(bytesSinceLastPersist int64, timeSinceLastPersist time.Duration) bool {
	return bytesSinceLastPersist >= sm.progressInterval.bytes ||
		timeSinceLastPersist >= sm.progressInterval.seconds
}

// GetIncompleteDownloads retrieves downloads eligible for resume
func (sm *StateManager) GetIncompleteDownloads(ctx context.Context, maxRetries int, limit int) ([]models.DownloadInfo, error) {
	log := logger.AppLogger()

	var downloads []models.DownloadInfo

	// Build query for incomplete downloads
	query := sm.db.WithContext(ctx).
		Where("status IN ?", []string{
			string(models.DownloadStatusPending),
			string(models.DownloadStatusDownloading),
			string(models.DownloadStatusPaused),
			string(models.DownloadStatusFailed),
			string(models.DownloadStatusRetrying),
		})

	// Exclude downloads exceeding max retries
	if maxRetries > 0 {
		query = query.Where("retry_count < ?", maxRetries)
	}

	// Exclude locked downloads (unless stale)
	cutoffTime := time.Now().Add(-sm.lockTimeout)
	query = query.Where("locked_at IS NULL OR locked_at < ?", cutoffTime)

	// Order by priority: failed recently, then oldest first
	query = query.Order("CASE WHEN status = 'failed' THEN 0 ELSE 1 END").
		Order("updated_at ASC")

	// Apply limit if specified
	if limit > 0 {
		query = query.Limit(limit)
	}

	result := query.Find(&downloads)
	if result.Error != nil {
		return nil, errors.Wrap(result.Error, errors.CodeInternal, "failed to query incomplete downloads")
	}

	log.WithFields(map[string]interface{}{
		"count": len(downloads),
	}).Debug("found incomplete downloads")

	return downloads, nil
}

// GetDownloadByID retrieves a download record by ID
func (sm *StateManager) GetDownloadByID(ctx context.Context, downloadID uint) (*models.DownloadInfo, error) {
	var download models.DownloadInfo
	result := sm.db.WithContext(ctx).First(&download, downloadID)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.NotFoundError("download", fmt.Sprintf("ID %d", downloadID))
		}
		return nil, errors.Wrap(result.Error, errors.CodeInternal, "failed to get download record")
	}
	return &download, nil
}

// CreateDownloadRecord creates a new download record
func (sm *StateManager) CreateDownloadRecord(ctx context.Context, status models.DownloadStatus) (*models.DownloadInfo, error) {
	download := &models.DownloadInfo{
		Status: string(status),
	}

	result := sm.db.WithContext(ctx).Create(download)
	if result.Error != nil {
		return nil, errors.Wrap(result.Error, errors.CodeInternal, "failed to create download record")
	}

	return download, nil
}

// GetDownloadsByProcessedLineID retrieves download records by ProcessedLine ID
func (sm *StateManager) GetDownloadsByProcessedLineID(ctx context.Context, processedLineID uint) ([]models.DownloadInfo, error) {
	var downloads []models.DownloadInfo

	// Query through ProcessedLine association
	result := sm.db.WithContext(ctx).
		Joins("JOIN processed_lines ON processed_lines.download_info_id = download_info.id").
		Where("processed_lines.id = ?", processedLineID).
		Find(&downloads)

	if result.Error != nil {
		return nil, errors.Wrap(result.Error, errors.CodeInternal, "failed to query downloads by processed line")
	}

	return downloads, nil
}
