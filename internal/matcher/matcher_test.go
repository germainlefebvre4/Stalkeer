package matcher

import (
	"fmt"
	"testing"

	"github.com/glefebvre/stalkeer/internal/external/radarr"
	"github.com/glefebvre/stalkeer/internal/external/sonarr"
	"github.com/glefebvre/stalkeer/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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

func TestMatchMovieByTMDB(t *testing.T) {
	// Setup in-memory database
	db := setupTestDB(t)

	// Create test movies
	movies := []models.Movie{
		{
			TMDBID:    603,
			TMDBTitle: "The Matrix",
			TMDBYear:  1999,
		},
		{
			TMDBID:    27205,
			TMDBTitle: "Inception",
			TMDBYear:  2010,
		},
		{
			TMDBID:    155,
			TMDBTitle: "The Dark Knight",
			TMDBYear:  2008,
		},
	}

	for i := range movies {
		if err := db.Create(&movies[i]).Error; err != nil {
			t.Fatalf("failed to create test movie: %v", err)
		}

		// Create associated processed lines
		lineURL := "http://example.com/stream.mkv"
		processedLine := models.ProcessedLine{
			MovieID:     &movies[i].ID,
			TvgName:     movies[i].TMDBTitle,
			LineURL:     &lineURL,
			LineContent: "#EXTINF:-1," + movies[i].TMDBTitle,
			LineHash:    fmt.Sprintf("hash%d", i),
			GroupTitle:  "Movies",
			ContentType: models.ContentTypeMovies,
			State:       models.StateProcessed,
		}
		if err := db.Create(&processedLine).Error; err != nil {
			t.Fatalf("failed to create processed line: %v", err)
		}
	}

	tests := []struct {
		name          string
		tmdbID        int
		title         string
		year          int
		expectMatch   bool
		expectedTMDB  int
		minConfidence int
	}{
		{
			name:          "exact TMDB ID match",
			tmdbID:        603,
			title:         "The Matrix",
			year:          1999,
			expectMatch:   true,
			expectedTMDB:  603,
			minConfidence: 100,
		},
		{
			name:          "TMDB ID match with different title",
			tmdbID:        27205,
			title:         "Some Other Title",
			year:          2010,
			expectMatch:   true,
			expectedTMDB:  27205,
			minConfidence: 100,
		},
		{
			name:          "fuzzy title match when TMDB ID not found",
			tmdbID:        99999,
			title:         "The Dark Knight",
			year:          2008,
			expectMatch:   true,
			expectedTMDB:  155,
			minConfidence: 70,
		},
		{
			name:          "fuzzy title match with slightly different title",
			tmdbID:        99999,
			title:         "Dark Knight",
			year:          2008,
			expectMatch:   true,
			expectedTMDB:  155,
			minConfidence: 70,
		},
		{
			name:        "no match - TMDB ID and title both not found",
			tmdbID:      88888,
			title:       "Nonexistent Movie",
			year:        2025,
			expectMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			movie, processedLine, confidence, err := MatchMovieByTMDB(db, tt.tmdbID, tt.title, tt.year)

			if tt.expectMatch {
				if err != nil {
					t.Errorf("expected match, got error: %v", err)
					return
				}
				if movie == nil {
					t.Error("expected movie, got nil")
					return
				}
				if processedLine == nil {
					t.Error("expected processed line, got nil")
					return
				}
				if movie.TMDBID != tt.expectedTMDB {
					t.Errorf("expected TMDB ID %d, got %d", tt.expectedTMDB, movie.TMDBID)
				}
				if confidence < tt.minConfidence {
					t.Errorf("expected confidence >= %d, got %d", tt.minConfidence, confidence)
				}
			} else {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if movie != nil {
					t.Errorf("expected nil movie, got %+v", movie)
				}
			}
		})
	}
}

func TestMatchTVShowByTMDB(t *testing.T) {
	// Setup in-memory database
	db := setupTestDB(t)

	// Create test TV shows
	season1, episode1 := 1, 1
	season1b, episode2 := 1, 2
	season2, episode5 := 2, 5

	tvshows := []models.TVShow{
		{
			TMDBID:    1396,
			TMDBTitle: "Breaking Bad",
			Season:    &season1,
			Episode:   &episode1,
		},
		{
			TMDBID:    1396,
			TMDBTitle: "Breaking Bad",
			Season:    &season1b,
			Episode:   &episode2,
		},
		{
			TMDBID:    1399,
			TMDBTitle: "Game of Thrones",
			Season:    &season2,
			Episode:   &episode5,
		},
	}

	for i := range tvshows {
		if err := db.Create(&tvshows[i]).Error; err != nil {
			t.Fatalf("failed to create test tvshow: %v", err)
		}

		// Create associated processed lines
		lineURL := "http://example.com/stream.mkv"
		processedLine := models.ProcessedLine{
			TVShowID:    &tvshows[i].ID,
			TvgName:     tvshows[i].TMDBTitle,
			LineURL:     &lineURL,
			LineContent: "#EXTINF:-1," + tvshows[i].TMDBTitle,
			LineHash:    fmt.Sprintf("tvhash%d", i),
			GroupTitle:  "TV Shows",
			ContentType: models.ContentTypeTVShows,
			State:       models.StateProcessed,
		}
		if err := db.Create(&processedLine).Error; err != nil {
			t.Fatalf("failed to create processed line: %v", err)
		}
	}

	tests := []struct {
		name          string
		tmdbID        int
		title         string
		season        int
		episode       int
		expectMatch   bool
		expectedTMDB  int
		minConfidence int
	}{
		{
			name:          "exact TMDB ID + season + episode match",
			tmdbID:        1396,
			title:         "Breaking Bad",
			season:        1,
			episode:       1,
			expectMatch:   true,
			expectedTMDB:  1396,
			minConfidence: 100,
		},
		{
			name:          "TMDB ID + season + episode match with different title",
			tmdbID:        1396,
			title:         "Some Other Show",
			season:        1,
			episode:       2,
			expectMatch:   true,
			expectedTMDB:  1396,
			minConfidence: 100,
		},
		{
			name:          "fuzzy title match when TMDB ID not found",
			tmdbID:        99999,
			title:         "Game of Thrones",
			season:        2,
			episode:       5,
			expectMatch:   true,
			expectedTMDB:  1399,
			minConfidence: 70,
		},
		{
			name:          "fuzzy title match with season/episode",
			tmdbID:        99999,
			title:         "Breaking Bad",
			season:        1,
			episode:       1,
			expectMatch:   true,
			expectedTMDB:  1396,
			minConfidence: 70,
		},
		{
			name:        "no match - TMDB ID and title not found",
			tmdbID:      88888,
			title:       "Nonexistent Show",
			season:      1,
			episode:     1,
			expectMatch: false,
		},
		{
			name:        "no match - wrong season/episode",
			tmdbID:      1396,
			title:       "Breaking Bad",
			season:      10,
			episode:     10,
			expectMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tvshow, processedLine, confidence, err := MatchTVShowByTMDB(db, tt.tmdbID, tt.title, tt.season, tt.episode)

			if tt.expectMatch {
				if err != nil {
					t.Errorf("expected match, got error: %v", err)
					return
				}
				if tvshow == nil {
					t.Error("expected tvshow, got nil")
					return
				}
				if processedLine == nil {
					t.Error("expected processed line, got nil")
					return
				}
				if tvshow.TMDBID != tt.expectedTMDB {
					t.Errorf("expected TMDB ID %d, got %d", tt.expectedTMDB, tvshow.TMDBID)
				}
				if tvshow.Season != nil && tt.season > 0 && *tvshow.Season != tt.season {
					t.Errorf("expected season %d, got %d", tt.season, *tvshow.Season)
				}
				if tvshow.Episode != nil && tt.episode > 0 && *tvshow.Episode != tt.episode {
					t.Errorf("expected episode %d, got %d", tt.episode, *tvshow.Episode)
				}
				if confidence < tt.minConfidence {
					t.Errorf("expected confidence >= %d, got %d", tt.minConfidence, confidence)
				}
			} else {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if tvshow != nil {
					t.Errorf("expected nil tvshow, got %+v", tvshow)
				}
			}
		})
	}
}

func TestMatchTVShowByTVDB(t *testing.T) {
	// Setup in-memory database
	db := setupTestDB(t)

	season := 1
	episode := 16
	tvdbID := 73838

	tvshow := models.TVShow{
		TMDBID:    2004,
		TVDBID:    &tvdbID,
		TMDBTitle: "Malcolm",
		Season:    &season,
		Episode:   &episode,
	}

	if err := db.Create(&tvshow).Error; err != nil {
		t.Fatalf("failed to create test tvshow: %v", err)
	}

	lineURL := "http://example.com/malcolm-s01e16.mkv"
	processedLine := models.ProcessedLine{
		TVShowID:    &tvshow.ID,
		TvgName:     tvshow.TMDBTitle,
		LineURL:     &lineURL,
		LineContent: "#EXTINF:-1," + tvshow.TMDBTitle,
		LineHash:    "tvdb-match-hash",
		GroupTitle:  "TV Shows",
		ContentType: models.ContentTypeTVShows,
		State:       models.StateProcessed,
	}

	if err := db.Create(&processedLine).Error; err != nil {
		t.Fatalf("failed to create processed line: %v", err)
	}

	matchedShow, matchedLine, confidence, err := MatchTVShowByTVDB(db, tvdbID, 0, "Malcolm in the Middle", season, episode)
	if err != nil {
		t.Fatalf("expected TVDB match, got error: %v", err)
	}
	if matchedShow == nil {
		t.Fatal("expected tvshow, got nil")
	}
	if matchedLine == nil {
		t.Fatal("expected processed line, got nil")
	}
	if matchedShow.ID != tvshow.ID {
		t.Fatalf("expected tvshow ID %d, got %d", tvshow.ID, matchedShow.ID)
	}
	if matchedLine.ID != processedLine.ID {
		t.Fatalf("expected processed line ID %d, got %d", processedLine.ID, matchedLine.ID)
	}
	if confidence != 100 {
		t.Fatalf("expected confidence 100, got %d", confidence)
	}
}

func TestFindMovieDownloadCandidates(t *testing.T) {
	db := setupTestDB(t)

	movie := models.Movie{TMDBID: 603, TMDBTitle: "The Matrix", TMDBYear: 1999}
	if err := db.Create(&movie).Error; err != nil {
		t.Fatalf("failed to create movie: %v", err)
	}

	res720p := "720p"
	res1080p := "1080p"
	res4K := "4K"

	lineURL := "http://example.com/stream.mkv"
	lines := []models.ProcessedLine{
		{
			MovieID: &movie.ID, TvgName: "The Matrix 4K", LineURL: &lineURL,
			LineContent: "#EXTINF", LineHash: "hash-4k", GroupTitle: "Movies",
			ContentType: models.ContentTypeMovies, State: models.StateProcessed,
			Resolution: &res4K,
		},
		{
			MovieID: &movie.ID, TvgName: "The Matrix 720p", LineURL: &lineURL,
			LineContent: "#EXTINF", LineHash: "hash-720p", GroupTitle: "Movies",
			ContentType: models.ContentTypeMovies, State: models.StateProcessed,
			Resolution: &res720p,
		},
		{
			MovieID: &movie.ID, TvgName: "The Matrix 1080p", LineURL: &lineURL,
			LineContent: "#EXTINF", LineHash: "hash-1080p", GroupTitle: "Movies",
			ContentType: models.ContentTypeMovies, State: models.StateProcessed,
			Resolution: &res1080p,
		},
	}

	for i := range lines {
		if err := db.Create(&lines[i]).Error; err != nil {
			t.Fatalf("failed to create processed line: %v", err)
		}
	}

	candidates, err := FindMovieDownloadCandidates(db, movie.ID)
	if err != nil {
		t.Fatalf("FindMovieDownloadCandidates returned error: %v", err)
	}

	if len(candidates) != 3 {
		t.Fatalf("expected 3 candidates, got %d", len(candidates))
	}

	// First must be 720p
	if candidates[0].Resolution == nil || *candidates[0].Resolution != "720p" {
		t.Errorf("expected first candidate resolution '720p', got %v", candidates[0].Resolution)
	}
	// Second must be 1080p
	if candidates[1].Resolution == nil || *candidates[1].Resolution != "1080p" {
		t.Errorf("expected second candidate resolution '1080p', got %v", candidates[1].Resolution)
	}
	// Third must be 4K
	if candidates[2].Resolution == nil || *candidates[2].Resolution != "4K" {
		t.Errorf("expected third candidate resolution '4K', got %v", candidates[2].Resolution)
	}
}

func TestFindMovieDownloadCandidatesNilResolutionLast(t *testing.T) {
	db := setupTestDB(t)

	movie := models.Movie{TMDBID: 27205, TMDBTitle: "Inception", TMDBYear: 2010}
	if err := db.Create(&movie).Error; err != nil {
		t.Fatalf("failed to create movie: %v", err)
	}

	res1080p := "1080p"
	lineURL := "http://example.com/stream.mkv"
	lines := []models.ProcessedLine{
		{
			MovieID: &movie.ID, TvgName: "Inception", LineURL: &lineURL,
			LineContent: "#EXTINF", LineHash: "hash-nil", GroupTitle: "Movies",
			ContentType: models.ContentTypeMovies, State: models.StateProcessed,
			Resolution: nil,
		},
		{
			MovieID: &movie.ID, TvgName: "Inception 1080p", LineURL: &lineURL,
			LineContent: "#EXTINF", LineHash: "hash-1080p-i", GroupTitle: "Movies",
			ContentType: models.ContentTypeMovies, State: models.StateProcessed,
			Resolution: &res1080p,
		},
	}

	for i := range lines {
		if err := db.Create(&lines[i]).Error; err != nil {
			t.Fatalf("failed to create processed line: %v", err)
		}
	}

	candidates, err := FindMovieDownloadCandidates(db, movie.ID)
	if err != nil {
		t.Fatalf("FindMovieDownloadCandidates returned error: %v", err)
	}

	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}

	// 1080p must come before nil
	if candidates[0].Resolution == nil || *candidates[0].Resolution != "1080p" {
		t.Errorf("expected first candidate '1080p', got %v", candidates[0].Resolution)
	}
	if candidates[1].Resolution != nil {
		t.Errorf("expected second candidate nil resolution, got '%s'", *candidates[1].Resolution)
	}
}

func TestFindMovieDownloadCandidatesExcludesDownloaded(t *testing.T) {
	db := setupTestDB(t)

	movie := models.Movie{TMDBID: 155, TMDBTitle: "The Dark Knight", TMDBYear: 2008}
	if err := db.Create(&movie).Error; err != nil {
		t.Fatalf("failed to create movie: %v", err)
	}

	res720p := "720p"
	lineURL := "http://example.com/stream.mkv"
	lines := []models.ProcessedLine{
		{
			MovieID: &movie.ID, TvgName: "The Dark Knight 720p", LineURL: &lineURL,
			LineContent: "#EXTINF", LineHash: "hash-dk-720p", GroupTitle: "Movies",
			ContentType: models.ContentTypeMovies, State: models.StateDownloaded,
			Resolution: &res720p,
		},
	}

	if err := db.Create(&lines[0]).Error; err != nil {
		t.Fatalf("failed to create processed line: %v", err)
	}

	candidates, err := FindMovieDownloadCandidates(db, movie.ID)
	if err != nil {
		t.Fatalf("FindMovieDownloadCandidates returned error: %v", err)
	}

	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates (downloaded excluded), got %d", len(candidates))
	}
}

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Auto-migrate all models
	if err := db.AutoMigrate(
		&models.ProcessedLine{},
		&models.Movie{},
		&models.TVShow{},
		&models.DownloadInfo{},
	); err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}

	return db
}
