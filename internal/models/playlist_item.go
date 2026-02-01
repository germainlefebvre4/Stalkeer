package models

import "time"

// ContentType represents the type of media content
type ContentType string

const (
	ContentTypeMovies        ContentType = "movies"
	ContentTypeTVShows       ContentType = "tvshows"
	ContentTypeChannels      ContentType = "channels"
	ContentTypeUncategorized ContentType = "uncategorized"
)

// ProcessingState represents the state of a processed line
type ProcessingState string

const (
	StateProcessed   ProcessingState = "processed"
	StatePending     ProcessingState = "pending"
	StateDownloading ProcessingState = "downloading"
	StateOrganizing  ProcessingState = "organizing"
	StateDownloaded  ProcessingState = "downloaded"
	StateFailed      ProcessingState = "failed"
)

// ProcessedLine represents an M3U playlist line with polymorphic relationships
type ProcessedLine struct {
	ID              uint            `gorm:"primaryKey" json:"id"`
	LineContent     string          `gorm:"type:text;not null" json:"line_content"`
	LineURL         *string         `gorm:"type:text" json:"line_url,omitempty"`
	LineHash        string          `gorm:"type:varchar(64);not null;uniqueIndex" json:"line_hash"`
	TvgName         string          `gorm:"type:varchar(255);not null;index:idx_processed_lines_m3u" json:"tvg_name"`
	GroupTitle      string          `gorm:"type:varchar(255);not null;index:idx_processed_lines_m3u" json:"group_title"`
	ProcessedAt     time.Time       `gorm:"not null" json:"processed_at"`
	ContentType     ContentType     `gorm:"type:varchar(20);not null;index:idx_processed_lines_content" json:"content_type"`
	ChannelID       *uint           `gorm:"index" json:"channel_id,omitempty"`
	MovieID         *uint           `gorm:"index" json:"movie_id,omitempty"`
	TVShowID        *uint           `gorm:"index" json:"tvshow_id,omitempty"`
	UncategorizedID *uint           `gorm:"index" json:"uncategorized_id,omitempty"`
	DownloadInfoID  *uint           `gorm:"index:idx_processed_lines_download" json:"download_info_id,omitempty"`
	State           ProcessingState `gorm:"type:varchar(50);not null;default:processed;index:idx_processed_lines_content" json:"state"`
	CreatedAt       time.Time       `gorm:"not null" json:"created_at"`
	UpdatedAt       time.Time       `gorm:"not null" json:"updated_at"`
	OverridesID     *uint           `gorm:"index" json:"overrides_id,omitempty"`
	OverridesAt     *time.Time      `json:"overrides_at,omitempty"`

	// Associations
	Movie     *Movie         `gorm:"foreignKey:MovieID;constraint:OnDelete=CASCADE" json:"movie,omitempty"`
	TVShow    *TVShow        `gorm:"foreignKey:TVShowID;constraint:OnDelete=CASCADE" json:"tvshow,omitempty"`
	Overrides *ProcessedLine `gorm:"foreignKey:OverridesID;constraint:OnDelete=SET NULL" json:"overrides,omitempty"`
}

// TableName specifies the table name for ProcessedLine
func (ProcessedLine) TableName() string {
	return "processed_lines"
}
