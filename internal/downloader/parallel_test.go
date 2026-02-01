package downloader

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewParallel(t *testing.T) {
	tests := []struct {
		name        string
		concurrency int
		expected    int
	}{
		{
			name:        "with positive concurrency",
			concurrency: 5,
			expected:    5,
		},
		{
			name:        "with zero concurrency uses default",
			concurrency: 0,
			expected:    3,
		},
		{
			name:        "with negative concurrency uses default",
			concurrency: -1,
			expected:    3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pd := NewParallel(10*time.Second, 3, tt.concurrency)
			assert.NotNil(t, pd)
			assert.Equal(t, tt.expected, pd.GetConcurrency())
		})
	}
}

func TestParallelDownloader_DownloadBatch(t *testing.T) {
	_ = setupTestDB(t)

	// Create test server
	content := []byte("test content for parallel download")
	requestCount := 0
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()

		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		w.WriteHeader(http.StatusOK)
		w.Write(content)
	}))
	defer server.Close()

	// Create temp directory
	tempDir := t.TempDir()

	// Create jobs
	numJobs := 10
	jobs := make([]DownloadJob, numJobs)
	for i := 0; i < numJobs; i++ {
		jobs[i] = DownloadJob{
			ID: i,
			Options: DownloadOptions{
				URL:             server.URL,
				BaseDestPath: filepath.Join(tempDir, fmt.Sprintf("file_%d.txt", i)),
			},
		}
	}

	// Create parallel downloader with 3 concurrent downloads
	pd := NewParallel(10*time.Second, 3, 3)

	// Perform parallel downloads
	ctx := context.Background()
	resultsChan := pd.DownloadBatch(ctx, jobs)

	// Collect results
	var results []DownloadJobResult
	for result := range resultsChan {
		results = append(results, result)
	}

	// Assertions
	assert.Equal(t, numJobs, len(results))
	assert.Equal(t, numJobs, requestCount)

	// Verify all downloads succeeded
	successCount := 0
	for _, result := range results {
		if result.Error == nil {
			successCount++
			assert.NotNil(t, result.Result)
			assert.Equal(t, int64(len(content)), result.Result.FileSize)

			// Verify file exists
			_, err := os.Stat(result.Result.FilePath)
			assert.NoError(t, err)
		}
	}
	assert.Equal(t, numJobs, successCount)
}

func TestParallelDownloader_DownloadBatchSync(t *testing.T) {
	_ = setupTestDB(t)

	// Create test server
	content := []byte("sync batch download content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		w.WriteHeader(http.StatusOK)
		w.Write(content)
	}))
	defer server.Close()

	// Create temp directory
	tempDir := t.TempDir()

	// Create jobs
	jobs := []DownloadJob{
		{ID: 1, Options: DownloadOptions{URL: server.URL, BaseDestPath: filepath.Join(tempDir, "file1.txt")}},
		{ID: 2, Options: DownloadOptions{URL: server.URL, BaseDestPath: filepath.Join(tempDir, "file2.txt")}},
		{ID: 3, Options: DownloadOptions{URL: server.URL, BaseDestPath: filepath.Join(tempDir, "file3.txt")}},
	}

	// Create parallel downloader
	pd := NewParallel(10*time.Second, 3, 2)

	// Perform sync batch download
	ctx := context.Background()
	results := pd.DownloadBatchSync(ctx, jobs)

	// Assertions
	assert.Equal(t, len(jobs), len(results))

	for _, result := range results {
		assert.NoError(t, result.Error)
		assert.NotNil(t, result.Result)
	}
}

func TestParallelDownloader_DownloadBatchWithProgress(t *testing.T) {
	_ = setupTestDB(t)

	// Create test server
	content := []byte("progress tracking content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		w.WriteHeader(http.StatusOK)
		w.Write(content)
	}))
	defer server.Close()

	// Create temp directory
	tempDir := t.TempDir()

	// Create jobs
	numJobs := 5
	jobs := make([]DownloadJob, numJobs)
	for i := 0; i < numJobs; i++ {
		jobs[i] = DownloadJob{
			ID: i,
			Options: DownloadOptions{
				URL:             server.URL,
				BaseDestPath: filepath.Join(tempDir, fmt.Sprintf("file_%d.txt", i)),
			},
		}
	}

	// Track progress
	var progressUpdates []int
	var progressMu sync.Mutex

	// Create parallel downloader
	pd := NewParallel(10*time.Second, 3, 3)

	// Perform download with progress tracking
	ctx := context.Background()
	results := pd.DownloadBatchWithProgress(ctx, jobs, func(completed, total int) {
		progressMu.Lock()
		progressUpdates = append(progressUpdates, completed)
		progressMu.Unlock()
	})

	// Assertions
	assert.Equal(t, numJobs, len(results))
	assert.Equal(t, numJobs, len(progressUpdates))

	// Verify progress updates are increasing
	for i := 1; i < len(progressUpdates); i++ {
		assert.GreaterOrEqual(t, progressUpdates[i], progressUpdates[i-1])
	}

	// Last progress should be total
	assert.Equal(t, numJobs, progressUpdates[len(progressUpdates)-1])
}

func TestParallelDownloader_ConcurrencyControl(t *testing.T) {
	_ = setupTestDB(t)

	// Create test server that tracks concurrent requests
	var currentConcurrent int
	var maxConcurrent int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		currentConcurrent++
		if currentConcurrent > maxConcurrent {
			maxConcurrent = currentConcurrent
		}
		mu.Unlock()

		// Simulate slow download
		time.Sleep(100 * time.Millisecond)

		mu.Lock()
		currentConcurrent--
		mu.Unlock()

		content := []byte("test")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		w.WriteHeader(http.StatusOK)
		w.Write(content)
	}))
	defer server.Close()

	// Create temp directory
	tempDir := t.TempDir()

	// Create many jobs
	numJobs := 10
	jobs := make([]DownloadJob, numJobs)
	for i := 0; i < numJobs; i++ {
		jobs[i] = DownloadJob{
			ID: i,
			Options: DownloadOptions{
				URL:             server.URL,
				BaseDestPath: filepath.Join(tempDir, fmt.Sprintf("file_%d.txt", i)),
			},
		}
	}

	// Create parallel downloader with concurrency limit of 3
	concurrencyLimit := 3
	pd := NewParallel(10*time.Second, 3, concurrencyLimit)

	// Perform parallel downloads
	ctx := context.Background()
	results := pd.DownloadBatchSync(ctx, jobs)

	// Verify all succeeded
	assert.Equal(t, numJobs, len(results))

	// Verify concurrency was respected
	assert.LessOrEqual(t, maxConcurrent, concurrencyLimit,
		"Max concurrent requests (%d) should not exceed limit (%d)", maxConcurrent, concurrencyLimit)
}

func TestParallelDownloader_ErrorHandling(t *testing.T) {
	_ = setupTestDB(t)

	// Create test server that fails for specific requests
	requestNum := 0
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestNum++
		currentRequest := requestNum
		mu.Unlock()

		// Fail requests 2 and 4
		if currentRequest == 2 || currentRequest == 4 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		content := []byte("success")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		w.WriteHeader(http.StatusOK)
		w.Write(content)
	}))
	defer server.Close()

	// Create temp directory
	tempDir := t.TempDir()

	// Create jobs
	numJobs := 5
	jobs := make([]DownloadJob, numJobs)
	for i := 0; i < numJobs; i++ {
		jobs[i] = DownloadJob{
			ID: i,
			Options: DownloadOptions{
				URL:             server.URL,
				BaseDestPath: filepath.Join(tempDir, fmt.Sprintf("file_%d.txt", i)),
			},
		}
	}

	// Create parallel downloader with retry disabled for faster test
	pd := NewParallel(10*time.Second, 1, 2)

	// Perform parallel downloads
	ctx := context.Background()
	results := pd.DownloadBatchSync(ctx, jobs)

	// Count successes and failures
	successCount := 0
	failureCount := 0
	for _, result := range results {
		if result.Error == nil {
			successCount++
		} else {
			failureCount++
		}
	}

	// Should have some successes and some failures
	assert.Greater(t, successCount, 0, "should have at least one success")
	assert.Greater(t, failureCount, 0, "should have at least one failure")
	assert.Equal(t, numJobs, successCount+failureCount)
}

func TestParallelDownloader_ContextCancellation(t *testing.T) {
	_ = setupTestDB(t)

	// Create test server with slow responses
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("content"))
	}))
	defer server.Close()

	// Create temp directory
	tempDir := t.TempDir()

	// Create jobs
	jobs := []DownloadJob{
		{ID: 1, Options: DownloadOptions{URL: server.URL, BaseDestPath: filepath.Join(tempDir, "file1.txt")}},
		{ID: 2, Options: DownloadOptions{URL: server.URL, BaseDestPath: filepath.Join(tempDir, "file2.txt")}},
		{ID: 3, Options: DownloadOptions{URL: server.URL, BaseDestPath: filepath.Join(tempDir, "file3.txt")}},
	}

	// Create parallel downloader
	pd := NewParallel(10*time.Second, 1, 2)

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Perform parallel downloads - should be cancelled
	results := pd.DownloadBatchSync(ctx, jobs)

	// All should fail due to context cancellation
	assert.Equal(t, len(jobs), len(results))
	for _, result := range results {
		assert.Error(t, result.Error)
	}
}

func TestParallelDownloader_SetConcurrency(t *testing.T) {
	pd := NewParallel(10*time.Second, 3, 5)
	assert.Equal(t, 5, pd.GetConcurrency())

	pd.SetConcurrency(10)
	assert.Equal(t, 10, pd.GetConcurrency())

	// Setting to 0 or negative should not change
	pd.SetConcurrency(0)
	assert.Equal(t, 10, pd.GetConcurrency())

	pd.SetConcurrency(-5)
	assert.Equal(t, 10, pd.GetConcurrency())
}
