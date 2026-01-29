package processor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glefebvre/stalkeer/internal/config"
	"github.com/glefebvre/stalkeer/internal/database"
	"github.com/glefebvre/stalkeer/internal/models"
)

func setupTestDB(t *testing.T) {
	t.Helper()

	// Set test configuration
	os.Setenv("DB_HOST", "localhost")
	os.Setenv("DB_PORT", "5432")
	os.Setenv("DB_USER", "postgres")
	os.Setenv("DB_PASSWORD", "postgres")
	os.Setenv("DB_NAME", "stalkeer_test")

	// Load config
	if err := config.Load(); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Initialize database
	if err := database.Initialize(); err != nil {
		t.Fatalf("failed to initialize database: %v", err)
	}

	// Clean up tables
	db := database.Get()
	db.Exec("TRUNCATE TABLE processed_lines, processing_logs, movies, tvshows CASCADE")
}

func teardownTestDB(t *testing.T) {
	t.Helper()
	if err := database.Close(); err != nil {
		t.Errorf("failed to close database: %v", err)
	}
}

func createTestM3U(t *testing.T, content string) string {
	t.Helper()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.m3u")

	err := os.WriteFile(tmpFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	return tmpFile
}

func TestNewProcessor(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	setupTestDB(t)
	defer teardownTestDB(t)

	tmpFile := createTestM3U(t, "#EXTM3U\n#EXTINF:-1,Test\nhttp://example.com/test.mkv")

	proc, err := NewProcessor(tmpFile)
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}

	if proc == nil {
		t.Fatal("processor should not be nil")
	}
	if proc.parser == nil {
		t.Error("parser should not be nil")
	}
	if proc.classifier == nil {
		t.Error("classifier should not be nil")
	}
	if proc.filter == nil {
		t.Error("filter should not be nil")
	}
}

func TestProcessBasic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	setupTestDB(t)
	defer teardownTestDB(t)

	content := `#EXTM3U
#EXTINF:-1 tvg-name="Test Movie" group-title="Movies",Test Movie
http://example.com/movie.mkv
#EXTINF:-1 tvg-name="Another Movie" group-title="Movies",Another Movie
http://example.com/movie2.mp4`

	tmpFile := createTestM3U(t, content)

	proc, err := NewProcessor(tmpFile)
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}

	opts := ProcessOptions{
		Force:            false,
		Limit:            0,
		BatchSize:        10,
		ProgressInterval: 100,
	}

	stats, err := proc.Process(opts)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if stats == nil {
		t.Fatal("stats should not be nil")
	}

	// Verify stats (may be filtered depending on config)
	if stats.TotalLines <= 0 {
		t.Errorf("expected TotalLines > 0, got %d", stats.TotalLines)
	}
}

func TestProcessWithLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	setupTestDB(t)
	defer teardownTestDB(t)

	content := `#EXTM3U
#EXTINF:-1 tvg-name="Movie 1" group-title="Movies",Movie 1
http://example.com/1.mkv
#EXTINF:-1 tvg-name="Movie 2" group-title="Movies",Movie 2
http://example.com/2.mkv
#EXTINF:-1 tvg-name="Movie 3" group-title="Movies",Movie 3
http://example.com/3.mkv`

	tmpFile := createTestM3U(t, content)

	proc, err := NewProcessor(tmpFile)
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}

	opts := ProcessOptions{
		Force:            false,
		Limit:            2,
		BatchSize:        10,
		ProgressInterval: 100,
	}

	stats, err := proc.Process(opts)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Processed count should not exceed limit
	if stats.Processed > opts.Limit {
		t.Errorf("expected Processed <= %d, got %d", opts.Limit, stats.Processed)
	}
}

func TestProcessDuplicates(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	setupTestDB(t)
	defer teardownTestDB(t)

	content := `#EXTM3U
#EXTINF:-1 tvg-name="Test Movie" group-title="Movies",Test Movie
http://example.com/movie.mkv`

	tmpFile := createTestM3U(t, content)

	proc, err := NewProcessor(tmpFile)
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}

	opts := ProcessOptions{
		Force:            false,
		Limit:            0,
		BatchSize:        10,
		ProgressInterval: 100,
	}

	// First processing
	stats1, err := proc.Process(opts)
	if err != nil {
		t.Fatalf("First Process failed: %v", err)
	}

	// Second processing (should detect duplicate)
	stats2, err := proc.Process(opts)
	if err != nil {
		t.Fatalf("Second Process failed: %v", err)
	}

	// Second run should have duplicates (if not filtered)
	if stats1.Processed > 0 && stats2.DuplicatesFound == 0 && stats2.FilteredOut == 0 {
		t.Error("expected duplicates to be detected in second run")
	}
}

func TestProcessWithForce(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	setupTestDB(t)
	defer teardownTestDB(t)

	content := `#EXTM3U
#EXTINF:-1 tvg-name="Test Movie" group-title="Movies",Test Movie
http://example.com/movie.mkv`

	tmpFile := createTestM3U(t, content)

	proc, err := NewProcessor(tmpFile)
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}

	opts := ProcessOptions{
		Force:            true,
		Limit:            0,
		BatchSize:        10,
		ProgressInterval: 100,
	}

	// First processing
	stats1, err := proc.Process(opts)
	if err != nil {
		t.Fatalf("First Process failed: %v", err)
	}

	// Second processing with force (should process again)
	stats2, err := proc.Process(opts)
	if err != nil {
		t.Fatalf("Second Process failed: %v", err)
	}

	// With force, duplicates should not be detected
	if stats2.DuplicatesFound > 0 {
		t.Errorf("expected no duplicates with force flag, got %d", stats2.DuplicatesFound)
	}

	// Both runs should have same processed count (if not filtered)
	if stats1.Processed > 0 && stats2.Processed != stats1.Processed && stats2.FilteredOut == 0 {
		t.Errorf("expected same processed count, got %d and %d", stats1.Processed, stats2.Processed)
	}
}

func TestProcessingLogCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	setupTestDB(t)
	defer teardownTestDB(t)

	content := `#EXTM3U
#EXTINF:-1 tvg-name="Test Movie" group-title="Movies",Test Movie
http://example.com/movie.mkv`

	tmpFile := createTestM3U(t, content)

	proc, err := NewProcessor(tmpFile)
	if err != nil {
		t.Fatalf("NewProcessor failed: %v", err)
	}

	opts := ProcessOptions{
		Force:            false,
		Limit:            0,
		BatchSize:        10,
		ProgressInterval: 100,
	}

	_, err = proc.Process(opts)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Check processing log was created
	db := database.Get()
	var count int64
	db.Model(&models.ProcessingLog{}).Where("action = ?", "process_m3u").Count(&count)
	if count == 0 {
		t.Error("expected processing log to be created")
	}

	// Check log has completed status
	var log models.ProcessingLog
	db.Where("action = ?", "process_m3u").Order("created_at DESC").First(&log)
	if log.Status != "success" && log.Status != "completed_with_errors" {
		t.Errorf("expected status 'success' or 'completed_with_errors', got '%s'", log.Status)
	}
	if log.CompletedAt == nil {
		t.Error("expected completed_at to be set")
	}
}

func TestComputeLineHash(t *testing.T) {
	hash1 := computeLineHash("Test Movie http://example.com/movie.mkv")
	hash2 := computeLineHash("Test Movie http://example.com/movie.mkv")
	hash3 := computeLineHash("Different Movie http://example.com/movie.mkv")

	if hash1 != hash2 {
		t.Error("same content should produce same hash")
	}

	if hash1 == hash3 {
		t.Error("different content should produce different hash")
	}

	if len(hash1) != 64 {
		t.Errorf("expected hash length 64, got %d", len(hash1))
	}
}
