package sonarr

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
		BaseURL: "http://localhost:8989",
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

func TestGetMissingSeries(t *testing.T) {
	series := []Series{
		{ID: 1, Title: "Test Series 1", TvdbID: 101, Monitored: true, TotalEpisodeCount: 10, EpisodeFileCount: 5},
		{ID: 2, Title: "Test Series 2", TvdbID: 102, Monitored: true, TotalEpisodeCount: 10, EpisodeFileCount: 10},
		{ID: 3, Title: "Test Series 3", TvdbID: 103, Monitored: false, TotalEpisodeCount: 10, EpisodeFileCount: 5},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/series" {
			t.Errorf("expected path /api/v3/series, got %s", r.URL.Path)
		}
		if r.Header.Get("X-Api-Key") != "test-key" {
			t.Errorf("expected X-Api-Key header")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(series)
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
	missing, err := client.GetMissingSeries(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only return monitored series with missing episodes
	if len(missing) != 1 {
		t.Errorf("expected 1 missing series, got %d", len(missing))
	}
	if missing[0].ID != 1 {
		t.Errorf("expected series ID 1, got %d", missing[0].ID)
	}
}

func TestGetMissingEpisodes(t *testing.T) {
	episodes := []Episode{
		{ID: 1, SeriesID: 1, SeasonNumber: 1, EpisodeNumber: 1, HasFile: false, Monitored: true},
		{ID: 2, SeriesID: 1, SeasonNumber: 1, EpisodeNumber: 2, HasFile: false, Monitored: true},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/wanted/missing" {
			t.Errorf("expected path /api/v3/wanted/missing, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		response := struct {
			Records []Episode `json:"records"`
		}{
			Records: episodes,
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
	missing, err := client.GetMissingEpisodes(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(missing) != 2 {
		t.Errorf("expected 2 missing episodes, got %d", len(missing))
	}
}

func TestGetEpisodeDetails(t *testing.T) {
	episode := Episode{
		ID:            1,
		SeriesID:      1,
		Title:         "Test Episode",
		SeasonNumber:  1,
		EpisodeNumber: 1,
		HasFile:       false,
		Monitored:     true,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/episode/1" {
			t.Errorf("expected path /api/v3/episode/1, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(episode)
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
	result, err := client.GetEpisodeDetails(ctx, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ID != episode.ID {
		t.Errorf("expected ID %d, got %d", episode.ID, result.ID)
	}
	if result.Title != episode.Title {
		t.Errorf("expected title %s, got %s", episode.Title, result.Title)
	}
}

func TestUpdateEpisode(t *testing.T) {
	episode := &Episode{
		ID:            1,
		SeriesID:      1,
		Title:         "Test Episode",
		SeasonNumber:  1,
		EpisodeNumber: 1,
		HasFile:       true,
		Monitored:     true,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("expected PUT method, got %s", r.Method)
		}
		if r.URL.Path != "/api/v3/episode/1" {
			t.Errorf("expected path /api/v3/episode/1, got %s", r.URL.Path)
		}

		var received Episode
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if received.ID != episode.ID {
			t.Errorf("expected ID %d, got %d", episode.ID, received.ID)
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
	err := client.UpdateEpisode(ctx, episode)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
