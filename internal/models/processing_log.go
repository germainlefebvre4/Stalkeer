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

// DownloadInfo represents download tracking information
type DownloadInfo struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	Status       string     `gorm:"type:varchar(50);not null" json:"status"` // "pending", "downloading", "completed", "failed"
	DownloadPath *string    `gorm:"type:text" json:"download_path,omitempty"`
	FileSize     *int64     `json:"file_size,omitempty"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	ErrorMessage *string    `gorm:"type:text" json:"error_message,omitempty"`
	CreatedAt    time.Time  `gorm:"not null" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"not null" json:"updated_at"`

	// Associations
	ProcessedLines []ProcessedLine `gorm:"foreignKey:DownloadInfoID" json:"processed_lines,omitempty"`
}

// TableName specifies the table name for DownloadInfo
func (DownloadInfo) TableName() string {
	return "download_info"
}
