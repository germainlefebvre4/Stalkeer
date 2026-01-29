package testing

import (
	"fmt"
	"testing"
	"time"

	"github.com/glefebvre/stalkeer/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestDB creates an in-memory SQLite database for testing
func TestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	// Run migrations
	if err := db.AutoMigrate(
		&models.Movie{},
		&models.TVShow{},
		&models.ProcessedLine{},
	); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	return db
}

// CleanupDB removes all records from test database tables
func CleanupDB(t *testing.T, db *gorm.DB) {
	t.Helper()

	db.Exec("DELETE FROM processed_lines")
	db.Exec("DELETE FROM movies")
	db.Exec("DELETE FROM tvshows")
}

// CreateProcessedLine creates a test processed line
func CreateProcessedLine(db *gorm.DB, overrides ...func(*models.ProcessedLine)) *models.ProcessedLine {
	lineURL := "http://example.com/stream"
	line := &models.ProcessedLine{
		LineContent: "#EXTINF:-1 tvg-name=\"Test Movie\" group-title=\"Movies\",Test Movie",
		LineURL:     &lineURL,
		LineHash:    fmt.Sprintf("hash_%d", time.Now().UnixNano()),
		TvgName:     "Test Movie",
		GroupTitle:  "Movies",
		ProcessedAt: time.Now(),
		ContentType: models.ContentTypeMovies,
		State:       models.StateProcessed,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	for _, override := range overrides {
		override(line)
	}

	db.Create(line)
	return line
}

// CreateMovie creates a test movie
func CreateMovie(db *gorm.DB, overrides ...func(*models.Movie)) *models.Movie {
	genres := "[\"Action\", \"Thriller\"]"
	duration := 120
	movie := &models.Movie{
		TMDBID:     12345,
		TMDBTitle:  "Test Movie",
		TMDBYear:   2024,
		TMDBGenres: &genres,
		Duration:   &duration,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	for _, override := range overrides {
		override(movie)
	}

	db.Create(movie)
	return movie
}

// CreateTVShow creates a test TV show
func CreateTVShow(db *gorm.DB, overrides ...func(*models.TVShow)) *models.TVShow {
	genres := "[\"Drama\", \"Comedy\"]"
	season := 1
	episode := 1
	tvshow := &models.TVShow{
		TMDBID:     67890,
		TMDBTitle:  "Test Show",
		TMDBYear:   2024,
		TMDBGenres: &genres,
		Season:     &season,
		Episode:    &episode,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	for _, override := range overrides {
		override(tvshow)
	}

	db.Create(tvshow)
	return tvshow
}

// AssertNoError fails the test if err is not nil
func AssertNoError(t *testing.T, err error, message string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", message, err)
	}
}

// AssertEqual fails the test if expected != actual
func AssertEqual[T comparable](t *testing.T, expected, actual T, message string) {
	t.Helper()
	if expected != actual {
		t.Fatalf("%s: expected %v, got %v", message, expected, actual)
	}
}

// AssertNotNil fails the test if value is nil
func AssertNotNil(t *testing.T, value interface{}, message string) {
	t.Helper()
	if value == nil {
		t.Fatalf("%s: expected non-nil value", message)
	}
}

// AssertCount verifies the count of records in a table
func AssertCount(t *testing.T, db *gorm.DB, model interface{}, expected int64, message string) {
	t.Helper()
	var count int64
	db.Model(model).Count(&count)
	if count != expected {
		t.Fatalf("%s: expected count %d, got %d", message, expected, count)
	}
}

// WithTVShow sets up a processed line as a TV show
func WithTVShow() func(*models.ProcessedLine) {
	return func(line *models.ProcessedLine) {
		line.ContentType = models.ContentTypeTVShows
	}
}

// WithGroupTitle sets the group title for a processed line
func WithGroupTitle(title string) func(*models.ProcessedLine) {
	return func(line *models.ProcessedLine) {
		line.GroupTitle = title
	}
}

// WithLineURL sets the line URL for a processed line
func WithLineURL(url string) func(*models.ProcessedLine) {
	return func(line *models.ProcessedLine) {
		line.LineURL = &url
	}
}

// WithState sets the state for a processed line
func WithState(state models.ProcessingState) func(*models.ProcessedLine) {
	return func(line *models.ProcessedLine) {
		line.State = state
	}
}

// WithMovieID sets the movie ID for a processed line
func WithMovieID(id uint) func(*models.ProcessedLine) {
	return func(line *models.ProcessedLine) {
		line.MovieID = &id
	}
}

// WithTVShowID sets the TV show ID for a processed line
func WithTVShowID(id uint) func(*models.ProcessedLine) {
	return func(line *models.ProcessedLine) {
		line.TVShowID = &id
	}
}

// WithTMDBID sets the TMDB ID for a movie
func WithTMDBID(id int) func(*models.Movie) {
	return func(movie *models.Movie) {
		movie.TMDBID = id
	}
}

// WithYear sets the year for a movie
func WithYear(year int) func(*models.Movie) {
	return func(movie *models.Movie) {
		movie.TMDBYear = year
	}
}

// WithSeasonEpisode sets season and episode for a TV show
func WithSeasonEpisode(season, episode int) func(*models.TVShow) {
	return func(tvshow *models.TVShow) {
		tvshow.Season = &season
		tvshow.Episode = &episode
	}
}

// TableTest represents a table-driven test case
type TableTest[T any] struct {
	Name     string
	Input    T
	Expected interface{}
	WantErr  bool
}

// RunTableTests executes table-driven tests
func RunTableTests[T any](t *testing.T, tests []TableTest[T], testFn func(t *testing.T, tc TableTest[T])) {
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			testFn(t, tc)
		})
	}
}

// Example usage documentation
func ExampleTestDB() {
	// In your test file:
	// func TestSomething(t *testing.T) {
	//     db := testing.TestDB(t)
	//     defer testing.CleanupDB(t, db)
	//
	//     line := testing.CreateProcessedLine(db, testing.WithTVShow())
	//     testing.AssertEqual(t, models.ContentTypeTVShows, line.ContentType, "content type mismatch")
	// }
	fmt.Println("See test files for usage examples")
}
