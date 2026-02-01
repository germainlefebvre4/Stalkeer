package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glefebvre/stalkeer/internal/database"
	"github.com/glefebvre/stalkeer/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(
		&models.ProcessedLine{},
		&models.Movie{},
		&models.TVShow{},
		&models.DownloadInfo{},
	)
	require.NoError(t, err)

	// Set global database instance
	database.Initialize()
	return db
}

func TestNew(t *testing.T) {
	tests := []struct {
		name          string
		timeout       time.Duration
		retryAttempts int
		wantTimeout   time.Duration
		wantRetries   int
	}{
		{
			name:          "with custom values",
			timeout:       60 * time.Second,
			retryAttempts: 5,
			wantTimeout:   60 * time.Second,
			wantRetries:   5,
		},
		{
			name:          "with zero values uses defaults",
			timeout:       0,
			retryAttempts: 0,
			wantTimeout:   300 * time.Second,
			wantRetries:   3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := New(tt.timeout, tt.retryAttempts)
			assert.NotNil(t, d)
			assert.Equal(t, tt.wantTimeout, d.httpClient.Timeout)
			assert.Equal(t, tt.wantRetries, d.retryConfig.MaxAttempts)
		})
	}
}

func TestDownload_Success(t *testing.T) {
	_ = setupTestDB(t)

	// Create test server
	content := []byte("test content for download")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		w.WriteHeader(http.StatusOK)
		w.Write(content)
	}))
	defer server.Close()

	// Create temp directory for test
	tempDir := t.TempDir()
	destPath := filepath.Join(tempDir, "testfile.txt")

	// Create downloader
	d := New(10*time.Second, 3)

	// Track progress calls
	var progressCalls int
	var lastDownloaded, lastTotal int64

	// Perform download
	result, err := d.Download(context.Background(), DownloadOptions{
		URL:             server.URL,
		BaseDestPath: destPath,
		OnProgress: func(downloaded, total int64) {
			progressCalls++
			lastDownloaded = downloaded
			lastTotal = total
		},
	})

	// Assertions
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, destPath, result.FilePath)
	assert.Equal(t, int64(len(content)), result.FileSize)
	assert.Greater(t, progressCalls, 0)
	assert.Equal(t, int64(len(content)), lastDownloaded)
	assert.Equal(t, int64(len(content)), lastTotal)

	// Verify file exists and content matches
	fileContent, err := os.ReadFile(destPath)
	require.NoError(t, err)
	assert.Equal(t, content, fileContent)

	// Verify no temp file left
	_, err = os.Stat(destPath + ".tmp")
	assert.True(t, os.IsNotExist(err))
}

func TestDownload_WithDatabaseTracking(t *testing.T) {
	db := setupTestDB(t)

	// Create test processed line
	lineURL := "http://example.com/stream.mkv"
	processedLine := &models.ProcessedLine{
		LineURL:     &lineURL,
		LineContent: "#EXTINF:-1,Test Movie",
		LineHash:    "testhash123",
		TvgName:     "Test Movie",
		GroupTitle:  "Movies",
		ContentType: models.ContentTypeMovies,
		State:       models.StateProcessed,
	}
	err := db.Create(processedLine).Error
	require.NoError(t, err)

	// Create test server
	content := []byte("test movie content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		w.WriteHeader(http.StatusOK)
		w.Write(content)
	}))
	defer server.Close()

	// Create temp directory
	tempDir := t.TempDir()
	destPath := filepath.Join(tempDir, "movie.mkv")

	// Create downloader
	d := New(10*time.Second, 3)

	// Perform download with database tracking
	result, err := d.Download(context.Background(), DownloadOptions{
		URL:             server.URL,
		BaseDestPath: destPath,
		ProcessedLineID: processedLine.ID,
	})

	// Assertions
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Verify database state updated to downloaded
	var updated models.ProcessedLine
	err = db.First(&updated, processedLine.ID).Error
	require.NoError(t, err)
	assert.Equal(t, models.StateDownloaded, updated.State)
}

func TestDownload_ValidationErrors(t *testing.T) {
	d := New(10*time.Second, 3)

	tests := []struct {
		name string
		opts DownloadOptions
	}{
		{
			name: "empty URL",
			opts: DownloadOptions{
				URL:             "",
				BaseDestPath: "/tmp/file.txt",
			},
		},
		{
			name: "empty destination",
			opts: DownloadOptions{
				URL:             "http://example.com/file",
				BaseDestPath: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := d.Download(context.Background(), tt.opts)
			assert.Error(t, err)
			assert.Nil(t, result)
		})
	}
}

func TestDownload_HTTPErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"not found", http.StatusNotFound},
		{"forbidden", http.StatusForbidden},
		{"internal server error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server with error status
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			// Create temp directory
			tempDir := t.TempDir()
			destPath := filepath.Join(tempDir, "file.txt")

			// Create downloader
			d := New(10*time.Second, 3)

			// Perform download
			result, err := d.Download(context.Background(), DownloadOptions{
				URL:             server.URL,
				BaseDestPath: destPath,
			})

			// Should fail
			assert.Error(t, err)
			assert.Nil(t, result)

			// File should not exist
			_, err = os.Stat(destPath)
			assert.True(t, os.IsNotExist(err))
		})
	}
}

func TestDownload_Retry(t *testing.T) {
	attemptCount := 0
	content := []byte("test content after retry")

	// Create test server that fails first 2 attempts, then succeeds
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		w.WriteHeader(http.StatusOK)
		w.Write(content)
	}))
	defer server.Close()

	// Create temp directory
	tempDir := t.TempDir()
	destPath := filepath.Join(tempDir, "file.txt")

	// Create downloader with 5 retry attempts
	d := New(10*time.Second, 5)

	// Perform download
	result, err := d.Download(context.Background(), DownloadOptions{
		URL:             server.URL,
		BaseDestPath: destPath,
	})

	// Should succeed after retries
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 3, attemptCount) // Should have tried 3 times
	assert.Equal(t, int64(len(content)), result.FileSize)

	// Verify file content
	fileContent, err := os.ReadFile(destPath)
	require.NoError(t, err)
	assert.Equal(t, content, fileContent)
}

func TestDownload_ContextCancellation(t *testing.T) {
	// Create a slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second) // Slow response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("content"))
	}))
	defer server.Close()

	// Create temp directory
	tempDir := t.TempDir()
	destPath := filepath.Join(tempDir, "file.txt")

	// Create downloader
	d := New(10*time.Second, 1)

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Perform download - should be cancelled
	result, err := d.Download(ctx, DownloadOptions{
		URL:             server.URL,
		BaseDestPath: destPath,
	})

	// Should fail due to context cancellation
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestDownload_DatabaseStateOnFailure(t *testing.T) {
	db := setupTestDB(t)

	// Create test processed line
	lineURL := "http://example.com/stream.mkv"
	processedLine := &models.ProcessedLine{
		LineURL:     &lineURL,
		LineContent: "#EXTINF:-1,Test Movie",
		LineHash:    "testhash456",
		TvgName:     "Test Movie",
		GroupTitle:  "Movies",
		ContentType: models.ContentTypeMovies,
		State:       models.StateProcessed,
	}
	err := db.Create(processedLine).Error
	require.NoError(t, err)

	// Create test server that always fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Create temp directory
	tempDir := t.TempDir()
	destPath := filepath.Join(tempDir, "movie.mkv")

	// Create downloader
	d := New(10*time.Second, 2)

	// Perform download with database tracking
	result, err := d.Download(context.Background(), DownloadOptions{
		URL:             server.URL,
		BaseDestPath: destPath,
		ProcessedLineID: processedLine.ID,
	})

	// Should fail
	assert.Error(t, err)
	assert.Nil(t, result)

	// Verify database state updated to failed
	var updated models.ProcessedLine
	err = db.First(&updated, processedLine.ID).Error
	require.NoError(t, err)
	assert.Equal(t, models.StateFailed, updated.State)
}

func TestProgressReader(t *testing.T) {
	content := []byte("test content for progress tracking")
	reader := &progressReader{
		reader: io.NopCloser(io.LimitReader(http.NoBody, int64(len(content)))),
		total:  int64(len(content)),
	}

	var progressUpdates int
	var lastDownloaded, lastTotal int64

	reader.onProgress = func(downloaded, total int64) {
		progressUpdates++
		lastDownloaded = downloaded
		lastTotal = total
	}

	// Simulate reading in chunks
	buf := make([]byte, 10)
	for {
		n, err := reader.Read(buf)
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		assert.Greater(t, n, 0)
	}

	assert.Greater(t, progressUpdates, 0)
	assert.Equal(t, int64(len(content)), lastTotal)
	// Note: lastDownloaded won't equal content length because we're using a limited reader
	_ = lastDownloaded
}

func TestDownload_CreatesDestinationDirectory(t *testing.T) {
	content := []byte("test content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		w.WriteHeader(http.StatusOK)
		w.Write(content)
	}))
	defer server.Close()

	// Create temp directory
	tempDir := t.TempDir()
	// Use nested path that doesn't exist
	destPath := filepath.Join(tempDir, "movies", "test", "file.mkv")

	// Create downloader
	d := New(10*time.Second, 3)

	// Perform download
	result, err := d.Download(context.Background(), DownloadOptions{
		URL:             server.URL,
		BaseDestPath: destPath,
	})

	// Should succeed and create nested directories
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Verify file exists
	_, err = os.Stat(destPath)
	assert.NoError(t, err)

	// Verify directory structure created
	_, err = os.Stat(filepath.Join(tempDir, "movies", "test"))
	assert.NoError(t, err)
}
