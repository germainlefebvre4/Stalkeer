package matcher

import (
	"testing"

	"github.com/glefebvre/stalkeer/internal/external/radarr"
	"github.com/glefebvre/stalkeer/internal/external/sonarr"
	"github.com/glefebvre/stalkeer/internal/models"
)

func TestNormalizeTitle(t *testing.T) {
	m := New(DefaultConfig())

	tests := []struct {
		input    string
		expected string
	}{
		{"The Matrix (1999)", "the matrix"},
		{"Inception [2010] 1080p", "inception"},
		{"Breaking.Bad.S01E01.720p.BluRay", "breaking bad"},
		{"The_Walking_Dead_2010", "the walking dead"},
		{"Game of Thrones - S08E06 - 4K", "game of thrones"},
		{"Avengers: Endgame", "avengers endgame"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := m.normalizeTitle(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeTitle(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCalculateStringSimilarity(t *testing.T) {
	m := New(DefaultConfig())

	tests := []struct {
		s1       string
		s2       string
		minScore float64
	}{
		{"identical", "identical", 1.0},
		{"similar", "similiar", 0.8},
		{"test", "best", 0.7},
		{"", "", 1.0},
		{"hello", "", 0.0},
		{"", "world", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.s1+"_vs_"+tt.s2, func(t *testing.T) {
			result := m.calculateStringSimilarity(tt.s1, tt.s2)
			if result < tt.minScore {
				t.Errorf("calculateStringSimilarity(%q, %q) = %f, want >= %f",
					tt.s1, tt.s2, result, tt.minScore)
			}
		})
	}
}

func TestMatchMovie(t *testing.T) {
	m := New(DefaultConfig())

	tests := []struct {
		name          string
		line          *models.ProcessedLine
		movie         *radarr.Movie
		expectMatch   bool
		minConfidence float64
	}{
		{
			name: "exact match with year",
			line: &models.ProcessedLine{
				TvgName: "The Matrix",
				Movie: &models.Movie{
					TMDBYear: 1999,
				},
			},
			movie: &radarr.Movie{
				ID:     1,
				Title:  "The Matrix",
				Year:   1999,
				TMDBID: 603,
			},
			expectMatch:   true,
			minConfidence: 0.95,
		},
		{
			name: "title match with year off by 1",
			line: &models.ProcessedLine{
				TvgName: "Inception",
				Movie: &models.Movie{
					TMDBYear: 2010,
				},
			},
			movie: &radarr.Movie{
				ID:     2,
				Title:  "Inception",
				Year:   2011,
				TMDBID: 27205,
			},
			expectMatch:   true,
			minConfidence: 0.8,
		},
		{
			name: "fuzzy title match",
			line: &models.ProcessedLine{
				TvgName: "The Dark Knight",
				Movie: &models.Movie{
					TMDBYear: 2008,
				},
			},
			movie: &radarr.Movie{
				ID:     3,
				Title:  "Dark Knight",
				Year:   2008,
				TMDBID: 155,
			},
			expectMatch:   true,
			minConfidence: 0.8,
		},
		{
			name: "no match - different titles",
			line: &models.ProcessedLine{
				TvgName: "The Matrix",
				Movie: &models.Movie{
					TMDBYear: 1999,
				},
			},
			movie: &radarr.Movie{
				ID:     4,
				Title:  "Inception",
				Year:   2010,
				TMDBID: 27205,
			},
			expectMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.MatchMovie(tt.line, tt.movie)

			if tt.expectMatch {
				if result == nil {
					t.Error("expected a match, got nil")
					return
				}
				if result.Confidence < tt.minConfidence {
					t.Errorf("confidence = %f, want >= %f", result.Confidence, tt.minConfidence)
				}
				if result.MovieID == nil || *result.MovieID != tt.movie.ID {
					t.Errorf("movieID = %v, want %d", result.MovieID, tt.movie.ID)
				}
			} else {
				if result != nil {
					t.Errorf("expected no match, got confidence %f", result.Confidence)
				}
			}
		})
	}
}

func TestMatchEpisode(t *testing.T) {
	m := New(DefaultConfig())

	season1 := 1
	episode1 := 1
	season2 := 2
	episode5 := 5

	tests := []struct {
		name          string
		line          *models.ProcessedLine
		series        *sonarr.Series
		episode       *sonarr.Episode
		expectMatch   bool
		minConfidence float64
	}{
		{
			name: "exact match with season and episode",
			line: &models.ProcessedLine{
				TvgName: "Breaking Bad",
				TVShow: &models.TVShow{
					Season:  &season1,
					Episode: &episode1,
				},
			},
			series: &sonarr.Series{
				ID:     1,
				Title:  "Breaking Bad",
				TvdbID: 81189,
			},
			episode: &sonarr.Episode{
				ID:            1,
				SeriesID:      1,
				SeasonNumber:  1,
				EpisodeNumber: 1,
			},
			expectMatch:   true,
			minConfidence: 0.95,
		},
		{
			name: "title match but wrong episode",
			line: &models.ProcessedLine{
				TvgName: "Game of Thrones",
				TVShow: &models.TVShow{
					Season:  &season1,
					Episode: &episode1,
				},
			},
			series: &sonarr.Series{
				ID:     2,
				Title:  "Game of Thrones",
				TvdbID: 121361,
			},
			episode: &sonarr.Episode{
				ID:            2,
				SeriesID:      2,
				SeasonNumber:  1,
				EpisodeNumber: 2,
			},
			expectMatch: false,
		},
		{
			name: "fuzzy title match with episode",
			line: &models.ProcessedLine{
				TvgName: "The Walking Dead",
				TVShow: &models.TVShow{
					Season:  &season2,
					Episode: &episode5,
				},
			},
			series: &sonarr.Series{
				ID:     3,
				Title:  "Walking Dead",
				TvdbID: 153021,
			},
			episode: &sonarr.Episode{
				ID:            3,
				SeriesID:      3,
				SeasonNumber:  2,
				EpisodeNumber: 5,
			},
			expectMatch:   true,
			minConfidence: 0.8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.MatchEpisode(tt.line, tt.series, tt.episode)

			if tt.expectMatch {
				if result == nil {
					t.Error("expected a match, got nil")
					return
				}
				if result.Confidence < tt.minConfidence {
					t.Errorf("confidence = %f, want >= %f", result.Confidence, tt.minConfidence)
				}
				if result.SeriesID == nil || *result.SeriesID != tt.series.ID {
					t.Errorf("seriesID = %v, want %d", result.SeriesID, tt.series.ID)
				}
				if result.EpisodeID == nil || *result.EpisodeID != tt.episode.ID {
					t.Errorf("episodeID = %v, want %d", result.EpisodeID, tt.episode.ID)
				}
			} else {
				if result != nil {
					t.Errorf("expected no match, got confidence %f", result.Confidence)
				}
			}
		})
	}
}

func TestFindBestMovieMatch(t *testing.T) {
	m := New(DefaultConfig())

	line := &models.ProcessedLine{
		TvgName: "The Matrix",
		Movie: &models.Movie{
			TMDBYear: 1999,
		},
	}

	movies := []radarr.Movie{
		{ID: 1, Title: "Matrix Revolutions", Year: 2003, TMDBID: 605},
		{ID: 2, Title: "The Matrix", Year: 1999, TMDBID: 603},
		{ID: 3, Title: "Matrix Reloaded", Year: 2003, TMDBID: 604},
	}

	result := m.FindBestMovieMatch(line, movies)

	if result == nil {
		t.Fatal("expected a match, got nil")
	}

	if result.MovieID == nil || *result.MovieID != 2 {
		t.Errorf("expected movie ID 2, got %v", result.MovieID)
	}

	if result.Confidence < 0.95 {
		t.Errorf("expected high confidence, got %f", result.Confidence)
	}
}

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		s1       string
		s2       string
		expected int
	}{
		{"", "", 0},
		{"a", "", 1},
		{"", "a", 1},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"abc", "def", 3},
		{"kitten", "sitting", 3},
	}

	for _, tt := range tests {
		t.Run(tt.s1+"_"+tt.s2, func(t *testing.T) {
			result := levenshteinDistance(tt.s1, tt.s2)
			if result != tt.expected {
				t.Errorf("levenshteinDistance(%q, %q) = %d, want %d",
					tt.s1, tt.s2, result, tt.expected)
			}
		})
	}
}
