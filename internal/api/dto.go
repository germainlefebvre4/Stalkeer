package api

import "github.com/glefebvre/stalkeer/internal/models"

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// PaginatedResponse wraps paginated results with metadata
type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Total      int64       `json:"total"`
	Limit      int         `json:"limit"`
	Offset     int         `json:"offset"`
	TotalPages int         `json:"total_pages"`
}

// ItemResponse represents a processed line response
type ItemResponse struct {
	ID          uint                   `json:"id"`
	TvgName     string                 `json:"tvg_name"`
	GroupTitle  string                 `json:"group_title"`
	ContentType models.ContentType     `json:"content_type"`
	State       models.ProcessingState `json:"state"`
	Season      *int                   `json:"season,omitempty"`
	Episode     *int                   `json:"episode,omitempty"`
	Resolution  *string                `json:"resolution,omitempty"`
	Movie       *MovieResponse         `json:"movie,omitempty"`
	TVShow      *TVShowResponse        `json:"tvshow,omitempty"`
	ProcessedAt string                 `json:"processed_at"`
	CreatedAt   string                 `json:"created_at"`
	UpdatedAt   string                 `json:"updated_at"`
}

// MovieResponse represents movie data
type MovieResponse struct {
	ID        uint    `json:"id"`
	TMDBID    int     `json:"tmdb_id"`
	TMDBTitle string  `json:"tmdb_title"`
	TMDBYear  int     `json:"tmdb_year"`
	Genres    *string `json:"genres,omitempty"`
	Duration  *int    `json:"duration,omitempty"`
}

// TVShowResponse represents TV show data
type TVShowResponse struct {
	ID        uint    `json:"id"`
	TMDBID    int     `json:"tmdb_id"`
	TMDBTitle string  `json:"tmdb_title"`
	TMDBYear  int     `json:"tmdb_year"`
	Genres    *string `json:"genres,omitempty"`
	Season    *int    `json:"season,omitempty"`
	Episode   *int    `json:"episode,omitempty"`
}

// FilterResponse represents a filter configuration
type FilterResponse struct {
	ID              uint    `json:"id"`
	Name            string  `json:"name"`
	Attribute       string  `json:"attribute"`
	IncludePatterns *string `json:"include_patterns,omitempty"`
	ExcludePatterns *string `json:"exclude_patterns,omitempty"`
	IsRuntime       bool    `json:"is_runtime"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

// StatsResponse represents statistics
type StatsResponse struct {
	TotalItems          int64            `json:"total_items"`
	ByContentType       map[string]int64 `json:"by_content_type"`
	ByResolution        map[string]int64 `json:"by_resolution"`
	ByState             map[string]int64 `json:"by_state"`
	TopGroups           []GroupCount     `json:"top_groups"`
	ProcessingTimestamp string           `json:"processing_timestamp,omitempty"`
}

// GroupCount represents group count data
type GroupCount struct {
	GroupTitle string `json:"group_title"`
	Count      int64  `json:"count"`
}

// UpdateItemRequest represents update request for an item
type UpdateItemRequest struct {
	ContentType *models.ContentType `json:"content_type,omitempty"`
	Season      *int                `json:"season,omitempty"`
	Episode     *int                `json:"episode,omitempty"`
	Resolution  *string             `json:"resolution,omitempty"`
}

// CreateFilterRequest represents create filter request
type CreateFilterRequest struct {
	Name            string  `json:"name" binding:"required"`
	Attribute       string  `json:"attribute" binding:"required"`
	IncludePatterns *string `json:"include_patterns,omitempty"`
	ExcludePatterns *string `json:"exclude_patterns,omitempty"`
}

// UpdateFilterRequest represents update filter request
type UpdateFilterRequest struct {
	Name            *string `json:"name,omitempty"`
	Attribute       *string `json:"attribute,omitempty"`
	IncludePatterns *string `json:"include_patterns,omitempty"`
	ExcludePatterns *string `json:"exclude_patterns,omitempty"`
}
