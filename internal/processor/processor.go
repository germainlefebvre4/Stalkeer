package processor

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/glefebvre/stalkeer/internal/classifier"
	"github.com/glefebvre/stalkeer/internal/database"
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
	Duration        time.Duration
	ErrorMessages   []string
}

// Processor handles M3U playlist processing
type Processor struct {
	filePath   string
	parser     *parser.Parser
	classifier *classifier.Classifier
	filter     *filter.Manager
	logger     *logger.Logger
	db         *gorm.DB
}

// NewProcessor creates a new processor instance
func NewProcessor(filePath string) (*Processor, error) {
	log := logger.Default()

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

	return &Processor{
		filePath:   filePath,
		parser:     p,
		classifier: c,
		filter:     f,
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

		// Set content type and create associations
		if err := p.setContentType(&line, classification); err != nil {
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

// setContentType sets the content type and creates necessary associations
func (p *Processor) setContentType(line *models.ProcessedLine, classification classifier.Classification) error {
	switch classification.ContentType {
	case classifier.ContentTypeMovie:
		line.ContentType = models.ContentTypeMovies
		// Movie association will be created later by enrichment service
		return nil

	case classifier.ContentTypeSeries:
		line.ContentType = models.ContentTypeTVShows
		// TVShow association will be created later by enrichment service
		// Store season/episode info for later use
		return nil

	default:
		line.ContentType = models.ContentTypeUncategorized
		return nil
	}
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
