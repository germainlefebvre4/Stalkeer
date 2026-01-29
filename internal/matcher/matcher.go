package matcher

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/glefebvre/stalkeer/internal/external/radarr"
	"github.com/glefebvre/stalkeer/internal/external/sonarr"
	"github.com/glefebvre/stalkeer/internal/models"
)

// Config holds matcher configuration
type Config struct {
	MinConfidence float64
}

// DefaultConfig returns sensible defaults for matcher
func DefaultConfig() Config {
	return Config{
		MinConfidence: 0.8,
	}
}

// Match represents a match between a processed line and external content
type Match struct {
	ProcessedLine *models.ProcessedLine
	MovieID       *int
	SeriesID      *int
	EpisodeID     *int
	Confidence    float64
	MatchType     string // "exact", "fuzzy", "manual"
}

// Matcher handles matching between processed lines and external services
type Matcher struct {
	cfg Config
}

// New creates a new matcher
func New(cfg Config) *Matcher {
	return &Matcher{cfg: cfg}
}

// MatchMovie attempts to match a processed line with a Radarr movie
func (m *Matcher) MatchMovie(line *models.ProcessedLine, movie *radarr.Movie) *Match {
	if movie == nil || line == nil {
		return nil
	}

	// Calculate title similarity
	titleScore := m.calculateStringSimilarity(
		m.normalizeTitle(line.TvgName),
		m.normalizeTitle(movie.Title),
	)

	// Calculate year match (if available)
	yearScore := 0.0
	if line.Movie != nil && line.Movie.TMDBYear > 0 && movie.Year > 0 {
		if line.Movie.TMDBYear == movie.Year {
			yearScore = 1.0
		} else if abs(line.Movie.TMDBYear-movie.Year) <= 1 {
			yearScore = 0.5
		}
	}

	// Overall confidence is weighted average
	confidence := (titleScore * 0.7) + (yearScore * 0.3)

	if confidence < m.cfg.MinConfidence {
		return nil
	}

	matchType := "fuzzy"
	if titleScore >= 0.95 && yearScore >= 0.9 {
		matchType = "exact"
	}

	return &Match{
		ProcessedLine: line,
		MovieID:       &movie.ID,
		Confidence:    confidence,
		MatchType:     matchType,
	}
}

// MatchEpisode attempts to match a processed line with a Sonarr episode
func (m *Matcher) MatchEpisode(line *models.ProcessedLine, series *sonarr.Series, episode *sonarr.Episode) *Match {
	if series == nil || episode == nil || line == nil {
		return nil
	}

	// Calculate title similarity
	titleScore := m.calculateStringSimilarity(
		m.normalizeTitle(line.TvgName),
		m.normalizeTitle(series.Title),
	)

	// Calculate season/episode match
	seasonEpisodeScore := 0.0
	if line.TVShow != nil && line.TVShow.Season != nil && line.TVShow.Episode != nil {
		if *line.TVShow.Season == episode.SeasonNumber && *line.TVShow.Episode == episode.EpisodeNumber {
			seasonEpisodeScore = 1.0
		}
	}

	// Overall confidence
	confidence := (titleScore * 0.5) + (seasonEpisodeScore * 0.5)

	if confidence < m.cfg.MinConfidence {
		return nil
	}

	matchType := "fuzzy"
	if titleScore >= 0.95 && seasonEpisodeScore >= 0.9 {
		matchType = "exact"
	}

	return &Match{
		ProcessedLine: line,
		SeriesID:      &series.ID,
		EpisodeID:     &episode.ID,
		Confidence:    confidence,
		MatchType:     matchType,
	}
}

// FindBestMovieMatch finds the best matching movie from a list
func (m *Matcher) FindBestMovieMatch(line *models.ProcessedLine, movies []radarr.Movie) *Match {
	var bestMatch *Match

	for i := range movies {
		match := m.MatchMovie(line, &movies[i])
		if match != nil {
			if bestMatch == nil || match.Confidence > bestMatch.Confidence {
				bestMatch = match
			}
		}
	}

	return bestMatch
}

// normalizeTitle normalizes a title for comparison
func (m *Matcher) normalizeTitle(title string) string {
	// Convert to lowercase
	title = strings.ToLower(title)

	// Remove common patterns
	patterns := []string{
		`\(\d{4}\)`, // (2020)
		`\[\d{4}\]`, // [2020]
		`\d{4}`,     // 2020
		`s\d+e\d+`,  // S01E01
		`\b(720p|1080p|2160p|4k|hd|uhd|bluray|web-dl|webrip|hdtv)\b`,
		`\b(x264|x265|h264|h265|hevc)\b`,
		`\b(aac|ac3|dts|mp3)\b`,
		`\[.*?\]`, // [any brackets]
		`\(.*?\)`, // (any parens)
		`[_\-\.]`, // underscores, dashes, dots
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		title = re.ReplaceAllString(title, " ")
	}

	// Remove extra whitespace
	title = strings.Join(strings.Fields(title), " ")

	// Remove punctuation
	title = strings.Map(func(r rune) rune {
		if unicode.IsPunct(r) {
			return -1
		}
		return r
	}, title)

	return strings.TrimSpace(title)
}

// calculateStringSimilarity calculates similarity between two strings using Levenshtein distance
func (m *Matcher) calculateStringSimilarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}

	// Quick checks
	if len(s1) == 0 || len(s2) == 0 {
		return 0.0
	}

	// Calculate Levenshtein distance
	distance := levenshteinDistance(s1, s2)

	// Convert to similarity score
	maxLen := max(len(s1), len(s2))
	similarity := 1.0 - float64(distance)/float64(maxLen)

	return similarity
}

// levenshteinDistance calculates the Levenshtein distance between two strings
func levenshteinDistance(s1, s2 string) int {
	len1 := len(s1)
	len2 := len(s2)

	// Create matrix
	matrix := make([][]int, len1+1)
	for i := range matrix {
		matrix[i] = make([]int, len2+1)
		matrix[i][0] = i
	}
	for j := 0; j <= len2; j++ {
		matrix[0][j] = j
	}

	// Fill matrix
	for i := 1; i <= len1; i++ {
		for j := 1; j <= len2; j++ {
			cost := 1
			if s1[i-1] == s2[j-1] {
				cost = 0
			}

			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len1][len2]
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
