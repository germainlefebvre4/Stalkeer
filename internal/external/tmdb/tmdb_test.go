package tmdb

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	cfg := Config{
		APIKey:   "test-api-key",
		Language: "en-US",
		Timeout:  5 * time.Second,
	}

	client := NewClient(cfg)

	if client == nil {
		t.Fatal("expected client to be created")
	}
	if client.apiKey != "test-api-key" {
		t.Errorf("expected API key 'test-api-key', got '%s'", client.apiKey)
	}
	if client.language != "en-US" {
		t.Errorf("expected language 'en-US', got '%s'", client.language)
	}
}

func TestNewClientDefaults(t *testing.T) {
	cfg := Config{
		APIKey: "test-api-key",
	}

	client := NewClient(cfg)

	if client.language != "en-US" {
		t.Errorf("expected default language 'en-US', got '%s'", client.language)
	}
	if client.httpClient.Timeout != defaultTimeout {
		t.Errorf("expected default timeout %v, got %v", defaultTimeout, client.httpClient.Timeout)
	}
}

func TestSearchMovie(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/movie" {
			t.Errorf("expected path '/search/movie', got '%s'", r.URL.Path)
		}

		query := r.URL.Query()
		if query.Get("query") != "The Matrix" {
			t.Errorf("expected query 'The Matrix', got '%s'", query.Get("query"))
		}
		if query.Get("year") != "1999" {
			t.Errorf("expected year '1999', got '%s'", query.Get("year"))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"page": 1,
			"results": [{
				"id": 603,
				"title": "The Matrix",
				"original_title": "The Matrix",
				"release_date": "1999-03-30",
				"poster_path": "/path/to/poster.jpg",
				"overview": "A computer hacker learns about the true nature of reality.",
				"vote_average": 8.7,
				"popularity": 58.123,
				"genre_ids": [28, 878]
			}],
			"total_pages": 1,
			"total_results": 1
		}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		APIKey:   "test-key",
		Language: "en-US",
	})

	year := 1999
	result, err := client.SearchMovie("The Matrix", &year)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.ID != 603 {
		t.Errorf("expected ID 603, got %d", result.ID)
	}
	if result.Title != "The Matrix" {
		t.Errorf("expected title 'The Matrix', got '%s'", result.Title)
	}
}

func TestSearchMovieNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"page": 1,
			"results": [],
			"total_pages": 0,
			"total_results": 0
		}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		APIKey:   "test-key",
		Language: "en-US",
	})

	_, err := client.SearchMovie("NonexistentMovie12345", nil)
	if err == nil {
		t.Fatal("expected error for movie not found")
	}
}

func TestExtractYear(t *testing.T) {
	tests := []struct {
		name     string
		dateStr  string
		expected int
	}{
		{"valid date", "2024-01-15", 2024},
		{"valid date 1999", "1999-03-30", 1999},
		{"empty string", "", 0},
		{"invalid format", "invalid", 0},
		{"year only", "2024", 2024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractYear(tt.dateStr)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestFormatGenres(t *testing.T) {
	tests := []struct {
		name     string
		genres   []Genre
		expected string
	}{
		{
			name:     "empty genres",
			genres:   []Genre{},
			expected: "",
		},
		{
			name: "single genre",
			genres: []Genre{
				{ID: 28, Name: "Action"},
			},
			expected: "Action",
		},
		{
			name: "multiple genres",
			genres: []Genre{
				{ID: 28, Name: "Action"},
				{ID: 878, Name: "Science Fiction"},
				{ID: 53, Name: "Thriller"},
			},
			expected: "Action, Science Fiction, Thriller",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatGenres(tt.genres)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}
