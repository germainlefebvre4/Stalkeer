package tmdb

import (
	"fmt"
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
		w.Write([]byte(`{"page":1,"results":[{"id":603,"title":"The Matrix","original_title":"The Matrix","release_date":"1999-03-30","overview":"A computer hacker learns about the true nature of reality.","vote_average":8.7,"popularity":58.123,"genre_ids":[28,878]}],"total_pages":1,"total_results":1}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL, 0)

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
		w.Write([]byte(`{"page":1,"results":[],"total_pages":0,"total_results":0}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL, 0)

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

// movieJSON is a minimal valid search response used across cache/rate-limit tests.
const movieJSON = `{"page":1,"results":[{"id":1,"title":"Test","original_title":"Test","release_date":"2020-01-01","vote_average":7.0,"popularity":10.0,"genre_ids":[]}],"total_pages":1,"total_results":1}`

// newTestClient creates a client pointed at the provided test server URL.
func newTestClient(serverURL string, rps float64) *Client {
	c := NewClient(Config{
		APIKey:            "test-key",
		Language:          "en-US",
		RequestsPerSecond: rps,
	})
	baseURL = serverURL
	return c
}

func TestCacheHitSkipsHTTP(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, movieJSON)
	}))
	defer server.Close()

	client := newTestClient(server.URL, 0)

	year := 2020
	if _, err := client.SearchMovie("Test", &year); err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	if _, err := client.SearchMovie("Test", &year); err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected 1 HTTP call (cache hit on second), got %d", callCount)
	}
}

func TestRateLimitingDisabledWhenZero(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, movieJSON)
	}))
	defer server.Close()

	client := newTestClient(server.URL, 0)

	start := time.Now()
	year := 2020
	// Use different titles to avoid cache hits
	for i := 0; i < 3; i++ {
		title := fmt.Sprintf("Movie%d", i)
		_ = client.cache // clear cache between calls by using distinct titles
		if _, err := client.SearchMovie(title, &year); err != nil {
			t.Fatalf("call %d failed: %v", i, err)
		}
	}
	elapsed := time.Since(start)

	// Without rate limiting, 3 calls should complete well under 500ms
	if elapsed > 500*time.Millisecond {
		t.Errorf("expected fast completion with rps=0, took %v", elapsed)
	}
}

func TestRetryAfterSecondsFormat(t *testing.T) {
	attempt := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		if attempt == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, movieJSON)
	}))
	defer server.Close()

	client := newTestClient(server.URL, 0)

	start := time.Now()
	year := 2020
	_, err := client.SearchMovie("RetryTest", &year)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	// The Retry-After: 1 sleep should have been observed
	if elapsed < 1*time.Second {
		t.Errorf("expected at least 1s wait for Retry-After, elapsed: %v", elapsed)
	}
	if attempt != 2 {
		t.Errorf("expected 2 attempts (429 then success), got %d", attempt)
	}
}

func TestRetryAfterHTTPDateFormat(t *testing.T) {
	attempt := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		if attempt == 1 {
			// Set Retry-After to 1 second from now in HTTP-date format
			retryAt := time.Now().Add(1 * time.Second).UTC().Format(http.TimeFormat)
			w.Header().Set("Retry-After", retryAt)
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, movieJSON)
	}))
	defer server.Close()

	client := newTestClient(server.URL, 0)

	start := time.Now()
	year := 2020
	_, err := client.SearchMovie("DateFormatTest", &year)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if elapsed < 1*time.Second {
		t.Errorf("expected at least 1s wait for HTTP-date Retry-After, elapsed: %v", elapsed)
	}
	if attempt != 2 {
		t.Errorf("expected 2 attempts, got %d", attempt)
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
