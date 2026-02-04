package m3udownloader

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glefebvre/stalkeer/internal/config"
	"github.com/glefebvre/stalkeer/internal/logger"
)

// setupTestDownloader creates a test downloader with default config
func setupTestDownloader(t *testing.T) (*Downloader, *config.M3UDownloadConfig) {
	t.Helper()

	cfg := &config.M3UDownloadConfig{
		Enabled:        true,
		ArchiveDir:     t.TempDir(),
		RetentionCount: 5,
		MaxFileSizeMB:  10,
		TimeoutSeconds: 30,
		RetryAttempts:  3,
	}

	log := logger.NewWithLevelAndFormat("info", "text")
	downloader := NewDownloader(cfg, log)

	return downloader, cfg
}

func TestDownload_Success(t *testing.T) {
	downloader, _ := setupTestDownloader(t)

	// Create test M3U content
	m3uContent := `#EXTM3U
#EXTINF:-1,Test Channel
http://example.com/stream.m3u8
`

	// Create mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(m3uContent))
	}))
	defer server.Close()

	// Create temp destination file
	destPath := filepath.Join(t.TempDir(), "playlist.m3u")

	// Download
	ctx := context.Background()
	err := downloader.Download(ctx, server.URL, destPath)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Fatal("Downloaded file does not exist")
	}

	// Verify content
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read downloaded file: %v", err)
	}

	if string(content) != m3uContent {
		t.Errorf("Content mismatch.\nExpected: %s\nGot: %s", m3uContent, string(content))
	}
}

func TestDownload_InvalidM3U(t *testing.T) {
	downloader, _ := setupTestDownloader(t)

	// Create invalid M3U content (missing #EXTM3U header)
	invalidContent := `This is not a valid M3U file`

	// Create mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(invalidContent))
	}))
	defer server.Close()

	// Create temp destination file
	destPath := filepath.Join(t.TempDir(), "playlist.m3u")

	// Download should fail validation
	ctx := context.Background()
	err := downloader.Download(ctx, server.URL, destPath)
	if err == nil {
		t.Fatal("Expected validation error, got nil")
	}

	if !strings.Contains(err.Error(), "invalid M3U") {
		t.Errorf("Expected 'invalid M3U' error, got: %v", err)
	}

	// File should not exist (atomic operation should roll back)
	if _, err := os.Stat(destPath); !os.IsNotExist(err) {
		t.Error("File should not exist after failed validation")
	}
}

func TestDownload_FileSizeExceeded(t *testing.T) {
	downloader, cfg := setupTestDownloader(t)
	cfg.MaxFileSizeMB = 1 // 1 MB limit

	// Create content larger than limit
	largeContent := make([]byte, 2*1024*1024) // 2 MB
	for i := range largeContent {
		largeContent[i] = 'A'
	}
	largeContent = append([]byte("#EXTM3U\n"), largeContent...)

	// Create mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(largeContent)))
		w.WriteHeader(http.StatusOK)
		w.Write(largeContent)
	}))
	defer server.Close()

	// Create temp destination file
	destPath := filepath.Join(t.TempDir(), "playlist.m3u")

	// Download should fail due to size
	ctx := context.Background()
	err := downloader.Download(ctx, server.URL, destPath)
	if err == nil {
		t.Fatal("Expected size limit error, got nil")
	}

	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("Expected size exceeded error, got: %v", err)
	}
}

func TestDownload_HTTPError(t *testing.T) {
	downloader, _ := setupTestDownloader(t)

	// Create mock HTTP server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Create temp destination file
	destPath := filepath.Join(t.TempDir(), "playlist.m3u")

	// Download should fail
	ctx := context.Background()
	err := downloader.Download(ctx, server.URL, destPath)
	if err == nil {
		t.Fatal("Expected HTTP error, got nil")
	}

	if !strings.Contains(err.Error(), "404") {
		t.Errorf("Expected 404 error, got: %v", err)
	}
}

func TestDownload_Timeout(t *testing.T) {
	downloader, cfg := setupTestDownloader(t)
	cfg.TimeoutSeconds = 1 // 1 second timeout
	downloader.httpClient.Timeout = 1 * time.Second

	// Create mock HTTP server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create temp destination file
	destPath := filepath.Join(t.TempDir(), "playlist.m3u")

	// Download should timeout
	ctx := context.Background()
	err := downloader.Download(ctx, server.URL, destPath)
	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}
}

func TestDownload_WithAuthentication(t *testing.T) {
	downloader, cfg := setupTestDownloader(t)
	cfg.AuthUsername = "testuser"
	cfg.AuthPassword = "testpass"

	m3uContent := `#EXTM3U
#EXTINF:-1,Test Channel
http://example.com/stream.m3u8
`

	// Create mock HTTP server that checks auth
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || username != "testuser" || password != "testpass" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(m3uContent))
	}))
	defer server.Close()

	// Recreate downloader with auth config
	log := logger.NewWithLevelAndFormat("info", "text")
	downloader = NewDownloader(cfg, log)

	// Create temp destination file
	destPath := filepath.Join(t.TempDir(), "playlist.m3u")

	// Download should succeed with auth
	ctx := context.Background()
	err := downloader.Download(ctx, server.URL, destPath)
	if err != nil {
		t.Fatalf("Download with auth failed: %v", err)
	}
}

func TestDownloadAndArchive(t *testing.T) {
	downloader, _ := setupTestDownloader(t)

	m3uContent := `#EXTM3U
#EXTINF:-1,Test Channel
http://example.com/stream.m3u8
`

	// Create mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(m3uContent))
	}))
	defer server.Close()

	// Create temp destination file
	destPath := filepath.Join(t.TempDir(), "playlist.m3u")

	// Download and archive
	ctx := context.Background()
	err := downloader.DownloadAndArchive(ctx, server.URL, destPath)
	if err != nil {
		t.Fatalf("DownloadAndArchive failed: %v", err)
	}

	// Verify main file exists
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Fatal("Downloaded file does not exist")
	}

	// Verify archive was created
	archives, err := downloader.GetArchiveManager().ListArchiveFiles()
	if err != nil {
		t.Fatalf("Failed to list archives: %v", err)
	}

	if len(archives) != 1 {
		t.Errorf("Expected 1 archive, got %d", len(archives))
	}
}

func TestValidateM3UContent(t *testing.T) {
	downloader, _ := setupTestDownloader(t)

	tests := []struct {
		name      string
		content   string
		wantError bool
	}{
		{
			name:      "valid M3U",
			content:   "#EXTM3U\n#EXTINF:-1,Channel\nhttp://example.com/stream",
			wantError: false,
		},
		{
			name:      "valid M3U with whitespace",
			content:   "\n\n#EXTM3U\n#EXTINF:-1,Channel\nhttp://example.com/stream",
			wantError: false,
		},
		{
			name:      "invalid - no header",
			content:   "#EXTINF:-1,Channel\nhttp://example.com/stream",
			wantError: true,
		},
		{
			name:      "invalid - empty",
			content:   "",
			wantError: true,
		},
		{
			name:      "invalid - wrong header",
			content:   "#M3U\n#EXTINF:-1,Channel\nhttp://example.com/stream",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := downloader.validateM3UContent([]byte(tt.content))
			if (err != nil) != tt.wantError {
				t.Errorf("validateM3UContent() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestIsValidContentType(t *testing.T) {
	downloader, _ := setupTestDownloader(t)

	tests := []struct {
		contentType string
		valid       bool
	}{
		{"application/vnd.apple.mpegurl", true},
		{"application/x-mpegurl", true},
		{"audio/x-mpegurl", true},
		{"text/plain", true},
		{"application/octet-stream", true},
		{"application/vnd.apple.mpegurl; charset=utf-8", true},
		{"application/json", false},
		{"text/html", false},
	}

	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			result := downloader.isValidContentType(tt.contentType)
			if result != tt.valid {
				t.Errorf("isValidContentType(%q) = %v, want %v", tt.contentType, result, tt.valid)
			}
		})
	}
}
