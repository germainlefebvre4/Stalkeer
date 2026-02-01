package matcher

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/glefebvre/stalkeer/internal/external/radarr"
	"github.com/glefebvre/stalkeer/internal/external/sonarr"
	"github.com/glefebvre/stalkeer/internal/models"
	"gorm.io/gorm"
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

// MatchMovieByTMDB finds a movie in the database by TMDB ID with fallback to title/year matching
// Returns (movie, processedLine, confidence, error)
func MatchMovieByTMDB(db *gorm.DB, tmdbID int, title string, year int) (*models.Movie, *models.ProcessedLine, int, error) {
	// Primary match: exact TMDB ID
	var movie models.Movie
	err := db.Where("tmdb_id = ?", tmdbID).First(&movie).Error
	if err == nil {
		// Found exact TMDB match, get processed line
		var processedLine models.ProcessedLine
		err = db.Where("movie_id = ?", movie.ID).
			Where("state IN ?", []string{string(models.StateProcessed), string(models.StateFailed)}).
			Order("created_at DESC").
			First(&processedLine).Error
		if err != nil {
			return nil, nil, 0, err
		}
		return &movie, &processedLine, 100, nil
	}

	// Fallback: title and year fuzzy matching
	if title == "" || year == 0 {
		return nil, nil, 0, gorm.ErrRecordNotFound
	}

	var movies []models.Movie
	err = db.Where("tmdb_year BETWEEN ? AND ?", year-1, year+1).Find(&movies).Error
	if err != nil {
		return nil, nil, 0, err
	}

	matcher := New(DefaultConfig())
	var bestMovie *models.Movie
	var bestScore float64

	normalizedSearchTitle := matcher.normalizeTitle(title)

	for i := range movies {
		normalizedMovieTitle := matcher.normalizeTitle(movies[i].TMDBTitle)
		score := matcher.calculateStringSimilarity(normalizedSearchTitle, normalizedMovieTitle)

		// Boost score if years match exactly
		if movies[i].TMDBYear == year {
			score = score*0.8 + 0.2
		}

		if score > bestScore && score >= 0.7 {
			bestScore = score
			bestMovie = &movies[i]
		}
	}

	if bestMovie == nil {
		return nil, nil, 0, gorm.ErrRecordNotFound
	}

	// Get processed line for the best match
	var processedLine models.ProcessedLine
	err = db.Where("movie_id = ?", bestMovie.ID).
		Where("state IN ?", []string{string(models.StateProcessed), string(models.StateFailed)}).
		Order("created_at DESC").
		First(&processedLine).Error
	if err != nil {
		return nil, nil, 0, err
	}

	confidence := int(bestScore * 100)
	return bestMovie, &processedLine, confidence, nil
}

// MatchTVShowByTMDB finds a TV show episode in the database by TMDB ID, season, and episode
// Returns (tvshow, processedLine, confidence, error)
func MatchTVShowByTMDB(db *gorm.DB, tmdbID int, title string, season, episode int) (*models.TVShow, *models.ProcessedLine, int, error) {
	// Primary match: exact TMDB ID + season + episode
	var tvshow models.TVShow
	query := db.Where("tmdb_id = ?", tmdbID)
	if season > 0 {
		query = query.Where("season = ?", season)
	}
	if episode > 0 {
		query = query.Where("episode = ?", episode)
	}

	err := query.First(&tvshow).Error
	if err == nil {
		// Found exact match, get processed line
		var processedLine models.ProcessedLine
		err = db.Where("tvshow_id = ?", tvshow.ID).
			Where("state IN ?", []string{string(models.StateProcessed), string(models.StateFailed)}).
			Order("created_at DESC").
			First(&processedLine).Error
		if err != nil {
			return nil, nil, 0, err
		}
		return &tvshow, &processedLine, 100, nil
	}

	// Fallback: title fuzzy matching with season/episode
	if title == "" {
		return nil, nil, 0, gorm.ErrRecordNotFound
	}

	var tvshows []models.TVShow
	query = db.Model(&models.TVShow{})
	if season > 0 {
		query = query.Where("season = ?", season)
	}
	if episode > 0 {
		query = query.Where("episode = ?", episode)
	}
	err = query.Find(&tvshows).Error
	if err != nil {
		return nil, nil, 0, err
	}

	matcher := New(DefaultConfig())
	var bestShow *models.TVShow
	var bestScore float64

	normalizedSearchTitle := matcher.normalizeTitle(title)

	for i := range tvshows {
		normalizedShowTitle := matcher.normalizeTitle(tvshows[i].TMDBTitle)
		score := matcher.calculateStringSimilarity(normalizedSearchTitle, normalizedShowTitle)

		// Boost score if season/episode match
		if tvshows[i].Season != nil && season > 0 && *tvshows[i].Season == season {
			score = score*0.7 + 0.15
		}
		if tvshows[i].Episode != nil && episode > 0 && *tvshows[i].Episode == episode {
			score = score*0.7 + 0.15
		}

		if score > bestScore && score >= 0.7 {
			bestScore = score
			bestShow = &tvshows[i]
		}
	}

	if bestShow == nil {
		return nil, nil, 0, gorm.ErrRecordNotFound
	}

	// Get processed line for the best match
	var processedLine models.ProcessedLine
	err = db.Where("tvshow_id = ?", bestShow.ID).
		Where("state IN ?", []string{string(models.StateProcessed), string(models.StateFailed)}).
		Order("created_at DESC").
		First(&processedLine).Error
	if err != nil {
		return nil, nil, 0, err
	}

	confidence := int(bestScore * 100)
	return bestShow, &processedLine, confidence, nil
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
