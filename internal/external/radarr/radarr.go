package radarr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	apperrors "github.com/glefebvre/stalkeer/internal/apperrors"
	"github.com/glefebvre/stalkeer/internal/logger"
	"github.com/glefebvre/stalkeer/internal/retry"
)

// Client represents a Radarr API client
type Client struct {
	baseURL     string
	apiKey      string
	httpClient  *http.Client
	retryConfig retry.Config
	logger      *logger.Logger
}

// Config holds Radarr client configuration
type Config struct {
	BaseURL     string
	APIKey      string
	Timeout     time.Duration
	RetryConfig retry.Config
	Logger      *logger.Logger
}

// Movie represents a Radarr movie
type Movie struct {
	ID               int       `json:"id"`
	Title            string    `json:"title"`
	Year             int       `json:"year"`
	TvdbID           int       `json:"tvdbId"`
	TMDBID           int       `json:"tmdbId"`
	Path             string    `json:"path"`
	Monitored        bool      `json:"monitored"`
	HasFile          bool      `json:"hasFile"`
	SizeOnDisk       int64     `json:"sizeOnDisk"`
	Added            time.Time `json:"added"`
	QualityProfileID int       `json:"qualityProfileId"`
}

// FetchOptions controls how many records are fetched. Limit 0 means unlimited.
type FetchOptions struct {
	Limit int
}

// New creates a new Radarr client
func New(cfg Config) *Client {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	if cfg.RetryConfig.MaxAttempts == 0 {
		cfg.RetryConfig = retry.DefaultConfig()
	}

	return &Client{
		baseURL: cfg.BaseURL,
		apiKey:  cfg.APIKey,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		retryConfig: cfg.RetryConfig,
		logger:      cfg.Logger,
	}
}

// GetMissingMovies retrieves all monitored movies that are not downloaded by paginating
// the wanted/missing endpoint with server-side filtering.
// GetMissingMovies retrieves missing movies by paginating the wanted/missing
// endpoint. Pagination stops when all records are fetched or opts.Limit is reached
// (0 = unlimited).
func (c *Client) GetMissingMovies(ctx context.Context, opts FetchOptions) ([]Movie, error) {
	const ps = 1000
	var all []Movie
	for page := 1; ; page++ {
		endpoint := fmt.Sprintf("/api/v3/wanted/missing?page=%d&pageSize=%d&sortKey=title&sortDirection=ascending", page, ps)

		var records []Movie
		var total int
		err := retry.Do(ctx, c.retryConfig, func() error {
			r, t, err := c.getPagedMovies(ctx, endpoint)
			if err != nil {
				return err
			}
			records = r
			total = t
			return nil
		}, apperrors.IsRetryable)

		if err != nil {
			return nil, apperrors.ExternalServiceError("radarr", "failed to get missing movies", err)
		}

		all = append(all, records...)

		if c.logger != nil {
			c.logger.Info(fmt.Sprintf("radarr: fetched page %d (%d/%d movies)", page, len(all), total))
		}

		if opts.Limit > 0 && len(all) >= opts.Limit {
			all = all[:opts.Limit]
			break
		}
		if len(all) >= total || len(records) == 0 {
			break
		}
	}
	return all, nil
}

// GetMovieDetails retrieves detailed information for a specific movie
func (c *Client) GetMovieDetails(ctx context.Context, id int) (*Movie, error) {
	endpoint := fmt.Sprintf("/api/v3/movie/%d", id)

	var movie Movie
	err := retry.Do(ctx, c.retryConfig, func() error {
		m, err := c.getMovie(ctx, endpoint)
		if err != nil {
			return err
		}
		movie = *m
		return nil
	}, apperrors.IsRetryable)

	if err != nil {
		return nil, apperrors.ExternalServiceError("radarr", "failed to get movie details", err)
	}

	return &movie, nil
}

// UpdateMovie updates a movie in Radarr
func (c *Client) UpdateMovie(ctx context.Context, movie *Movie) error {
	endpoint := fmt.Sprintf("/api/v3/movie/%d", movie.ID)

	err := retry.Do(ctx, c.retryConfig, func() error {
		return c.putMovie(ctx, endpoint, movie)
	}, apperrors.IsRetryable)

	if err != nil {
		return apperrors.ExternalServiceError("radarr", "failed to update movie", err)
	}

	return nil
}

func (c *Client) getPagedMovies(ctx context.Context, endpoint string) ([]Movie, int, error) {
	req, err := c.newRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, 0, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, 0, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var response struct {
		TotalRecords int     `json:"totalRecords"`
		Records      []Movie `json:"records"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, 0, fmt.Errorf("failed to decode response: %w", err)
	}

	return response.Records, response.TotalRecords, nil
}

func (c *Client) getMovies(ctx context.Context, endpoint string) ([]Movie, error) {
	req, err := c.newRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var movies []Movie
	if err := json.NewDecoder(resp.Body).Decode(&movies); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return movies, nil
}

func (c *Client) getMovie(ctx context.Context, endpoint string) (*Movie, error) {
	req, err := c.newRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var movie Movie
	if err := json.NewDecoder(resp.Body).Decode(&movie); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &movie, nil
}

func (c *Client) putMovie(ctx context.Context, endpoint string, movie *Movie) error {
	req, err := c.newRequest(ctx, "PUT", endpoint, movie)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *Client) newRequest(ctx context.Context, method, endpoint string, body interface{}) (*http.Request, error) {
	url := c.baseURL + endpoint

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	return req, nil
}
