package classifier

import (
	"regexp"
	"strconv"
	"strings"
)

// ContentType represents the type of content
type ContentType string

const (
	ContentTypeMovie         ContentType = "movie"
	ContentTypeSeries        ContentType = "series"
	ContentTypeUncategorized ContentType = "uncategorized"
)

// Classification represents the result of classifying a title
type Classification struct {
	ContentType ContentType
	Season      *int
	Episode     *int
	Resolution  *string
	Confidence  int // 0-100
}

// Classifier provides content classification functionality
type Classifier struct {
	seasonEpisodePatterns []*regexp.Regexp
	resolutionPatterns    []*regexp.Regexp
	yearPattern           *regexp.Regexp
}

// New creates a new Classifier with precompiled regex patterns
func New() *Classifier {
	return &Classifier{
		seasonEpisodePatterns: compileSeasonEpisodePatterns(),
		resolutionPatterns:    compileResolutionPatterns(),
		yearPattern:           regexp.MustCompile(`\((\d{4})\)`),
	}
}

// Classify analyzes a title and returns classification information
func (c *Classifier) Classify(title string, groupTitle string) Classification {
	classification := Classification{
		ContentType: ContentTypeUncategorized,
		Confidence:  0,
	}

	// Extract season and episode
	season, episode := c.ExtractSeasonEpisode(title)
	classification.Season = season
	classification.Episode = episode

	// Extract resolution
	classification.Resolution = c.ExtractResolution(title)

	// Determine content type and confidence
	classification.ContentType, classification.Confidence = c.determineContentType(title, groupTitle, season, episode)

	return classification
}

// ExtractSeasonEpisode attempts to extract season and episode numbers from a title
func (c *Classifier) ExtractSeasonEpisode(title string) (*int, *int) {
	for _, pattern := range c.seasonEpisodePatterns {
		matches := pattern.FindStringSubmatch(title)
		if len(matches) >= 3 {
			season, err := strconv.Atoi(matches[1])
			if err != nil {
				continue
			}
			episode, err := strconv.Atoi(matches[2])
			if err != nil {
				continue
			}
			return &season, &episode
		}
	}
	return nil, nil
}

// ExtractResolution attempts to extract resolution information from a title
func (c *Classifier) ExtractResolution(title string) *string {
	titleLower := strings.ToLower(title)

	// Check for 4K/UHD
	if strings.Contains(titleLower, "4k") || strings.Contains(titleLower, "uhd") || strings.Contains(titleLower, "2160p") {
		res := "4K"
		return &res
	}

	// Check for 1080p
	if strings.Contains(titleLower, "1080p") || strings.Contains(titleLower, "fullhd") || strings.Contains(titleLower, "fhd") {
		res := "1080p"
		return &res
	}

	// Check for 720p
	if strings.Contains(titleLower, "720p") || strings.Contains(titleLower, "hd") {
		res := "720p"
		return &res
	}

	// Check for 480p/SD
	if strings.Contains(titleLower, "480p") || strings.Contains(titleLower, "sd") {
		res := "480p"
		return &res
	}

	return nil
}

// determineContentType determines if the content is a movie or series
func (c *Classifier) determineContentType(title string, groupTitle string, season *int, episode *int) (ContentType, int) {
	titleLower := strings.ToLower(title)
	groupTitleLower := strings.ToLower(groupTitle)
	confidence := 0

	// Check group-title first for strong indicators
	// Series group titles typically start with "Séries" or "Series"
	if strings.HasPrefix(groupTitleLower, "séries") || strings.HasPrefix(groupTitleLower, "series") {
		confidence += 70
		return ContentTypeSeries, min(confidence, 100)
	}

	// Movies group titles typically start with patterns like "FR: FILMS", "ES: FILMS", etc.
	// where the country code is 2-3 letters followed by ": FILMS" or "FILMS"
	if strings.Contains(groupTitleLower, "films") || strings.Contains(groupTitleLower, "movies") {
		confidence += 70
		return ContentTypeMovie, min(confidence, 100)
	}

	// Strong indicators for series from season/episode
	if season != nil && episode != nil {
		confidence += 80
		return ContentTypeSeries, min(confidence, 100)
	}

	// Keywords indicating series in title
	seriesKeywords := []string{"season", "episode", "series", "saison", "episodio", "staffel", "folge"}
	for _, keyword := range seriesKeywords {
		if strings.Contains(titleLower, keyword) {
			confidence += 40
			return ContentTypeSeries, min(confidence, 100)
		}
	}

	// Year in parentheses is typical for movies
	if c.yearPattern.MatchString(title) {
		confidence += 60
	}

	// If we found a year but no season/episode, likely a movie
	if confidence >= 50 && season == nil && episode == nil {
		return ContentTypeMovie, confidence
	}

	// If we have series indicators, classify as series
	if confidence > 0 && season == nil && episode == nil {
		return ContentTypeSeries, confidence
	}

	// Default: check for movie indicators
	movieKeywords := []string{"film", "movie", "cinema"}
	for _, keyword := range movieKeywords {
		if strings.Contains(titleLower, keyword) {
			confidence += 50
			return ContentTypeMovie, min(confidence, 100)
		}
	}

	// Not enough information
	if confidence < 30 {
		return ContentTypeUncategorized, confidence
	}

	// Default to movie if we have some confidence but no series indicators
	if season == nil && episode == nil {
		return ContentTypeMovie, max(confidence, 40)
	}

	return ContentTypeUncategorized, confidence
}

// compileSeasonEpisodePatterns returns all precompiled season/episode regex patterns
func compileSeasonEpisodePatterns() []*regexp.Regexp {
	patterns := []string{
		// Standard: S01E05, S1E5
		`[Ss](\d{1,2})[Ee](\d{1,3})`,
		// Dash: S01-E05, S1-E5
		`[Ss](\d{1,2})[-][Ee](\d{1,3})`,
		// Space: S01 E05, S1 E5
		`[Ss](\d{1,2})\s+[Ee](\d{1,3})`,
		// Alternative: 1x05, 01x05
		`(\d{1,2})[xX](\d{1,3})`,
		// Words: Season 1 Episode 5, Season 01 Episode 05
		`[Ss]eason\s*(\d{1,2})\s*[Ee]pisode\s*(\d{1,3})`,
		// French: Saison 1 Episode 5
		`[Ss]aison\s*(\d{1,2})\s*[EeÉé]pisode\s*(\d{1,3})`,
		// Spanish: Temporada 1 Episodio 5
		`[Tt]emporada\s*(\d{1,2})\s*[Ee]pisodio\s*(\d{1,3})`,
		// German: Staffel 1 Folge 5
		`[Ss]taffel\s*(\d{1,2})\s*[Ff]olge\s*(\d{1,3})`,
		// Compact: s1e5
		`s(\d{1,2})e(\d{1,3})`,
	}

	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		compiled = append(compiled, regexp.MustCompile(pattern))
	}
	return compiled
}

// compileResolutionPatterns returns precompiled resolution regex patterns
func compileResolutionPatterns() []*regexp.Regexp {
	patterns := []string{
		`\b(4K|UHD|2160p)\b`,
		`\b(1080p|FullHD|FHD)\b`,
		`\b(720p|HD)\b`,
		`\b(480p|SD)\b`,
	}

	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		compiled = append(compiled, regexp.MustCompile(`(?i)`+pattern))
	}
	return compiled
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
