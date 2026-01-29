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
		{ID: 2, Title: "Test Movie 2", Year: 2021, TMDBID: 102, Monitored: true, HasFile: true},
		{ID: 3, Title: "Test Movie 3", Year: 2022, TMDBID: 103, Monitored: false, HasFile: false},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/movie" {
			t.Errorf("expected path /api/v3/movie, got %s", r.URL.Path)
		}
		if r.Header.Get("X-Api-Key") != "test-key" {
			t.Errorf("expected X-Api-Key header")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(movies)
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
	missing, err := client.GetMissingMovies(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only return monitored movies without files
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

func TestClientRetry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Movie{})
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
	_, err := client.GetMissingMovies(ctx)

	// The first attempt fails with 503, but second succeeds, so no error expected
	if err == nil {
		if attempts < 2 {
			t.Errorf("expected at least 2 attempts, got %d", attempts)
		}
	} else {
		// If we still got an error, it means retries weren't working
		t.Fatalf("unexpected error after retries: %v", err)
	}
}
