package models

import "time"

// TVShow represents TV show metadata from TMDB with season/episode information
type TVShow struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	TMDBID     int       `gorm:"not null;index:idx_tvshows_tmdb" json:"tmdb_id"`
	TVDBID     *int      `gorm:"index:idx_tvshows_tvdb" json:"tvdb_id,omitempty"`
	TMDBTitle  string    `gorm:"type:varchar(255);not null" json:"tmdb_title"`
	TMDBYear   int       `gorm:"not null" json:"tmdb_year"`
	TMDBGenres *string   `gorm:"type:text" json:"tmdb_genres,omitempty"`
	Season     *int      `gorm:"index:idx_tvshows_season_episode" json:"season,omitempty"`
	Episode    *int      `gorm:"index:idx_tvshows_season_episode" json:"episode,omitempty"`
	CreatedAt  time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt  time.Time `gorm:"not null" json:"updated_at"`

	// Associations
	ProcessedLines []ProcessedLine `gorm:"foreignKey:TVShowID" json:"processed_lines,omitempty"`
}

// TableName specifies the table name for TVShow
func (TVShow) TableName() string {
	return "tvshows"
}

// ProcessingLog represents a log entry for processing actions
type ProcessingLog struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	Action       string     `gorm:"type:varchar(100);not null" json:"action"`
	ItemCount    int        `gorm:"not null;default:0" json:"item_count"`
	Status       string     `gorm:"type:varchar(50);not null" json:"status"` // "success", "failed", "in_progress"
	StartedAt    time.Time  `gorm:"not null" json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	ErrorMessage *string    `gorm:"type:text" json:"error_message,omitempty"`
	CreatedAt    time.Time  `gorm:"not null" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"not null" json:"updated_at"`
}

// TableName specifies the table name for ProcessingLog
func (ProcessingLog) TableName() string {
	return "processing_logs"
}

// DownloadStatus represents possible download states
type DownloadStatus string

const (
	// DownloadStatusPending indicates download is queued but not started
	DownloadStatusPending DownloadStatus = "pending"
	// DownloadStatusDownloading indicates download is in progress
	DownloadStatusDownloading DownloadStatus = "downloading"
	// DownloadStatusPaused indicates download was paused (can be resumed)
	DownloadStatusPaused DownloadStatus = "paused"
	// DownloadStatusCompleted indicates download finished successfully
	DownloadStatusCompleted DownloadStatus = "completed"
	// DownloadStatusFailed indicates download failed
	DownloadStatusFailed DownloadStatus = "failed"
	// DownloadStatusRetrying indicates download is being retried after failure
	DownloadStatusRetrying DownloadStatus = "retrying"
)

// DownloadInfo represents download tracking information
type DownloadInfo struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	Status          string     `gorm:"type:varchar(50);not null;index:idx_download_info_status" json:"status"` // "pending", "downloading", "paused", "completed", "failed", "retrying"
	DownloadPath    *string    `gorm:"type:text" json:"download_path,omitempty"`
	FileSize        *int64     `json:"file_size,omitempty"`
	BytesDownloaded *int64     `gorm:"default:0" json:"bytes_downloaded,omitempty"`                  // Track partial download progress
	TotalBytes      *int64     `json:"total_bytes,omitempty"`                                        // Expected total file size
	ResumeToken     *string    `gorm:"type:varchar(255)" json:"resume_token,omitempty"`              // Server-specific resume identifier (ETag, etc.)
	RetryCount      int        `gorm:"default:0;not null" json:"retry_count"`                        // Number of retry attempts
	LastRetryAt     *time.Time `json:"last_retry_at,omitempty"`                                      // Timestamp of last retry attempt
	LockedAt        *time.Time `gorm:"index:idx_download_info_locked_at" json:"locked_at,omitempty"` // Lock timestamp to prevent concurrent downloads
	LockedBy        *string    `gorm:"type:varchar(100)" json:"locked_by,omitempty"`                 // Instance/process that acquired lock
	StartedAt       *time.Time `json:"started_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	ErrorMessage    *string    `gorm:"type:text" json:"error_message,omitempty"`
	CreatedAt       time.Time  `gorm:"not null" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"not null;index:idx_download_info_updated_at" json:"updated_at"`

	// Associations
	ProcessedLines []ProcessedLine `gorm:"foreignKey:DownloadInfoID" json:"processed_lines,omitempty"`
}

// TableName specifies the table name for DownloadInfo
func (DownloadInfo) TableName() string {
	return "download_info"
}

// IsEligibleForResume returns true if the download can be resumed
func (d *DownloadInfo) IsEligibleForResume(maxRetries int, lockTimeout time.Duration) bool {
	// Can't resume if completed
	if d.Status == string(DownloadStatusCompleted) {
		return false
	}

	// Exceeded max retry attempts
	if maxRetries > 0 && d.RetryCount >= maxRetries {
		return false
	}

	// Check if locked by another process
	if d.LockedAt != nil {
		// If lock is stale (older than timeout), it can be resumed
		if time.Since(*d.LockedAt) < lockTimeout {
			return false
		}
	}

	// Eligible states: pending, downloading (stale), paused, failed, retrying (stale)
	return d.Status == string(DownloadStatusPending) ||
		d.Status == string(DownloadStatusDownloading) ||
		d.Status == string(DownloadStatusPaused) ||
		d.Status == string(DownloadStatusFailed) ||
		d.Status == string(DownloadStatusRetrying)
}

// HasPartialDownload returns true if there's a partial download that can be resumed
func (d *DownloadInfo) HasPartialDownload() bool {
	return d.BytesDownloaded != nil && *d.BytesDownloaded > 0 && d.TotalBytes != nil && *d.BytesDownloaded < *d.TotalBytes
}

// IsLocked returns true if the download is currently locked
func (d *DownloadInfo) IsLocked(lockTimeout time.Duration) bool {
	if d.LockedAt == nil {
		return false
	}
	return time.Since(*d.LockedAt) < lockTimeout
}
