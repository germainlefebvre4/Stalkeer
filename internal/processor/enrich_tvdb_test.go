package processor

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/glefebvre/stalkeer/internal/database"
	"github.com/glefebvre/stalkeer/internal/external/tmdb"
	"github.com/glefebvre/stalkeer/internal/models"
)

// setupEnrichTestDB initialises a test database and returns a cleanup function.
func setupEnrichTestDB(t *testing.T) {
	t.Helper()
	setupTestDB(t)
}

// newTMDBTestServer creates an httptest server that responds to TMDB External ID
// requests. The handler is provided by the caller to control per-test behaviour.
func newTMDBTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(func() { srv.Close() })
	return srv
}

func newTMDBClientForTest(t *testing.T, serverURL string) *tmdb.Client {
	t.Helper()
	tmdb.SetBaseURL(serverURL)
	return tmdb.NewClient(tmdb.Config{
		APIKey:            "test-key",
		Language:          "en-US",
		RequestsPerSecond: 0,
	})
}

// TestEnrichMissingTVDBIDs_MovieUpdated verifies that a Movie with a missing tvdb_id
// is updated when TMDB returns a valid TVDB ID.
func TestEnrichMissingTVDBIDs_MovieUpdated(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	setupEnrichTestDB(t)
	defer teardownTestDB(t)

	db := database.Get()

	// Insert a movie with no TVDB ID
	movie := models.Movie{
		TMDBID:    12345,
		TMDBTitle: "Test Movie",
		TMDBYear:  2020,
	}
	if err := db.Create(&movie).Error; err != nil {
		t.Fatalf("failed to create test movie: %v", err)
	}

	srv := newTMDBTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tvdb_id": 99001}`))
	})

	client := newTMDBClientForTest(t, srv.URL)
	stats, err := EnrichMissingTVDBIDs(db, client, EnrichTVDBOptions{})
	if err != nil {
		t.Fatalf("EnrichMissingTVDBIDs error: %v", err)
	}

	if stats.Updated != 1 {
		t.Errorf("expected Updated=1, got %d", stats.Updated)
	}
	if stats.Errors != 0 {
		t.Errorf("expected Errors=0, got %d", stats.Errors)
	}

	// Verify DB was updated
	var updated models.Movie
	db.First(&updated, movie.ID)
	if updated.TVDBID == nil || *updated.TVDBID != 99001 {
		t.Errorf("expected tvdb_id=99001, got %v", updated.TVDBID)
	}
}

// TestEnrichMissingTVDBIDs_MovieSkippedNoTVDB verifies that a Movie whose TMDB
// entry has no TVDB ID is skipped without error.
func TestEnrichMissingTVDBIDs_MovieSkippedNoTVDB(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	setupEnrichTestDB(t)
	defer teardownTestDB(t)

	db := database.Get()

	movie := models.Movie{
		TMDBID:    55555,
		TMDBTitle: "No TVDB Movie",
		TMDBYear:  2021,
	}
	if err := db.Create(&movie).Error; err != nil {
		t.Fatalf("failed to create test movie: %v", err)
	}

	srv := newTMDBTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// tvdb_id is absent/null in TMDB
		w.Write([]byte(`{"tvdb_id": null}`))
	})

	client := newTMDBClientForTest(t, srv.URL)
	stats, err := EnrichMissingTVDBIDs(db, client, EnrichTVDBOptions{})
	if err != nil {
		t.Fatalf("EnrichMissingTVDBIDs error: %v", err)
	}

	if stats.Skipped != 1 {
		t.Errorf("expected Skipped=1, got %d", stats.Skipped)
	}
	if stats.Updated != 0 {
		t.Errorf("expected Updated=0, got %d", stats.Updated)
	}
}

// TestEnrichMissingTVDBIDs_APIErrorContinues verifies that an API error for one
// record does not abort the entire run.
func TestEnrichMissingTVDBIDs_APIErrorContinues(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	setupEnrichTestDB(t)
	defer teardownTestDB(t)

	db := database.Get()

	// Two movies; the first triggers an API error, the second succeeds.
	movie1 := models.Movie{TMDBID: 11111, TMDBTitle: "Error Movie", TMDBYear: 2019}
	movie2 := models.Movie{TMDBID: 22222, TMDBTitle: "OK Movie", TMDBYear: 2019}
	db.Create(&movie1)
	db.Create(&movie2)

	callCount := 0
	srv := newTMDBTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tvdb_id": 77777}`))
	})

	client := newTMDBClientForTest(t, srv.URL)
	stats, err := EnrichMissingTVDBIDs(db, client, EnrichTVDBOptions{})
	if err != nil {
		t.Fatalf("EnrichMissingTVDBIDs error: %v", err)
	}

	if stats.Errors == 0 {
		t.Error("expected at least 1 error")
	}
	if stats.Updated == 0 {
		t.Error("expected at least 1 updated record")
	}
}

// TestEnrichMissingTVDBIDs_TVShowDeduplication verifies that multiple TVShow rows
// sharing the same tmdb_id result in only one TMDB API call.
func TestEnrichMissingTVDBIDs_TVShowDeduplication(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	setupEnrichTestDB(t)
	defer teardownTestDB(t)

	db := database.Get()

	// Three episodes of the same show (same tmdb_id, different season/episode)
	s1, s2 := 1, 2
	e1, e2, e3 := 1, 2, 1
	shows := []models.TVShow{
		{TMDBID: 9876, TMDBTitle: "Test Show", TMDBYear: 2022, Season: &s1, Episode: &e1},
		{TMDBID: 9876, TMDBTitle: "Test Show", TMDBYear: 2022, Season: &s1, Episode: &e2},
		{TMDBID: 9876, TMDBTitle: "Test Show", TMDBYear: 2022, Season: &s2, Episode: &e3},
	}
	for i := range shows {
		if err := db.Create(&shows[i]).Error; err != nil {
			t.Fatalf("failed to create tvshow: %v", err)
		}
	}

	apiCallCount := 0
	srv := newTMDBTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		apiCallCount++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tvdb_id": 54321}`))
	})

	client := newTMDBClientForTest(t, srv.URL)
	stats, err := EnrichMissingTVDBIDs(db, client, EnrichTVDBOptions{})
	if err != nil {
		t.Fatalf("EnrichMissingTVDBIDs error: %v", err)
	}

	if stats.Updated != 3 {
		t.Errorf("expected Updated=3, got %d", stats.Updated)
	}
	// The API should have been called exactly once for the shared tmdb_id
	if apiCallCount != 1 {
		t.Errorf("expected 1 API call (deduplication), got %d", apiCallCount)
	}
}

// TestEnrichMissingTVDBIDs_DryRun verifies that dry-run mode makes no DB writes.
func TestEnrichMissingTVDBIDs_DryRun(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	setupEnrichTestDB(t)
	defer teardownTestDB(t)

	db := database.Get()

	movie := models.Movie{TMDBID: 33333, TMDBTitle: "DryRun Movie", TMDBYear: 2023}
	if err := db.Create(&movie).Error; err != nil {
		t.Fatalf("failed to create test movie: %v", err)
	}

	srv := newTMDBTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tvdb_id": 11111}`))
	})

	client := newTMDBClientForTest(t, srv.URL)
	_, err := EnrichMissingTVDBIDs(db, client, EnrichTVDBOptions{DryRun: true})
	if err != nil {
		t.Fatalf("EnrichMissingTVDBIDs error: %v", err)
	}

	// Verify DB was NOT updated
	var check models.Movie
	db.First(&check, movie.ID)
	if check.TVDBID != nil {
		t.Errorf("expected tvdb_id to remain nil in dry-run, got %v", check.TVDBID)
	}
}
