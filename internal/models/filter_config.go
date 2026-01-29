package models

import "time"

// Movie represents movie metadata from TMDB
type Movie struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	TMDBID     int       `gorm:"not null;index:idx_movies_tmdb" json:"tmdb_id"`
	TMDBTitle  string    `gorm:"type:varchar(255);not null;uniqueIndex:idx_movies_unique" json:"tmdb_title"`
	TMDBYear   int       `gorm:"not null;index:idx_movies_year;uniqueIndex:idx_movies_unique" json:"tmdb_year"`
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
