package radarr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/glefebvre/stalkeer/internal/retry"
)

func TestNew(t *testing.T) {
	cfg := Config{
		BaseURL: "http://localhost:7878",
		APIKey:  "test-key",
	}

	client := New(cfg)

	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.baseURL != cfg.BaseURL {
		t.Errorf("expected baseURL %s, got %s", cfg.BaseURL, client.baseURL)
	}
	if client.apiKey != cfg.APIKey {
		t.Errorf("expected apiKey %s, got %s", cfg.APIKey, client.apiKey)
	}
}

func TestGetMissingMovies(t *testing.T) {
	movies := []Movie{
		{ID: 1, Title: "Test Movie 1", Year: 2020, TMDBID: 101, Monitored: true, HasFile: false},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/wanted/missing" {
			t.Errorf("expected path /api/v3/wanted/missing, got %s", r.URL.Path)
		}
		if r.Header.Get("X-Api-Key") != "test-key" {
			t.Errorf("expected X-Api-Key header")
		}

		w.Header().Set("Content-Type", "application/json")
		response := struct {
			TotalRecords int     `json:"totalRecords"`
			Records      []Movie `json:"records"`
		}{
			TotalRecords: len(movies),
			Records:      movies,
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := New(Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Timeout: 5 * time.Second,
		RetryConfig: retry.Config{
			MaxAttempts: 1,
		},
	})

	ctx := context.Background()
	missing, err := client.GetMissingMovies(ctx, FetchOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(missing) != 1 {
		t.Errorf("expected 1 missing movie, got %d", len(missing))
	}
	if missing[0].ID != 1 {
		t.Errorf("expected movie ID 1, got %d", missing[0].ID)
	}
}

func TestGetMovieDetails(t *testing.T) {
	movie := Movie{
		ID:        1,
		Title:     "Test Movie",
		Year:      2020,
		TMDBID:    101,
		Monitored: true,
		HasFile:   false,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/movie/1" {
			t.Errorf("expected path /api/v3/movie/1, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(movie)
	}))
	defer server.Close()

	client := New(Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Timeout: 5 * time.Second,
		RetryConfig: retry.Config{
			MaxAttempts: 1,
		},
	})

	ctx := context.Background()
	result, err := client.GetMovieDetails(ctx, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ID != movie.ID {
		t.Errorf("expected ID %d, got %d", movie.ID, result.ID)
	}
	if result.Title != movie.Title {
		t.Errorf("expected title %s, got %s", movie.Title, result.Title)
	}
}

func TestUpdateMovie(t *testing.T) {
	movie := &Movie{
		ID:        1,
		Title:     "Test Movie",
		Year:      2020,
		TMDBID:    101,
		Monitored: true,
		HasFile:   true,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("expected PUT method, got %s", r.Method)
		}
		if r.URL.Path != "/api/v3/movie/1" {
			t.Errorf("expected path /api/v3/movie/1, got %s", r.URL.Path)
		}

		var received Movie
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if received.ID != movie.ID {
			t.Errorf("expected ID %d, got %d", movie.ID, received.ID)
		}

		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(received)
	}))
	defer server.Close()

	client := New(Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Timeout: 5 * time.Second,
		RetryConfig: retry.Config{
			MaxAttempts: 1,
		},
	})

	ctx := context.Background()
	err := client.UpdateMovie(ctx, movie)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMovieTvdbIDDeserialization(t *testing.T) {
	t.Run("with tvdbId field", func(t *testing.T) {
		payload := `{"id":1,"title":"Test Movie","year":2020,"tvdbId":12345,"tmdbId":99}`
		var m Movie
		if err := json.Unmarshal([]byte(payload), &m); err != nil {
			t.Fatalf("unexpected unmarshal error: %v", err)
		}
		if m.TvdbID != 12345 {
			t.Errorf("expected TvdbID 12345, got %d", m.TvdbID)
		}
	})

	t.Run("without tvdbId field", func(t *testing.T) {
		payload := `{"id":2,"title":"No TVDB Movie","year":2021,"tmdbId":42}`
		var m Movie
		if err := json.Unmarshal([]byte(payload), &m); err != nil {
			t.Fatalf("unexpected unmarshal error: %v", err)
		}
		if m.TvdbID != 0 {
			t.Errorf("expected TvdbID 0 when field absent, got %d", m.TvdbID)
		}
	})
}

func TestClientRetry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.Header().Set("Content-Type", "application/json")
		response := struct {
			TotalRecords int     `json:"totalRecords"`
			Records      []Movie `json:"records"`
		}{TotalRecords: 0, Records: nil}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := New(Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Timeout: 5 * time.Second,
		RetryConfig: retry.Config{
			MaxAttempts:       3,
			InitialBackoff:    10 * time.Millisecond,
			MaxBackoff:        100 * time.Millisecond,
			BackoffMultiplier: 2.0,
			JitterFraction:    0.1,
		},
	})

	ctx := context.Background()
	result, err := client.GetMissingMovies(ctx, FetchOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d movies", len(result))
	}
	if attempts != 1 {
		t.Errorf("expected 1 request for empty result set, got %d", attempts)
	}
}

func TestGetMissingMoviesMultiPage(t *testing.T) {
	allMovies := []Movie{
		{ID: 1, Title: "Movie A", Year: 2020, TMDBID: 101},
		{ID: 2, Title: "Movie B", Year: 2021, TMDBID: 102},
		{ID: 3, Title: "Movie C", Year: 2022, TMDBID: 103},
	}
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/wanted/missing" {
			t.Errorf("expected path /api/v3/wanted/missing, got %s", r.URL.Path)
		}
		page := r.URL.Query().Get("page")
		pageSize := 2

		requestCount++
		var records []Movie
		switch page {
		case "1":
			records = allMovies[:pageSize]
		default:
			records = allMovies[pageSize:]
		}

		w.Header().Set("Content-Type", "application/json")
		response := struct {
			TotalRecords int     `json:"totalRecords"`
			Records      []Movie `json:"records"`
		}{
			TotalRecords: len(allMovies),
			Records:      records,
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := New(Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Timeout: 5 * time.Second,
		RetryConfig: retry.Config{
			MaxAttempts: 1,
		},
	})

	ctx := context.Background()
	result, err := client.GetMissingMovies(ctx, FetchOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != len(allMovies) {
		t.Errorf("expected %d movies, got %d", len(allMovies), len(result))
	}
	if requestCount != 2 {
		t.Errorf("expected 2 page requests, got %d", requestCount)
	}
}

func TestGetMissingMoviesEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := struct {
			TotalRecords int     `json:"totalRecords"`
			Records      []Movie `json:"records"`
		}{TotalRecords: 0, Records: nil}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := New(Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Timeout: 5 * time.Second,
		RetryConfig: retry.Config{
			MaxAttempts: 1,
		},
	})

	ctx := context.Background()
	result, err := client.GetMissingMovies(ctx, FetchOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %d movies", len(result))
	}
}

func TestGetMissingMoviesWithLimit(t *testing.T) {
	allMovies := []Movie{
		{ID: 1, Title: "Movie A", Year: 2020, TMDBID: 101},
		{ID: 2, Title: "Movie B", Year: 2021, TMDBID: 102},
		{ID: 3, Title: "Movie C", Year: 2022, TMDBID: 103},
		{ID: 4, Title: "Movie D", Year: 2023, TMDBID: 104},
		{ID: 5, Title: "Movie E", Year: 2024, TMDBID: 105},
	}
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		const pageSize = 2
		requestCount++
		var records []Movie
		switch page {
		case "1":
			records = allMovies[:pageSize]
		case "2":
			records = allMovies[pageSize : pageSize*2]
		default:
			records = allMovies[pageSize*2:]
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			TotalRecords int     `json:"totalRecords"`
			Records      []Movie `json:"records"`
		}{TotalRecords: len(allMovies), Records: records})
	}))
	defer server.Close()

	client := New(Config{
		BaseURL:     server.URL,
		APIKey:      "test-key",
		Timeout:     5 * time.Second,
		RetryConfig: retry.Config{MaxAttempts: 1},
	})

	t.Run("limit within first page", func(t *testing.T) {
		requestCount = 0
		result, err := client.GetMissingMovies(context.Background(), FetchOptions{Limit: 1})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 1 {
			t.Errorf("expected 1 movie, got %d", len(result))
		}
		if requestCount != 1 {
			t.Errorf("expected 1 request, got %d", requestCount)
		}
	})

	t.Run("limit on page boundary", func(t *testing.T) {
		requestCount = 0
		result, err := client.GetMissingMovies(context.Background(), FetchOptions{Limit: 2})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("expected 2 movies, got %d", len(result))
		}
		if requestCount != 1 {
			t.Errorf("expected 1 request, got %d", requestCount)
		}
	})

	t.Run("limit spanning two pages", func(t *testing.T) {
		requestCount = 0
		result, err := client.GetMissingMovies(context.Background(), FetchOptions{Limit: 3})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 3 {
			t.Errorf("expected 3 movies, got %d", len(result))
		}
		if requestCount != 2 {
			t.Errorf("expected 2 requests, got %d", requestCount)
		}
	})

	t.Run("limit larger than total", func(t *testing.T) {
		requestCount = 0
		result, err := client.GetMissingMovies(context.Background(), FetchOptions{Limit: 100})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != len(allMovies) {
			t.Errorf("expected %d movies, got %d", len(allMovies), len(result))
		}
	})
}
