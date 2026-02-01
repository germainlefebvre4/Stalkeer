package tmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/glefebvre/stalkeer/internal/circuitbreaker"
	"github.com/glefebvre/stalkeer/internal/logger"
	"github.com/glefebvre/stalkeer/internal/retry"
)

const (
	baseURL        = "https://api.themoviedb.org/3"
	defaultTimeout = 10 * time.Second
)

// Client handles TMDB API interactions
type Client struct {
	apiKey     string
	language   string
	httpClient *http.Client
	logger     *logger.Logger
	circuitBrk *circuitbreaker.CircuitBreaker
}

// Config holds TMDB client configuration
type Config struct {
	APIKey   string
	Language string // e.g., "en-US", "fr-FR,fr;q=0.9,en-US;q=0.5,en;q=0.5"
	Timeout  time.Duration
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

	return &Client{
		apiKey:   cfg.APIKey,
		language: cfg.Language,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		logger:     logger.Default(),
		circuitBrk: cb,
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

// makeRequest performs an HTTP request to the TMDB API with circuit breaker and retry
func (c *Client) makeRequest(endpoint string, params url.Values, result interface{}) error {
	// Add API key and language to parameters
	params.Set("api_key", c.apiKey)
	params.Set("language", c.language)

	requestURL := fmt.Sprintf("%s%s?%s", baseURL, endpoint, params.Encode())

	// Use retry logic with circuit breaker
	ctx := context.Background()
	retryCfg := retry.Config{
		MaxAttempts:       3,
		InitialBackoff:    1 * time.Second,
		MaxBackoff:        10 * time.Second,
		BackoffMultiplier: 2.0,
		JitterFraction:    0.1,
	}

	operation := func() error {
		// Execute through circuit breaker
		return c.circuitBrk.Execute(func() error {
			req, err := http.NewRequest("GET", requestURL, nil)
			if err != nil {
				return err
			}

			// Set Accept-Language header
			req.Header.Set("Accept-Language", c.language)
			req.Header.Set("Accept", "application/json")

			resp, err := c.httpClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode == 429 {
				// Rate limit exceeded
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

			return nil
		})
	}

	// Define retryable errors
	isRetryable := func(err error) bool {
		if err == nil {
			return false
		}
		// Retry on rate limits and temporary errors
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
