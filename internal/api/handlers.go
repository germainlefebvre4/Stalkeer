package api

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/glefebvre/stalkeer/internal/config"
	"github.com/glefebvre/stalkeer/internal/database"
	"github.com/glefebvre/stalkeer/internal/dryrun"
	"github.com/glefebvre/stalkeer/internal/models"
	"gorm.io/gorm"
)

const (
	defaultLimit = 20
	maxLimit     = 1000
)

// healthCheck returns the health status of the API and database
func (s *Server) healthCheck(c *gin.Context) {
	if err := database.HealthCheck(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
	})
}

// listItems returns paginated list of items with filtering and sorting
func (s *Server) listItems(c *gin.Context) {
	db := database.Get()

	// Parse pagination params
	limit, offset := parsePagination(c)

	// Parse filters
	contentType := c.Query("content_type")
	state := c.Query("state")
	groupTitle := c.Query("group_title")

	// Parse sort
	sortBy := c.DefaultQuery("sort", "created_at")
	sortOrder := c.DefaultQuery("order", "desc")

	// Validate sort field
	validSortFields := map[string]bool{
		"tvg_name":     true,
		"created_at":   true,
		"processed_at": true,
		"group_title":  true,
	}
	if !validSortFields[sortBy] {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_sort_field",
			Message: fmt.Sprintf("invalid sort field: %s", sortBy),
		})
		return
	}

	// Build query
	query := db.Model(&models.ProcessedLine{}).Preload("Movie").Preload("TVShow")

	if contentType != "" {
		query = query.Where("content_type = ?", contentType)
	}
	if state != "" {
		query = query.Where("state = ?", state)
	}
	if groupTitle != "" {
		query = query.Where("group_title ILIKE ?", "%"+groupTitle+"%")
	}

	// Count total
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "failed to count items",
		})
		return
	}

	// Apply sorting and pagination
	orderClause := fmt.Sprintf("%s %s", sortBy, strings.ToUpper(sortOrder))
	query = query.Order(orderClause).Limit(limit).Offset(offset)

	// Fetch items
	var items []models.ProcessedLine
	if err := query.Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "failed to fetch items",
		})
		return
	}

	// Convert to response DTOs
	responses := make([]ItemResponse, len(items))
	for i, item := range items {
		responses[i] = toItemResponse(item)
	}

	// Calculate total pages
	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	c.JSON(http.StatusOK, PaginatedResponse{
		Data:       responses,
		Total:      total,
		Limit:      limit,
		Offset:     offset,
		TotalPages: totalPages,
	})
}

// getItem returns a single item by ID
func (s *Server) getItem(c *gin.Context) {
	db := database.Get()
	id := c.Param("id")

	var item models.ProcessedLine
	if err := db.Preload("Movie").Preload("TVShow").First(&item, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "not_found",
				Message: fmt.Sprintf("item with id %s not found", id),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "failed to fetch item",
		})
		return
	}

	c.JSON(http.StatusOK, toItemResponse(item))
}

// updateItem updates item metadata
func (s *Server) updateItem(c *gin.Context) {
	db := database.Get()
	id := c.Param("id")

	var req UpdateItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	var item models.ProcessedLine
	if err := db.First(&item, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "not_found",
				Message: fmt.Sprintf("item with id %s not found", id),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "failed to fetch item",
		})
		return
	}

	// Update fields
	if req.ContentType != nil {
		item.ContentType = *req.ContentType
	}

	if err := db.Save(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "failed to update item",
		})
		return
	}

	c.JSON(http.StatusOK, toItemResponse(item))
}

// searchItems performs advanced search
func (s *Server) searchItems(c *gin.Context) {
	db := database.Get()

	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "query parameter 'q' is required",
		})
		return
	}

	limit, offset := parsePagination(c)

	// Build search query
	dbQuery := db.Model(&models.ProcessedLine{}).
		Preload("Movie").
		Preload("TVShow").
		Where("tvg_name ILIKE ? OR group_title ILIKE ?", "%"+query+"%", "%"+query+"%")

	// Count total
	var total int64
	if err := dbQuery.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "failed to count results",
		})
		return
	}

	// Fetch results
	var items []models.ProcessedLine
	if err := dbQuery.Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "failed to search items",
		})
		return
	}

	// Convert to response DTOs
	responses := make([]ItemResponse, len(items))
	for i, item := range items {
		responses[i] = toItemResponse(item)
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	c.JSON(http.StatusOK, PaginatedResponse{
		Data:       responses,
		Total:      total,
		Limit:      limit,
		Offset:     offset,
		TotalPages: totalPages,
	})
}

// listMovies returns paginated list of movies
func (s *Server) listMovies(c *gin.Context) {
	db := database.Get()
	limit, offset := parsePagination(c)

	var total int64
	if err := db.Model(&models.Movie{}).Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "failed to count movies",
		})
		return
	}

	var movies []models.Movie
	if err := db.Limit(limit).Offset(offset).Find(&movies).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "failed to fetch movies",
		})
		return
	}

	responses := make([]MovieResponse, len(movies))
	for i, movie := range movies {
		responses[i] = toMovieResponse(movie)
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	c.JSON(http.StatusOK, PaginatedResponse{
		Data:       responses,
		Total:      total,
		Limit:      limit,
		Offset:     offset,
		TotalPages: totalPages,
	})
}

// getMovie returns a single movie by ID
func (s *Server) getMovie(c *gin.Context) {
	db := database.Get()
	id := c.Param("id")

	var movie models.Movie
	if err := db.Preload("ProcessedLines").First(&movie, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "not_found",
				Message: fmt.Sprintf("movie with id %s not found", id),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "failed to fetch movie",
		})
		return
	}

	c.JSON(http.StatusOK, toMovieResponse(movie))
}

// listTVShows returns paginated list of TV shows
func (s *Server) listTVShows(c *gin.Context) {
	db := database.Get()
	limit, offset := parsePagination(c)

	var total int64
	if err := db.Model(&models.TVShow{}).Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "failed to count TV shows",
		})
		return
	}

	var tvShows []models.TVShow
	if err := db.Limit(limit).Offset(offset).Find(&tvShows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "failed to fetch TV shows",
		})
		return
	}

	responses := make([]TVShowResponse, len(tvShows))
	for i, tvShow := range tvShows {
		responses[i] = toTVShowResponse(tvShow)
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	c.JSON(http.StatusOK, PaginatedResponse{
		Data:       responses,
		Total:      total,
		Limit:      limit,
		Offset:     offset,
		TotalPages: totalPages,
	})
}

// getTVShow returns a single TV show by ID
func (s *Server) getTVShow(c *gin.Context) {
	db := database.Get()
	id := c.Param("id")

	var tvShow models.TVShow
	if err := db.Preload("ProcessedLines").First(&tvShow, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "not_found",
				Message: fmt.Sprintf("TV show with id %s not found", id),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "failed to fetch TV show",
		})
		return
	}

	c.JSON(http.StatusOK, toTVShowResponse(tvShow))
}

// listFilters returns all filter configurations
func (s *Server) listFilters(c *gin.Context) {
	db := database.Get()

	var filters []models.FilterConfig
	if err := db.Find(&filters).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "failed to fetch filters",
		})
		return
	}

	responses := make([]FilterResponse, len(filters))
	for i, filter := range filters {
		responses[i] = toFilterResponse(filter)
	}

	c.JSON(http.StatusOK, gin.H{
		"filters": responses,
	})
}

// createFilter creates a new runtime filter
func (s *Server) createFilter(c *gin.Context) {
	db := database.Get()

	var req CreateFilterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	// Validate attribute
	if req.Attribute != "group_title" && req.Attribute != "tvg_name" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_attribute",
			Message: "attribute must be 'group_title' or 'tvg_name'",
		})
		return
	}

	filter := models.FilterConfig{
		Name:            req.Name,
		Attribute:       req.Attribute,
		IncludePatterns: req.IncludePatterns,
		ExcludePatterns: req.ExcludePatterns,
		IsRuntime:       true,
	}

	if err := db.Create(&filter).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "failed to create filter",
		})
		return
	}

	c.JSON(http.StatusCreated, toFilterResponse(filter))
}

// updateFilter updates an existing filter
func (s *Server) updateFilter(c *gin.Context) {
	db := database.Get()
	id := c.Param("id")

	var req UpdateFilterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	var filter models.FilterConfig
	if err := db.First(&filter, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error:   "not_found",
				Message: fmt.Sprintf("filter with id %s not found", id),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "failed to fetch filter",
		})
		return
	}

	// Update fields
	if req.Name != nil {
		filter.Name = *req.Name
	}
	if req.Attribute != nil {
		if *req.Attribute != "group_title" && *req.Attribute != "tvg_name" {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "invalid_attribute",
				Message: "attribute must be 'group_title' or 'tvg_name'",
			})
			return
		}
		filter.Attribute = *req.Attribute
	}
	if req.IncludePatterns != nil {
		filter.IncludePatterns = req.IncludePatterns
	}
	if req.ExcludePatterns != nil {
		filter.ExcludePatterns = req.ExcludePatterns
	}

	if err := db.Save(&filter).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "failed to update filter",
		})
		return
	}

	c.JSON(http.StatusOK, toFilterResponse(filter))
}

// deleteFilter deletes a filter
func (s *Server) deleteFilter(c *gin.Context) {
	db := database.Get()
	id := c.Param("id")

	result := db.Delete(&models.FilterConfig{}, id)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "failed to delete filter",
		})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: fmt.Sprintf("filter with id %s not found", id),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "filter deleted successfully",
	})
}

// clearRuntimeFilters deletes all runtime filters
func (s *Server) clearRuntimeFilters(c *gin.Context) {
	db := database.Get()

	if err := db.Where("is_runtime = ?", true).Delete(&models.FilterConfig{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "failed to clear runtime filters",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "runtime filters cleared successfully",
	})
}

// getStats returns statistics about the data
func (s *Server) getStats(c *gin.Context) {
	db := database.Get()

	var totalItems int64
	if err := db.Model(&models.ProcessedLine{}).Count(&totalItems).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "failed to count items",
		})
		return
	}

	// Count by content type
	byContentType := make(map[string]int64)
	contentTypes := []models.ContentType{
		models.ContentTypeMovies,
		models.ContentTypeTVShows,
		models.ContentTypeChannels,
		models.ContentTypeUncategorized,
	}
	for _, ct := range contentTypes {
		var count int64
		db.Model(&models.ProcessedLine{}).Where("content_type = ?", ct).Count(&count)
		byContentType[string(ct)] = count
	}

	// Count by state
	byState := make(map[string]int64)
	states := []models.ProcessingState{
		models.StateProcessed,
		models.StatePending,
		models.StateDownloading,
		models.StateDownloaded,
		models.StateFailed,
	}
	for _, state := range states {
		var count int64
		db.Model(&models.ProcessedLine{}).Where("state = ?", state).Count(&count)
		byState[string(state)] = count
	}

	// Top 10 groups
	var topGroups []GroupCount
	db.Model(&models.ProcessedLine{}).
		Select("group_title, COUNT(*) as count").
		Group("group_title").
		Order("count DESC").
		Limit(10).
		Scan(&topGroups)

	c.JSON(http.StatusOK, StatsResponse{
		TotalItems:    totalItems,
		ByContentType: byContentType,
		ByState:       byState,
		TopGroups:     topGroups,
	})
}

// executeDryRun executes a dry-run analysis
func (s *Server) executeDryRun(c *gin.Context) {
	cfg := config.Get()
	filePath := cfg.M3U.FilePath

	var req struct {
		FilePath *string `json:"file_path,omitempty"`
		Limit    int     `json:"limit"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	if req.FilePath != nil {
		filePath = *req.FilePath
	}

	if filePath == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "missing_file_path",
			Message: "M3U file path must be provided",
		})
		return
	}

	limit := req.Limit
	if limit == 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	analyzer := dryrun.NewAnalyzer(limit)
	result, err := analyzer.Analyze(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "analysis_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// Helper functions

func parsePagination(c *gin.Context) (limit, offset int) {
	limit = defaultLimit
	offset = 0

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
			if limit > maxLimit {
				limit = maxLimit
			}
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	return limit, offset
}

func toItemResponse(item models.ProcessedLine) ItemResponse {
	resp := ItemResponse{
		ID:          item.ID,
		TvgName:     item.TvgName,
		GroupTitle:  item.GroupTitle,
		ContentType: item.ContentType,
		State:       item.State,
		ProcessedAt: item.ProcessedAt.Format("2006-01-02T15:04:05Z07:00"),
		CreatedAt:   item.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   item.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if item.Movie != nil {
		movie := toMovieResponse(*item.Movie)
		resp.Movie = &movie
	}

	if item.TVShow != nil {
		tvshow := toTVShowResponse(*item.TVShow)
		resp.TVShow = &tvshow
		resp.Season = tvshow.Season
		resp.Episode = tvshow.Episode
	}

	return resp
}

func toMovieResponse(movie models.Movie) MovieResponse {
	return MovieResponse{
		ID:        movie.ID,
		TMDBID:    movie.TMDBID,
		TMDBTitle: movie.TMDBTitle,
		TMDBYear:  movie.TMDBYear,
		Genres:    movie.TMDBGenres,
		Duration:  movie.Duration,
	}
}

func toTVShowResponse(tvShow models.TVShow) TVShowResponse {
	return TVShowResponse{
		ID:        tvShow.ID,
		TMDBID:    tvShow.TMDBID,
		TMDBTitle: tvShow.TMDBTitle,
		TMDBYear:  tvShow.TMDBYear,
		Genres:    tvShow.TMDBGenres,
		Season:    tvShow.Season,
		Episode:   tvShow.Episode,
	}
}

func toFilterResponse(filter models.FilterConfig) FilterResponse {
	return FilterResponse{
		ID:              filter.ID,
		Name:            filter.Name,
		Attribute:       filter.Attribute,
		IncludePatterns: filter.IncludePatterns,
		ExcludePatterns: filter.ExcludePatterns,
		IsRuntime:       filter.IsRuntime,
		CreatedAt:       filter.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:       filter.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
