package models

import "time"

// Movie represents movie metadata from TMDB
type Movie struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	TMDBID     int       `gorm:"not null;index:idx_movies_tmdb" json:"tmdb_id"`
	TVDBID     *int      `gorm:"index:idx_movies_tvdb" json:"tvdb_id,omitempty"`
	TMDBTitle  string    `gorm:"type:varchar(255);not null;uniqueIndex:idx_movies_unique,composite:tmdb_title_year" json:"tmdb_title"`
	TMDBYear   int       `gorm:"not null;uniqueIndex:idx_movies_unique,composite:tmdb_title_year" json:"tmdb_year"`
	TMDBGenres *string   `gorm:"type:text" json:"tmdb_genres,omitempty"`
	Duration   *int      `json:"duration,omitempty"`
	CreatedAt  time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt  time.Time `gorm:"not null" json:"updated_at"`

	// Associations
	ProcessedLines []ProcessedLine `gorm:"foreignKey:MovieID" json:"processed_lines,omitempty"`
}

// TableName specifies the table name for Movie
func (Movie) TableName() string {
	return "movies"
}

// FilterConfig represents runtime filters stored in the database
type FilterConfig struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	Name            string    `gorm:"type:varchar(255);not null;uniqueIndex" json:"name"`
	Attribute       string    `gorm:"type:varchar(50);not null" json:"attribute"`  // "group_title" or "tvg_name"
	IncludePatterns *string   `gorm:"type:text" json:"include_patterns,omitempty"` // JSON array
	ExcludePatterns *string   `gorm:"type:text" json:"exclude_patterns,omitempty"` // JSON array
	IsRuntime       bool      `gorm:"not null;default:true;index" json:"is_runtime"`
	CreatedAt       time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt       time.Time `gorm:"not null" json:"updated_at"`
}

// TableName specifies the table name for FilterConfig
func (FilterConfig) TableName() string {
	return "filter_configs"
}

// Channel represents live TV channel metadata
type Channel struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Name       string    `gorm:"type:varchar(255);not null" json:"name"`
	Logo       *string   `gorm:"type:text" json:"logo,omitempty"`
	GroupTitle string    `gorm:"type:varchar(255);not null;index" json:"group_title"`
	CreatedAt  time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt  time.Time `gorm:"not null" json:"updated_at"`

	// Associations
	ProcessedLines []ProcessedLine `gorm:"foreignKey:ChannelID" json:"processed_lines,omitempty"`
}

// TableName specifies the table name for Channel
func (Channel) TableName() string {
	return "channels"
}

// Uncategorized represents content that couldn't be classified
type Uncategorized struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Title      string    `gorm:"type:varchar(255);not null" json:"title"`
	GroupTitle string    `gorm:"type:varchar(255);not null;index" json:"group_title"`
	CreatedAt  time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt  time.Time `gorm:"not null" json:"updated_at"`

	// Associations
	ProcessedLines []ProcessedLine `gorm:"foreignKey:UncategorizedID" json:"processed_lines,omitempty"`
}

// TableName specifies the table name for Uncategorized
func (Uncategorized) TableName() string {
	return "uncategorized"
}
