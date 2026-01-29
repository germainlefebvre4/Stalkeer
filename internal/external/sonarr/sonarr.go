package sonarr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/glefebvre/stalkeer/internal/errors"
	"github.com/glefebvre/stalkeer/internal/retry"
)

// Client represents a Sonarr API client
type Client struct {
	baseURL     string
	apiKey      string
	httpClient  *http.Client
	retryConfig retry.Config
}

// Config holds Sonarr client configuration
type Config struct {
	BaseURL     string
	APIKey      string
	Timeout     time.Duration
	RetryConfig retry.Config
}

// Series represents a Sonarr series
type Series struct {
	ID                int       `json:"id"`
	Title             string    `json:"title"`
	TvdbID            int       `json:"tvdbId"`
	Path              string    `json:"path"`
	Monitored         bool      `json:"monitored"`
	SeasonCount       int       `json:"seasonCount"`
	EpisodeFileCount  int       `json:"episodeFileCount"`
	TotalEpisodeCount int       `json:"totalEpisodeCount"`
	Added             time.Time `json:"added"`
	QualityProfileID  int       `json:"qualityProfileId"`
}

// Episode represents a Sonarr episode
type Episode struct {
	ID            int       `json:"id"`
	SeriesID      int       `json:"seriesId"`
	Title         string    `json:"title"`
	SeasonNumber  int       `json:"seasonNumber"`
	EpisodeNumber int       `json:"episodeNumber"`
	HasFile       bool      `json:"hasFile"`
	Monitored     bool      `json:"monitored"`
	AirDate       string    `json:"airDate"`
	AirDateUtc    time.Time `json:"airDateUtc"`
}

// New creates a new Sonarr client
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
	}
}

// GetMissingSeries retrieves all monitored series with missing episodes
func (c *Client) GetMissingSeries(ctx context.Context) ([]Series, error) {
	endpoint := "/api/v3/series"

	var allSeries []Series
	err := retry.Do(ctx, c.retryConfig, func() error {
		series, err := c.getSeries(ctx, endpoint)
		if err != nil {
			return err
		}
		allSeries = series
		return nil
	}, errors.IsRetryable)

	if err != nil {
		return nil, errors.ExternalServiceError("sonarr", "failed to get series", err)
	}

	// Filter for monitored series with missing episodes
	var missing []Series
	for _, s := range allSeries {
		if s.Monitored && s.EpisodeFileCount < s.TotalEpisodeCount {
			missing = append(missing, s)
		}
	}

	return missing, nil
}

// GetMissingEpisodes retrieves all missing episodes across all series
func (c *Client) GetMissingEpisodes(ctx context.Context) ([]Episode, error) {
	endpoint := "/api/v3/wanted/missing?page=1&pageSize=1000"

	var episodes []Episode
	err := retry.Do(ctx, c.retryConfig, func() error {
		eps, err := c.getEpisodes(ctx, endpoint)
		if err != nil {
			return err
		}
		episodes = eps
		return nil
	}, errors.IsRetryable)

	if err != nil {
		return nil, errors.ExternalServiceError("sonarr", "failed to get missing episodes", err)
	}

	return episodes, nil
}

// GetEpisodeDetails retrieves detailed information for a specific episode
func (c *Client) GetEpisodeDetails(ctx context.Context, id int) (*Episode, error) {
	endpoint := fmt.Sprintf("/api/v3/episode/%d", id)

	var episode Episode
	err := retry.Do(ctx, c.retryConfig, func() error {
		ep, err := c.getEpisode(ctx, endpoint)
		if err != nil {
			return err
		}
		episode = *ep
		return nil
	}, errors.IsRetryable)

	if err != nil {
		return nil, errors.ExternalServiceError("sonarr", "failed to get episode details", err)
	}

	return &episode, nil
}

// UpdateEpisode updates an episode in Sonarr
func (c *Client) UpdateEpisode(ctx context.Context, episode *Episode) error {
	endpoint := fmt.Sprintf("/api/v3/episode/%d", episode.ID)

	err := retry.Do(ctx, c.retryConfig, func() error {
		return c.putEpisode(ctx, endpoint, episode)
	}, errors.IsRetryable)

	if err != nil {
		return errors.ExternalServiceError("sonarr", "failed to update episode", err)
	}

	return nil
}

func (c *Client) getSeries(ctx context.Context, endpoint string) ([]Series, error) {
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

	var series []Series
	if err := json.NewDecoder(resp.Body).Decode(&series); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return series, nil
}

func (c *Client) getEpisodes(ctx context.Context, endpoint string) ([]Episode, error) {
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

	var response struct {
		Records []Episode `json:"records"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return response.Records, nil
}

func (c *Client) getEpisode(ctx context.Context, endpoint string) (*Episode, error) {
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

	var episode Episode
	if err := json.NewDecoder(resp.Body).Decode(&episode); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &episode, nil
}

func (c *Client) putEpisode(ctx context.Context, endpoint string, episode *Episode) error {
	req, err := c.newRequest(ctx, "PUT", endpoint, episode)
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
