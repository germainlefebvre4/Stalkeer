package processor

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/glefebvre/stalkeer/internal/classifier"
	"github.com/glefebvre/stalkeer/internal/config"
	"github.com/glefebvre/stalkeer/internal/database"
	"github.com/glefebvre/stalkeer/internal/external/tmdb"
	"github.com/glefebvre/stalkeer/internal/filter"
	"github.com/glefebvre/stalkeer/internal/logger"
	"github.com/glefebvre/stalkeer/internal/models"
	"github.com/glefebvre/stalkeer/internal/parser"
	"gorm.io/gorm"
)

// ProcessOptions holds configuration for processing
type ProcessOptions struct {
	Force            bool
	Limit            int
	BatchSize        int
	ProgressInterval int
	SkipTMDB         bool
	TMDBLanguage     string
}

// Statistics holds processing statistics
type Statistics struct {
	TotalLines      int
	Processed       int
	DuplicatesFound int
	FilteredOut     int
	Errors          int
	Movies          int
	TVShows         int
	Channels        int
	Uncategorized   int
	TMDBMatched     int
	TMDBNotFound    int
	TMDBErrors      int
	Duration        time.Duration
	ErrorMessages   []string
}

// Processor handles M3U playlist processing
type Processor struct {
	filePath   string
	parser     *parser.Parser
	classifier *classifier.Classifier
	filter     *filter.Manager
	tmdbClient *tmdb.Client
	logger     *logger.Logger
	db         *gorm.DB
}

// NewProcessor creates a new processor instance
func NewProcessor(filePath string) (*Processor, error) {
	log := logger.AppLogger()

	db := database.Get()
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	p := parser.NewParserWithLogger(filePath, log)
	c := classifier.New()
	f := filter.NewManager()

	// Load filters from config and database
	if err := f.LoadAll(); err != nil {
		log.WithFields(map[string]interface{}{
			"error": err,
		}).Warn("failed to load filters, continuing without filters")
	}
	// Initialize TMDB client if enabled
	var tmdbClient *tmdb.Client
	cfg := config.Get()
	if cfg.TMDB.Enabled && cfg.TMDB.APIKey != "" {
		tmdbClient = tmdb.NewClient(tmdb.Config{
			APIKey:   cfg.TMDB.APIKey,
			Language: cfg.TMDB.Language,
		})
		log.Info("TMDB client initialized")
	} else {
		log.Warn("TMDB integration disabled or API key not configured")
	}

	return &Processor{
		filePath:   filePath,
		parser:     p,
		classifier: c,
		filter:     f,
		tmdbClient: tmdbClient,
		logger:     log,
		db:         db,
	}, nil
}

// Process parses and processes the M3U file
func (p *Processor) Process(opts ProcessOptions) (*Statistics, error) {
	startTime := time.Now()

	stats := &Statistics{
		ErrorMessages: make([]string, 0),
	}

	p.logger.WithFields(map[string]interface{}{
		"file":  p.filePath,
		"limit": opts.Limit,
		"force": opts.Force,
	}).Info("starting M3U processing")

	// Create processing log entry
	logEntry := &models.ProcessingLog{
		Action:    "process_m3u",
		Status:    "in_progress",
		StartedAt: time.Now(),
	}
	if err := p.db.Create(logEntry).Error; err != nil {
		return nil, fmt.Errorf("failed to create processing log: %w", err)
	}

	// Parse the M3U file
	lines, err := p.parser.Parse()
	if err != nil {
		p.updateProcessingLog(logEntry, "failed", stats, err.Error())
		return nil, fmt.Errorf("failed to parse M3U file: %w", err)
	}

	stats.TotalLines = len(lines)

	// Process entries in batches
	if opts.BatchSize <= 0 {
		opts.BatchSize = 100
	}
	if opts.ProgressInterval <= 0 {
		opts.ProgressInterval = 1000
	}

	batch := make([]*models.ProcessedLine, 0, opts.BatchSize)
	processed := 0

	for i, line := range lines {
		// Check limit
		if opts.Limit > 0 && processed >= opts.Limit {
			p.logger.Info(fmt.Sprintf("reached processing limit of %d entries", opts.Limit))
			break
		}

		// Check for duplicate
		if !opts.Force {
			exists, err := p.checkDuplicate(line.LineHash)
			if err != nil {
				stats.Errors++
				errMsg := fmt.Sprintf("error checking duplicate for line %d: %v", i+1, err)
				stats.ErrorMessages = append(stats.ErrorMessages, errMsg)
				continue
			}
			if exists {
				stats.DuplicatesFound++
				continue
			}
		}

		// Apply filters
		if !p.filter.ShouldProcess(line.GroupTitle, line.TvgName) {
			stats.FilteredOut++
			continue
		}

		// Classify content
		classification := p.classifier.Classify(line.TvgName, line.GroupTitle)

		// Set content type and create associations (with TMDB enrichment)
		if err := p.setContentType(&line, classification, &opts, stats); err != nil {
			stats.Errors++
			errMsg := fmt.Sprintf("error setting content type for line %d: %v", i+1, err)
			stats.ErrorMessages = append(stats.ErrorMessages, errMsg)
			continue
		}

		// Add to batch
		batch = append(batch, &line)

		// Process batch when full
		if len(batch) >= opts.BatchSize {
			if err := p.saveBatch(batch, stats); err != nil {
				stats.Errors++
				errMsg := fmt.Sprintf("error saving batch: %v", err)
				stats.ErrorMessages = append(stats.ErrorMessages, errMsg)
			}
			batch = batch[:0]
		}

		processed++

		// Show progress
		if processed%opts.ProgressInterval == 0 {
			p.logger.Info(fmt.Sprintf("processed %d/%d entries", processed, stats.TotalLines))
		}
	}

	// Process remaining entries in batch
	if len(batch) > 0 {
		if err := p.saveBatch(batch, stats); err != nil {
			stats.Errors++
			errMsg := fmt.Sprintf("error saving final batch: %v", err)
			stats.ErrorMessages = append(stats.ErrorMessages, errMsg)
		}
	}

	stats.Duration = time.Since(startTime)

	// Update processing log
	status := "success"
	var errorMsg *string
	if stats.Errors > 0 {
		status = "completed_with_errors"
		msg := fmt.Sprintf("%d errors occurred during processing", stats.Errors)
		errorMsg = &msg
	}
	p.updateProcessingLog(logEntry, status, stats, "")
	if errorMsg != nil {
		logEntry.ErrorMessage = errorMsg
		p.db.Save(logEntry)
	}

	p.logger.WithFields(map[string]interface{}{
		"processed":        stats.Processed,
		"duplicates":       stats.DuplicatesFound,
		"filtered":         stats.FilteredOut,
		"errors":           stats.Errors,
		"duration_seconds": stats.Duration.Seconds(),
	}).Info("processing completed")

	return stats, nil
}

// checkDuplicate checks if a line with the given hash already exists
func (p *Processor) checkDuplicate(lineHash string) (bool, error) {
	var count int64
	err := p.db.Model(&models.ProcessedLine{}).Where("line_hash = ?", lineHash).Count(&count).Error
	return count > 0, err
}

// setContentType sets the content type and creates necessary associations with TMDB enrichment
func (p *Processor) setContentType(line *models.ProcessedLine, classification classifier.Classification, opts *ProcessOptions, stats *Statistics) error {
	// Determine language for TMDB
	language := opts.TMDBLanguage
	if language == "" {
		cfg := config.Get()
		language = cfg.TMDB.Language
		if language == "" {
			language = "en-US"
		}
	}

	switch classification.ContentType {
	case classifier.ContentTypeMovie:
		line.ContentType = models.ContentTypeMovies

		// Try to enrich with TMDB if enabled
		if !opts.SkipTMDB && p.tmdbClient != nil {
			if err := p.enrichMovie(line, language, stats); err != nil {
				// Log error but don't fail the processing
				p.logger.WithFields(map[string]interface{}{
					"title": line.TvgName,
					"error": err,
				}).Warn("failed to enrich movie with TMDB")
			}
		}
		return nil

	case classifier.ContentTypeSeries:
		line.ContentType = models.ContentTypeTVShows

		// Try to enrich with TMDB if enabled
		if !opts.SkipTMDB && p.tmdbClient != nil {
			if err := p.enrichTVShow(line, classification, language, stats); err != nil {
				// Log error but don't fail the processing
				p.logger.WithFields(map[string]interface{}{
					"title": line.TvgName,
					"error": err,
				}).Warn("failed to enrich TV show with TMDB")
			}
		}
		return nil

	default:
		line.ContentType = models.ContentTypeUncategorized
		return nil
	}
}

// enrichMovie fetches movie data from TMDB and creates/updates Movie association
func (p *Processor) enrichMovie(line *models.ProcessedLine, language string, stats *Statistics) error {
	// Extract title and year from tvg-name
	title, year := p.extractTitleAndYear(line.TvgName)

	// Search TMDB
	result, err := p.tmdbClient.SearchMovie(title, year)
	if err != nil {
		stats.TMDBNotFound++
		return err
	}

	// Get detailed information
	details, err := p.tmdbClient.GetMovieDetails(result.ID)
	if err != nil {
		stats.TMDBErrors++
		return err
	}

	// Get external IDs (including TVDB ID)
	externalIDs, err := p.tmdbClient.GetMovieExternalIDs(result.ID)
	if err != nil {
		// Log warning but don't fail - external IDs are optional
		p.logger.WithFields(map[string]interface{}{
			"tmdb_id": result.ID,
			"error":   err,
		}).Warn("Failed to fetch movie external IDs")
	}

	// Create or find existing movie
	var movie models.Movie
	tmdbYear := tmdb.ExtractYear(details.ReleaseDate)
	genres := tmdb.FormatGenres(details.Genres)

	// Check if movie already exists
	err = p.db.Where("tmdb_id = ? AND tmdb_year = ?", details.ID, tmdbYear).First(&movie).Error
	if err == gorm.ErrRecordNotFound {
		// Create new movie
		movie = models.Movie{
			TMDBID:     details.ID,
			TVDBID:     externalIDs.TVDBID,
			TMDBTitle:  details.Title,
			TMDBYear:   tmdbYear,
			TMDBGenres: &genres,
			Duration:   details.Runtime,
		}
		if err := p.db.Create(&movie).Error; err != nil {
			stats.TMDBErrors++
			return fmt.Errorf("failed to create movie: %w", err)
		}
	} else if err != nil {
		stats.TMDBErrors++
		return fmt.Errorf("failed to check for existing movie: %w", err)
	} else if externalIDs != nil && externalIDs.TVDBID != nil && movie.TVDBID == nil {
		// Update existing movie with TVDB ID if it's missing
		movie.TVDBID = externalIDs.TVDBID
		if err := p.db.Save(&movie).Error; err != nil {
			p.logger.WithFields(map[string]interface{}{
				"movie_id": movie.ID,
				"error":    err,
			}).Warn("Failed to update movie with TVDB ID")
		}
	}

	// Associate with processed line
	line.MovieID = &movie.ID
	stats.TMDBMatched++

	return nil
}

// enrichTVShow fetches TV show data from TMDB and creates/updates TVShow association
func (p *Processor) enrichTVShow(line *models.ProcessedLine, classification classifier.Classification, language string, stats *Statistics) error {
	// Extract title from tvg-name (remove season/episode info)
	title := p.cleanTVShowTitle(line.TvgName)

	// Search TMDB
	result, err := p.tmdbClient.SearchTVShow(title)
	if err != nil {
		stats.TMDBNotFound++
		return err
	}

	// Get detailed information
	details, err := p.tmdbClient.GetTVShowDetails(result.ID)
	if err != nil {
		stats.TMDBErrors++
		return err
	}

	// Get external IDs (including TVDB ID)
	externalIDs, err := p.tmdbClient.GetTVShowExternalIDs(result.ID)
	if err != nil {
		// Log warning but don't fail - external IDs are optional
		p.logger.WithFields(map[string]interface{}{
			"tmdb_id": result.ID,
			"error":   err,
		}).Warn("Failed to fetch TV show external IDs")
	}

	// Create or find existing TV show
	var tvshow models.TVShow
	tmdbYear := tmdb.ExtractYear(details.FirstAirDate)
	genres := tmdb.FormatGenres(details.Genres)

	// Check if TV show already exists with same TMDB ID, season, and episode
	query := p.db.Where("tmdb_id = ?", details.ID)
	if classification.Season != nil {
		query = query.Where("season = ?", *classification.Season)
	} else {
		query = query.Where("season IS NULL")
	}
	if classification.Episode != nil {
		query = query.Where("episode = ?", *classification.Episode)
	} else {
		query = query.Where("episode IS NULL")
	}

	err = query.First(&tvshow).Error
	if err == gorm.ErrRecordNotFound {
		// Create new TV show entry
		tvshow = models.TVShow{
			TMDBID:     details.ID,
			TVDBID:     externalIDs.TVDBID,
			TMDBTitle:  details.Name,
			TMDBYear:   tmdbYear,
			TMDBGenres: &genres,
			Season:     classification.Season,
			Episode:    classification.Episode,
		}
		if err := p.db.Create(&tvshow).Error; err != nil {
			stats.TMDBErrors++
			return fmt.Errorf("failed to create TV show: %w", err)
		}
	} else if err != nil {
		stats.TMDBErrors++
		return fmt.Errorf("failed to check for existing TV show: %w", err)
	} else if externalIDs != nil && externalIDs.TVDBID != nil && tvshow.TVDBID == nil {
		// Update existing TV show with TVDB ID if it's missing
		tvshow.TVDBID = externalIDs.TVDBID
		if err := p.db.Save(&tvshow).Error; err != nil {
			p.logger.WithFields(map[string]interface{}{
				"tvshow_id": tvshow.ID,
				"error":     err,
			}).Warn("Failed to update TV show with TVDB ID")
		}
	}

	// Associate with processed line
	line.TVShowID = &tvshow.ID
	stats.TMDBMatched++

	return nil
}

// extractTitleAndYear extracts title and optional year from a string
func (p *Processor) extractTitleAndYear(title string) (string, *int) {
	// Look for year in parentheses like "Movie Title (2024)"
	if strings.Contains(title, "(") {
		parts := strings.Split(title, "(")
		cleanTitle := strings.TrimSpace(parts[0])

		// Try to extract year
		for i := 1; i < len(parts); i++ {
			if strings.Contains(parts[i], ")") {
				yearStr := strings.TrimSuffix(parts[i], ")")
				var year int
				if _, err := fmt.Sscanf(yearStr, "%d", &year); err == nil && year >= 1900 && year <= 2100 {
					return cleanTitle, &year
				}
			}
		}
		return cleanTitle, nil
	}

	return strings.TrimSpace(title), nil
}

// cleanTVShowTitle removes season/episode markers and quality tags from title
func (p *Processor) cleanTVShowTitle(title string) string {
	// Remove common patterns like "S01 E01", "S01E01", quality tags, etc.
	patterns := []string{
		`\s+S\d{2}\s*E\d{2}`,                                 // S01 E01
		`\s+S\d{2}E\d{2}`,                                    // S01E01
		`\s+\d{1,2}x\d{1,2}`,                                 // 1x01
		`\s+\(\d{4}\)`,                                       // (2024)
		`\s+\(.*?(HD|SD|4K|1080p|720p|480p).*?\)`,            // Quality tags
		`\s+(HD|FHD|UHD|4K|1080p|720p|480p|SD|SDTV|HDTV).*$`, // Quality suffixes
		`\s+\(MULTI\)`,                                       // Language tags
		`\s+\(VOSTFR\)`,
		`\s+\(VF\)`,
	}

	cleanTitle := title
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		cleanTitle = re.ReplaceAllString(cleanTitle, "")
	}

	return strings.TrimSpace(cleanTitle)
}

// saveBatch saves a batch of processed lines to the database
func (p *Processor) saveBatch(batch []*models.ProcessedLine, stats *Statistics) error {
	return p.db.Transaction(func(tx *gorm.DB) error {
		for _, line := range batch {
			// Set timestamps
			now := time.Now()
			line.ProcessedAt = now
			line.State = models.StateProcessed
			line.CreatedAt = now
			line.UpdatedAt = now

			// Check if entry exists and handle based on force mode
			var existing models.ProcessedLine
			err := tx.Where("line_hash = ?", line.LineHash).First(&existing).Error

			if err == nil {
				// Entry exists - update it
				line.ID = existing.ID
				line.CreatedAt = existing.CreatedAt
				if err := tx.Save(line).Error; err != nil {
					return fmt.Errorf("failed to update processed line: %w", err)
				}
			} else if err == gorm.ErrRecordNotFound {
				// Entry doesn't exist - create it
				if err := tx.Create(line).Error; err != nil {
					return fmt.Errorf("failed to create processed line: %w", err)
				}
			} else {
				return fmt.Errorf("failed to check for existing line: %w", err)
			}

			// Update statistics
			stats.Processed++
			switch line.ContentType {
			case models.ContentTypeMovies:
				stats.Movies++
			case models.ContentTypeTVShows:
				stats.TVShows++
			case models.ContentTypeChannels:
				stats.Channels++
			case models.ContentTypeUncategorized:
				stats.Uncategorized++
			}
		}
		return nil
	})
}

// updateProcessingLog updates the processing log entry with final statistics
func (p *Processor) updateProcessingLog(logEntry *models.ProcessingLog, status string, stats *Statistics, errorMsg string) {
	now := time.Now()
	logEntry.Status = status
	logEntry.ItemCount = stats.Processed
	logEntry.CompletedAt = &now
	if errorMsg != "" {
		logEntry.ErrorMessage = &errorMsg
	}
	p.db.Save(logEntry)
}

// computeLineHash generates a SHA-256 hash for a line
func computeLineHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}
