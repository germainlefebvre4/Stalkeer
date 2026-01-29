package models

import "time"

// TVShow represents TV show metadata from TMDB with season/episode information
type TVShow struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	TMDBID     int       `gorm:"not null;index:idx_tvshows_tmdb" json:"tmdb_id"`
	TMDBTitle  string    `gorm:"type:varchar(255);not null;uniqueIndex:idx_tvshows_unique" json:"tmdb_title"`
	TMDBYear   int       `gorm:"not null;uniqueIndex:idx_tvshows_unique" json:"tmdb_year"`
	TMDBGenres *string   `gorm:"type:text" json:"tmdb_genres,omitempty"`
	Season     *int      `gorm:"index:idx_tvshows_season_episode;uniqueIndex:idx_tvshows_unique" json:"season,omitempty"`
	Episode    *int      `gorm:"index:idx_tvshows_season_episode;uniqueIndex:idx_tvshows_unique" json:"episode,omitempty"`
	CreatedAt  time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt  time.Time `gorm:"not null" json:"updated_at"`

	// Associations
	ProcessedLines []ProcessedLine `gorm:"foreignKey:TVShowID" json:"processed_lines,omitempty"`
}

// TableName specifies the table name for TVShow
func (TVShow) TableName() string {
	return "tvshows"
}
