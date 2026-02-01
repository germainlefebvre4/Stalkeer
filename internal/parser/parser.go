package parser

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	apperrors "github.com/glefebvre/stalkeer/internal/errors"
	"github.com/glefebvre/stalkeer/internal/logger"
	"github.com/glefebvre/stalkeer/internal/models"
)

// M3UEntry represents a parsed M3U playlist entry
type M3UEntry struct {
	TvgID      string
	TvgName    string
	TvgLogo    string
	GroupTitle string
	Duration   string
	Title      string
	URL        string
}

// ParseStats tracks parsing statistics
type ParseStats struct {
	ParsedEntries     int
	SkippedDuplicates int
	MalformedEntries  int
	TotalLines        int
	Duration          time.Duration
	ErrorsByType      map[string]int
}

// Parser handles M3U playlist parsing
type Parser struct {
	filePath   string
	logger     *logger.Logger
	seenHashes map[string]bool
	stats      ParseStats
}

// NewParser creates a new parser instance
func NewParser(filePath string) *Parser {
	return &Parser{
		filePath:   filePath,
		logger:     logger.AppLogger(),
		seenHashes: make(map[string]bool),
		stats: ParseStats{
			ErrorsByType: make(map[string]int),
		},
	}
}

// NewParserWithLogger creates a new parser instance with a custom logger
func NewParserWithLogger(filePath string, log *logger.Logger) *Parser {
	return &Parser{
		filePath:   filePath,
		logger:     log,
		seenHashes: make(map[string]bool),
		stats: ParseStats{
			ErrorsByType: make(map[string]int),
		},
	}
}

// Parse reads and parses an M3U playlist file
func (p *Parser) Parse() ([]models.ProcessedLine, error) {
	startTime := time.Now()

	p.logger.WithFields(map[string]interface{}{
		"file": p.filePath,
	}).Info("starting M3U playlist parsing")

	file, err := os.Open(p.filePath)
	if err != nil {
		return nil, apperrors.ParseError("failed to open playlist file", err)
	}
	defer file.Close()

	var lines []models.ProcessedLine
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	var currentEntry *M3UEntry
	hasHeader := false

	for scanner.Scan() {
		lineNumber++
		p.stats.TotalLines++
		line := strings.TrimSpace(scanner.Text())

		// Check for M3U header
		if lineNumber == 1 && strings.HasPrefix(line, "#EXTM3U") {
			hasHeader = true
			continue
		}

		// Skip empty lines
		if line == "" {
			continue
		}

		// Parse EXTINF line
		if strings.HasPrefix(line, "#EXTINF") {
			// If we have a pending entry without URL, it's malformed
			if currentEntry != nil && currentEntry.URL == "" {
				p.stats.MalformedEntries++
				p.stats.ErrorsByType["missing_url"]++
				p.logger.WithFields(map[string]interface{}{
					"line_number": lineNumber - 1,
					"tvg_name":    currentEntry.TvgName,
				}).Warn("EXTINF entry without URL")
			}

			currentEntry = p.parseExtinf(line, lineNumber)
			continue
		}

		// Skip other comments
		if strings.HasPrefix(line, "#") {
			continue
		}

		// This is a URL line
		if currentEntry != nil {
			currentEntry.URL = line

			// Create ProcessedLine from entry
			processedLine, err := p.createProcessedLine(currentEntry)
			if err != nil {
				p.stats.MalformedEntries++
				p.stats.ErrorsByType["invalid_entry"]++
				p.logger.WithFields(map[string]interface{}{
					"line_number": lineNumber,
					"error":       err,
				}).Warn("failed to create processed line")
				currentEntry = nil
				continue
			}

			// Check for duplicates
			if p.seenHashes[processedLine.LineHash] {
				p.stats.SkippedDuplicates++
				currentEntry = nil
				continue
			}

			p.seenHashes[processedLine.LineHash] = true
			lines = append(lines, *processedLine)
			p.stats.ParsedEntries++
			currentEntry = nil
		} else {
			// URL without EXTINF
			p.stats.MalformedEntries++
			p.stats.ErrorsByType["orphan_url"]++
			p.logger.WithFields(map[string]interface{}{
				"line_number": lineNumber,
			}).Warn("URL without EXTINF entry")
		}
	}

	// Check for final entry without URL
	if currentEntry != nil && currentEntry.URL == "" {
		p.stats.MalformedEntries++
		p.stats.ErrorsByType["missing_url"]++
	}

	if err := scanner.Err(); err != nil {
		return nil, apperrors.ParseError("error reading playlist file", err)
	}

	// Warn if missing header
	if !hasHeader {
		p.stats.ErrorsByType["missing_header"]++
		p.logger.Warn("M3U file missing #EXTM3U header")
	}

	p.stats.Duration = time.Since(startTime)

	p.logger.WithFields(map[string]interface{}{
		"total_lines":      p.stats.TotalLines,
		"parsed":           p.stats.ParsedEntries,
		"duplicates":       p.stats.SkippedDuplicates,
		"malformed":        p.stats.MalformedEntries,
		"duration_seconds": p.stats.Duration.Seconds(),
	}).Info("parsing complete")

	return lines, nil
}

// parseExtinf parses an EXTINF line and extracts metadata
func (p *Parser) parseExtinf(line string, lineNumber int) *M3UEntry {
	entry := &M3UEntry{}

	// Extract attributes using regex
	tvgIDRegex := regexp.MustCompile(`tvg-id="([^"]*)"`)
	tvgNameRegex := regexp.MustCompile(`tvg-name="([^"]*)"`)
	tvgLogoRegex := regexp.MustCompile(`tvg-logo="([^"]*)"`)
	groupTitleRegex := regexp.MustCompile(`group-title="([^"]*)"`)

	if matches := tvgIDRegex.FindStringSubmatch(line); len(matches) > 1 {
		entry.TvgID = matches[1]
	}
	if matches := tvgNameRegex.FindStringSubmatch(line); len(matches) > 1 {
		entry.TvgName = matches[1]
	}
	if matches := tvgLogoRegex.FindStringSubmatch(line); len(matches) > 1 {
		entry.TvgLogo = matches[1]
	}
	if matches := groupTitleRegex.FindStringSubmatch(line); len(matches) > 1 {
		entry.GroupTitle = matches[1]
	}

	// Extract title (text after last comma)
	if commaIdx := strings.LastIndex(line, ","); commaIdx != -1 {
		entry.Title = strings.TrimSpace(line[commaIdx+1:])
	}

	// If tvg-name is empty, use title as fallback
	if entry.TvgName == "" && entry.Title != "" {
		entry.TvgName = entry.Title
	}

	return entry
}

// createProcessedLine creates a ProcessedLine from an M3UEntry
func (p *Parser) createProcessedLine(entry *M3UEntry) (*models.ProcessedLine, error) {
	if entry == nil {
		return nil, fmt.Errorf("entry is nil")
	}
	if entry.TvgName == "" {
		return nil, fmt.Errorf("missing tvg-name")
	}
	if entry.URL == "" {
		return nil, fmt.Errorf("missing URL")
	}

	// Create line content (EXTINF + URL)
	lineContent := fmt.Sprintf("#EXTINF:-1 tvg-name=\"%s\" group-title=\"%s\",%s\n%s",
		entry.TvgName, entry.GroupTitle, entry.Title, entry.URL)

	// Calculate hash
	hash := p.calculateHash(entry.TvgName, entry.URL)

	return &models.ProcessedLine{
		LineContent: lineContent,
		LineURL:     &entry.URL,
		LineHash:    hash,
		TvgName:     entry.TvgName,
		GroupTitle:  entry.GroupTitle,
		State:       models.StatePending,
		ContentType: models.ContentTypeUncategorized,
	}, nil
}

// calculateHash generates a SHA-256 hash for a title and URL combination
func (p *Parser) calculateHash(tvgName, url string) string {
	content := tvgName + url
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// GetStats returns the current parsing statistics
func (p *Parser) GetStats() ParseStats {
	return p.stats
}
