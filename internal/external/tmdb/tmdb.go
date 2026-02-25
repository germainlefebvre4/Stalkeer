package tmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/glefebvre/stalkeer/internal/circuitbreaker"
	"github.com/glefebvre/stalkeer/internal/logger"
	"github.com/glefebvre/stalkeer/internal/retry"
)

const defaultTimeout = 30 * time.Second

// baseURL is a var so tests can override it with an httptest server address.
var baseURL = "https://api.themoviedb.org/3"

// Client handles TMDB API interactions
type Client struct {
	apiKey          string
	language        string
	httpClient      *http.Client
	logger          *logger.Logger
	circuitBrk      *circuitbreaker.CircuitBreaker
	requestInterval time.Duration     // minimum gap between HTTP requests; 0 = no limiting
	lastRequestAt   time.Time         // when the last HTTP request was initiated
	cache           map[string][]byte // URL → raw JSON response (scoped to client lifetime)
	cacheMu         sync.RWMutex      // protects cache
}

// Config holds TMDB client configuration
type Config struct {
	APIKey            string
	Language          string // e.g., "en-US", "fr-FR,fr;q=0.9,en-US;q=0.5,en;q=0.5"
	Timeout           time.Duration
	RequestsPerSecond float64 // max outbound requests per second; 0 = no limit (default: 4.0)
}

// MovieResult represents a movie search result from TMDB
type MovieResult struct {
	ID            int     `json:"id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	ReleaseDate   string  `json:"release_date"` // YYYY-MM-DD
	PosterPath    *string `json:"poster_path"`
	BackdropPath  *string `json:"backdrop_path"`
	Overview      string  `json:"overview"`
	VoteAverage   float64 `json:"vote_average"`
	Popularity    float64 `json:"popularity"`
	GenreIDs      []int   `json:"genre_ids"`
}

// TVShowResult represents a TV show search result from TMDB
type TVShowResult struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	OriginalName string  `json:"original_name"`
	FirstAirDate string  `json:"first_air_date"` // YYYY-MM-DD
	PosterPath   *string `json:"poster_path"`
	BackdropPath *string `json:"backdrop_path"`
	Overview     string  `json:"overview"`
	VoteAverage  float64 `json:"vote_average"`
	Popularity   float64 `json:"popularity"`
	GenreIDs     []int   `json:"genre_ids"`
}

// MovieSearchResponse represents the TMDB movie search API response
type MovieSearchResponse struct {
	Page         int           `json:"page"`
	Results      []MovieResult `json:"results"`
	TotalPages   int           `json:"total_pages"`
	TotalResults int           `json:"total_results"`
}

// TVShowSearchResponse represents the TMDB TV show search API response
type TVShowSearchResponse struct {
	Page         int            `json:"page"`
	Results      []TVShowResult `json:"results"`
	TotalPages   int            `json:"total_pages"`
	TotalResults int            `json:"total_results"`
}

// MovieDetails represents detailed movie information
type MovieDetails struct {
	ID            int     `json:"id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	ReleaseDate   string  `json:"release_date"`
	PosterPath    *string `json:"poster_path"`
	BackdropPath  *string `json:"backdrop_path"`
	Overview      string  `json:"overview"`
	VoteAverage   float64 `json:"vote_average"`
	Popularity    float64 `json:"popularity"`
	Runtime       *int    `json:"runtime"`
	Genres        []Genre `json:"genres"`
}

// TVShowDetails represents detailed TV show information
type TVShowDetails struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	OriginalName string  `json:"original_name"`
	FirstAirDate string  `json:"first_air_date"`
	PosterPath   *string `json:"poster_path"`
	BackdropPath *string `json:"backdrop_path"`
	Overview     string  `json:"overview"`
	VoteAverage  float64 `json:"vote_average"`
	Popularity   float64 `json:"popularity"`
	Genres       []Genre `json:"genres"`
}

// Genre represents a TMDB genre
type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// ExternalIDs represents external IDs for a movie or TV show
type ExternalIDs struct {
	IMDBID      *string `json:"imdb_id"`
	TVDBID      *int    `json:"tvdb_id"`
	FacebookID  *string `json:"facebook_id"`
	InstagramID *string `json:"instagram_id"`
	TwitterID   *string `json:"twitter_id"`
}

// NewClient creates a new TMDB API client
func NewClient(cfg Config) *Client {
	if cfg.Timeout == 0 {
		cfg.Timeout = defaultTimeout
	}
	if cfg.Language == "" {
		cfg.Language = "en-US"
	}

	cb := circuitbreaker.New(circuitbreaker.Config{
		MaxFailures: 5,
		Timeout:     60 * time.Second,
	})

	var requestInterval time.Duration
	if cfg.RequestsPerSecond > 0 {
		requestInterval = time.Duration(float64(time.Second) / cfg.RequestsPerSecond)
	}

	return &Client{
		apiKey:   cfg.APIKey,
		language: cfg.Language,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		logger:          logger.AppLogger(),
		circuitBrk:      cb,
		requestInterval: requestInterval,
		cache:           make(map[string][]byte),
	}
}

// SearchMovie searches for movies by title and optional year
func (c *Client) SearchMovie(title string, year *int) (*MovieResult, error) {
	params := url.Values{}
	params.Set("query", title)
	if year != nil && *year > 0 {
		params.Set("year", fmt.Sprintf("%d", *year))
	}

	var response MovieSearchResponse
	if err := c.makeRequest("/search/movie", params, &response); err != nil {
		return nil, err
	}

	if len(response.Results) == 0 {
		return nil, fmt.Errorf("no results found for movie: %s", title)
	}

	// Return the first (most relevant) result
	return &response.Results[0], nil
}

// SearchTVShow searches for TV shows by title
func (c *Client) SearchTVShow(title string) (*TVShowResult, error) {
	params := url.Values{}
	params.Set("query", title)

	var response TVShowSearchResponse
	if err := c.makeRequest("/search/tv", params, &response); err != nil {
		return nil, err
	}

	if len(response.Results) == 0 {
		return nil, fmt.Errorf("no results found for TV show: %s", title)
	}

	// Return the first (most relevant) result
	return &response.Results[0], nil
}

// GetMovieDetails retrieves detailed information for a specific movie
func (c *Client) GetMovieDetails(movieID int) (*MovieDetails, error) {
	var details MovieDetails
	endpoint := fmt.Sprintf("/movie/%d", movieID)
	if err := c.makeRequest(endpoint, url.Values{}, &details); err != nil {
		return nil, err
	}
	return &details, nil
}

// GetTVShowDetails retrieves detailed information for a specific TV show
func (c *Client) GetTVShowDetails(tvShowID int) (*TVShowDetails, error) {
	var details TVShowDetails
	endpoint := fmt.Sprintf("/tv/%d", tvShowID)
	if err := c.makeRequest(endpoint, url.Values{}, &details); err != nil {
		return nil, err
	}
	return &details, nil
}

// GetMovieExternalIDs retrieves external IDs for a specific movie
func (c *Client) GetMovieExternalIDs(movieID int) (*ExternalIDs, error) {
	var externalIDs ExternalIDs
	endpoint := fmt.Sprintf("/movie/%d/external_ids", movieID)
	if err := c.makeRequest(endpoint, url.Values{}, &externalIDs); err != nil {
		return nil, err
	}
	return &externalIDs, nil
}

// GetTVShowExternalIDs retrieves external IDs for a specific TV show
func (c *Client) GetTVShowExternalIDs(tvShowID int) (*ExternalIDs, error) {
	var externalIDs ExternalIDs
	endpoint := fmt.Sprintf("/tv/%d/external_ids", tvShowID)
	if err := c.makeRequest(endpoint, url.Values{}, &externalIDs); err != nil {
		return nil, err
	}
	return &externalIDs, nil
}

// makeRequest performs an HTTP request to the TMDB API with caching, rate limiting,
// circuit breaker, and retry.
func (c *Client) makeRequest(endpoint string, params url.Values, result interface{}) error {
	// Add API key and language to parameters
	params.Set("api_key", c.apiKey)
	params.Set("language", c.language)

	requestURL := fmt.Sprintf("%s%s?%s", baseURL, endpoint, params.Encode())

	// Check cache first — no HTTP call, no rate-limit slot consumed on hit.
	c.cacheMu.RLock()
	if cached, ok := c.cache[requestURL]; ok {
		c.cacheMu.RUnlock()
		return json.Unmarshal(cached, result)
	}
	c.cacheMu.RUnlock()

	// Rate-limit: sleep until the minimum interval has elapsed since the last request.
	if c.requestInterval > 0 {
		if gap := c.requestInterval - time.Since(c.lastRequestAt); gap > 0 {
			time.Sleep(gap)
		}
		c.lastRequestAt = time.Now()
	}

	ctx := context.Background()
	retryCfg := retry.Config{
		MaxAttempts:       3,
		InitialBackoff:    1 * time.Second,
		MaxBackoff:        10 * time.Second,
		BackoffMultiplier: 2.0,
		JitterFraction:    0.1,
	}

	// rawBody is captured by the closure on success for caching after retry.Do.
	var rawBody []byte

	operation := func() error {
		// Execute through circuit breaker
		return c.circuitBrk.Execute(func() error {
			req, err := http.NewRequest("GET", requestURL, nil)
			if err != nil {
				return err
			}

			req.Header.Set("Accept-Language", c.language)
			req.Header.Set("Accept", "application/json")

			resp, err := c.httpClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode == 429 {
				// Honour the Retry-After header before returning the retryable error.
				// Supports both seconds ("30") and HTTP-date formats.
				if h := resp.Header.Get("Retry-After"); h != "" {
					var wait time.Duration
					if secs, err := strconv.Atoi(h); err == nil {
						wait = time.Duration(secs) * time.Second
					} else if t, err := http.ParseTime(h); err == nil {
						wait = time.Until(t)
					}
					if wait > 0 {
						time.Sleep(wait)
					}
				}
				return fmt.Errorf("TMDB API rate limit exceeded")
			}

			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("TMDB API error (status %d): %s", resp.StatusCode, string(body))
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}

			if err := json.Unmarshal(body, result); err != nil {
				return fmt.Errorf("failed to unmarshal response: %w", err)
			}

			rawBody = body
			return nil
		})
	}

	// Define retryable errors
	isRetryable := func(err error) bool {
		if err == nil {
			return false
		}
		return strings.Contains(err.Error(), "rate limit") ||
			strings.Contains(err.Error(), "timeout") ||
			strings.Contains(err.Error(), "temporary")
	}

	err := retry.Do(ctx, retryCfg, operation, isRetryable)
	if err != nil {
		c.logger.WithFields(map[string]interface{}{
			"endpoint": endpoint,
			"error":    err,
		}).Warn("TMDB API request failed after retries")
		return err
	}

	// Cache the successful response for the lifetime of this client.
	if rawBody != nil {
		c.cacheMu.Lock()
		c.cache[requestURL] = rawBody
		c.cacheMu.Unlock()
	}

	return nil
}

// ExtractYear extracts year from TMDB date string (YYYY-MM-DD)
func ExtractYear(dateStr string) int {
	if dateStr == "" {
		return 0
	}
	parts := strings.Split(dateStr, "-")
	if len(parts) == 0 {
		return 0
	}
	var year int
	fmt.Sscanf(parts[0], "%d", &year)
	return year
}

// FormatGenres converts genre slice to comma-separated string
func FormatGenres(genres []Genre) string {
	if len(genres) == 0 {
		return ""
	}
	names := make([]string, len(genres))
	for i, g := range genres {
		names[i] = g.Name
	}
	return strings.Join(names, ", ")
}
