package parser

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glefebvre/stalkeer/internal/models"
)

func TestNewParser(t *testing.T) {
	parser := NewParser("test.m3u")
	if parser == nil {
		t.Fatal("NewParser returned nil")
	}
	if parser.filePath != "test.m3u" {
		t.Errorf("expected filePath to be 'test.m3u', got '%s'", parser.filePath)
	}
	if parser.seenHashes == nil {
		t.Error("seenHashes map should be initialized")
	}
	if parser.stats.ErrorsByType == nil {
		t.Error("ErrorsByType map should be initialized")
	}
}

func TestParseValidM3U(t *testing.T) {
	content := `#EXTM3U
#EXTINF:-1 tvg-id="movie1" tvg-name="Test Movie" tvg-logo="http://example.com/logo.jpg" group-title="Movies",Test Movie
http://example.com/movie.mkv
#EXTINF:-1 tvg-id="movie2" tvg-name="Another Movie" tvg-logo="http://example.com/logo2.jpg" group-title="Movies",Another Movie
http://example.com/movie2.mp4`

	tempFile := createTempM3U(t, content)
	defer os.Remove(tempFile)

	parser := NewParser(tempFile)
	lines, err := parser.Parse()

	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}

	// Verify first entry
	if lines[0].TvgName != "Test Movie" {
		t.Errorf("expected TvgName 'Test Movie', got '%s'", lines[0].TvgName)
	}
	if lines[0].GroupTitle != "Movies" {
		t.Errorf("expected GroupTitle 'Movies', got '%s'", lines[0].GroupTitle)
	}
	if *lines[0].LineURL != "http://example.com/movie.mkv" {
		t.Errorf("expected URL 'http://example.com/movie.mkv', got '%s'", *lines[0].LineURL)
	}
	if lines[0].LineHash == "" {
		t.Error("LineHash should not be empty")
	}
	if lines[0].State != models.StatePending {
		t.Errorf("expected State to be 'pending', got '%s'", lines[0].State)
	}
	if lines[0].ContentType != models.ContentTypeUncategorized {
		t.Errorf("expected ContentType to be 'uncategorized', got '%s'", lines[0].ContentType)
	}

	// Verify stats
	stats := parser.GetStats()
	if stats.ParsedEntries != 2 {
		t.Errorf("expected 2 parsed entries, got %d", stats.ParsedEntries)
	}
	if stats.SkippedDuplicates != 0 {
		t.Errorf("expected 0 duplicates, got %d", stats.SkippedDuplicates)
	}
}

func TestParseMissingHeader(t *testing.T) {
	content := `#EXTINF:-1 tvg-name="Test Movie" group-title="Movies",Test Movie
http://example.com/movie.mkv`

	tempFile := createTempM3U(t, content)
	defer os.Remove(tempFile)

	parser := NewParser(tempFile)
	lines, err := parser.Parse()

	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(lines) != 1 {
		t.Errorf("expected 1 line, got %d", len(lines))
	}

	stats := parser.GetStats()
	if stats.ErrorsByType["missing_header"] != 1 {
		t.Errorf("expected 1 missing_header error, got %d", stats.ErrorsByType["missing_header"])
	}
}

func TestParseDuplicates(t *testing.T) {
	content := `#EXTM3U
#EXTINF:-1 tvg-name="Test Movie" group-title="Movies",Test Movie
http://example.com/movie.mkv
#EXTINF:-1 tvg-name="Test Movie" group-title="Movies",Test Movie
http://example.com/movie.mkv
#EXTINF:-1 tvg-name="Different Movie" group-title="Movies",Different Movie
http://example.com/movie.mkv`

	tempFile := createTempM3U(t, content)
	defer os.Remove(tempFile)

	parser := NewParser(tempFile)
	lines, err := parser.Parse()

	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Should have 2 lines: first Test Movie and Different Movie (second Test Movie is duplicate)
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}

	stats := parser.GetStats()
	if stats.SkippedDuplicates != 1 {
		t.Errorf("expected 1 duplicate, got %d", stats.SkippedDuplicates)
	}
	if stats.ParsedEntries != 2 {
		t.Errorf("expected 2 parsed entries, got %d", stats.ParsedEntries)
	}
}

func TestParseMalformedEntries(t *testing.T) {
	content := `#EXTM3U
#EXTINF:-1 tvg-name="Movie Without URL" group-title="Movies",Movie Without URL
#EXTINF:-1 tvg-name="Valid Movie" group-title="Movies",Valid Movie
http://example.com/valid.mkv
http://example.com/orphan_url.mkv
#EXTINF:-1 tvg-name="Another Valid" group-title="Movies",Another Valid
http://example.com/another.mkv`

	tempFile := createTempM3U(t, content)
	defer os.Remove(tempFile)

	parser := NewParser(tempFile)
	lines, err := parser.Parse()

	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Should have 2 valid entries
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}

	stats := parser.GetStats()
	if stats.MalformedEntries < 2 {
		t.Errorf("expected at least 2 malformed entries, got %d", stats.MalformedEntries)
	}
	if stats.ErrorsByType["missing_url"] < 1 {
		t.Errorf("expected at least 1 missing_url error, got %d", stats.ErrorsByType["missing_url"])
	}
	if stats.ErrorsByType["orphan_url"] < 1 {
		t.Errorf("expected at least 1 orphan_url error, got %d", stats.ErrorsByType["orphan_url"])
	}
}

func TestParseUTF8Content(t *testing.T) {
	content := `#EXTM3U
#EXTINF:-1 tvg-name="فيلم عربي" group-title="أفلام",فيلم عربي
http://example.com/arabic.mkv
#EXTINF:-1 tvg-name="电影中文" group-title="电影",电影中文
http://example.com/chinese.mkv
#EXTINF:-1 tvg-name="日本の映画" group-title="映画",日本の映画
http://example.com/japanese.mkv`

	tempFile := createTempM3U(t, content)
	defer os.Remove(tempFile)

	parser := NewParser(tempFile)
	lines, err := parser.Parse()

	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}

	// Verify UTF-8 content is preserved
	if lines[0].TvgName != "فيلم عربي" {
		t.Errorf("Arabic text not preserved: got '%s'", lines[0].TvgName)
	}
	if lines[1].TvgName != "电影中文" {
		t.Errorf("Chinese text not preserved: got '%s'", lines[1].TvgName)
	}
	if lines[2].TvgName != "日本の映画" {
		t.Errorf("Japanese text not preserved: got '%s'", lines[2].TvgName)
	}
}

func TestParseMissingTvgName(t *testing.T) {
	content := `#EXTM3U
#EXTINF:-1 group-title="Movies",Title Only
http://example.com/movie.mkv`

	tempFile := createTempM3U(t, content)
	defer os.Remove(tempFile)

	parser := NewParser(tempFile)
	lines, err := parser.Parse()

	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(lines) != 1 {
		t.Errorf("expected 1 line, got %d", len(lines))
	}

	// Should use title as fallback for tvg-name
	if lines[0].TvgName != "Title Only" {
		t.Errorf("expected TvgName to be 'Title Only', got '%s'", lines[0].TvgName)
	}
}

func TestParseEmptyFile(t *testing.T) {
	content := `#EXTM3U`

	tempFile := createTempM3U(t, content)
	defer os.Remove(tempFile)

	parser := NewParser(tempFile)
	lines, err := parser.Parse()

	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(lines) != 0 {
		t.Errorf("expected 0 lines, got %d", len(lines))
	}

	stats := parser.GetStats()
	if stats.ParsedEntries != 0 {
		t.Errorf("expected 0 parsed entries, got %d", stats.ParsedEntries)
	}
}

func TestParseNonExistentFile(t *testing.T) {
	parser := NewParser("/nonexistent/file.m3u")
	_, err := parser.Parse()

	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestCalculateHash(t *testing.T) {
	parser := NewParser("")

	hash1 := parser.calculateHash("Movie Title", "http://example.com/movie.mkv")
	hash2 := parser.calculateHash("Movie Title", "http://example.com/movie.mkv")
	hash3 := parser.calculateHash("Different Title", "http://example.com/movie.mkv")

	if hash1 != hash2 {
		t.Error("same title+URL should produce same hash")
	}

	if hash1 == hash3 {
		t.Error("different title should produce different hash")
	}

	if len(hash1) != 64 {
		t.Errorf("expected hash length 64, got %d", len(hash1))
	}
}

func TestParseExtinf(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantTvgID string
		wantName  string
		wantLogo  string
		wantGroup string
		wantTitle string
	}{
		{
			name:      "full attributes",
			line:      `#EXTINF:-1 tvg-id="movie1" tvg-name="Test Movie" tvg-logo="http://example.com/logo.jpg" group-title="Movies",Test Movie`,
			wantTvgID: "movie1",
			wantName:  "Test Movie",
			wantLogo:  "http://example.com/logo.jpg",
			wantGroup: "Movies",
			wantTitle: "Test Movie",
		},
		{
			name:      "minimal attributes",
			line:      `#EXTINF:-1,Simple Title`,
			wantTvgID: "",
			wantName:  "Simple Title",
			wantLogo:  "",
			wantGroup: "",
			wantTitle: "Simple Title",
		},
		{
			name:      "no title fallback",
			line:      `#EXTINF:-1 tvg-name="Named Entry" group-title="Group"`,
			wantTvgID: "",
			wantName:  "Named Entry",
			wantLogo:  "",
			wantGroup: "Group",
			wantTitle: "",
		},
	}

	parser := NewParser("")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := parser.parseExtinf(tt.line, 1)

			if entry.TvgID != tt.wantTvgID {
				t.Errorf("TvgID: got '%s', want '%s'", entry.TvgID, tt.wantTvgID)
			}
			if entry.TvgName != tt.wantName {
				t.Errorf("TvgName: got '%s', want '%s'", entry.TvgName, tt.wantName)
			}
			if entry.TvgLogo != tt.wantLogo {
				t.Errorf("TvgLogo: got '%s', want '%s'", entry.TvgLogo, tt.wantLogo)
			}
			if entry.GroupTitle != tt.wantGroup {
				t.Errorf("GroupTitle: got '%s', want '%s'", entry.GroupTitle, tt.wantGroup)
			}
			if entry.Title != tt.wantTitle {
				t.Errorf("Title: got '%s', want '%s'", entry.Title, tt.wantTitle)
			}
		})
	}
}

func TestCreateProcessedLine(t *testing.T) {
	parser := NewParser("")

	entry := &M3UEntry{
		TvgID:      "movie1",
		TvgName:    "Test Movie",
		TvgLogo:    "http://example.com/logo.jpg",
		GroupTitle: "Movies",
		Duration:   "-1",
		Title:      "Test Movie",
		URL:        "http://example.com/movie.mkv",
	}

	line, err := parser.createProcessedLine(entry)
	if err != nil {
		t.Fatalf("createProcessedLine failed: %v", err)
	}

	if line.TvgName != "Test Movie" {
		t.Errorf("TvgName: got '%s', want 'Test Movie'", line.TvgName)
	}
	if line.GroupTitle != "Movies" {
		t.Errorf("GroupTitle: got '%s', want 'Movies'", line.GroupTitle)
	}
	if *line.LineURL != "http://example.com/movie.mkv" {
		t.Errorf("LineURL: got '%s', want 'http://example.com/movie.mkv'", *line.LineURL)
	}
	if line.LineHash == "" {
		t.Error("LineHash should not be empty")
	}
	if line.State != models.StatePending {
		t.Errorf("State: got '%s', want 'pending'", line.State)
	}
	if line.ContentType != models.ContentTypeUncategorized {
		t.Errorf("ContentType: got '%s', want 'uncategorized'", line.ContentType)
	}
}

func TestCreateProcessedLineErrors(t *testing.T) {
	parser := NewParser("")

	tests := []struct {
		name    string
		entry   *M3UEntry
		wantErr bool
	}{
		{
			name:    "nil entry",
			entry:   nil,
			wantErr: true,
		},
		{
			name: "missing tvg-name",
			entry: &M3UEntry{
				URL: "http://example.com/movie.mkv",
			},
			wantErr: true,
		},
		{
			name: "missing URL",
			entry: &M3UEntry{
				TvgName: "Test Movie",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.createProcessedLine(tt.entry)
			if (err != nil) != tt.wantErr {
				t.Errorf("createProcessedLine() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetStats(t *testing.T) {
	parser := NewParser("")
	parser.stats.ParsedEntries = 100
	parser.stats.SkippedDuplicates = 5
	parser.stats.MalformedEntries = 3

	stats := parser.GetStats()
	if stats.ParsedEntries != 100 {
		t.Errorf("ParsedEntries: got %d, want 100", stats.ParsedEntries)
	}
	if stats.SkippedDuplicates != 5 {
		t.Errorf("SkippedDuplicates: got %d, want 5", stats.SkippedDuplicates)
	}
	if stats.MalformedEntries != 3 {
		t.Errorf("MalformedEntries: got %d, want 3", stats.MalformedEntries)
	}
}

func TestParsePerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping performance test in short mode")
	}

	// Use the pre-generated test file
	testFile := "/home/glefebvre/Documents/Dev/Perso/TorrentTracker/stalkeer/m3u_playlist/test_10000_entries.m3u"

	// Check if file exists, if not skip test
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skip("test file not found, run generate_test_files.sh first")
	}

	parser := NewParser(testFile)
	start := time.Now()
	lines, err := parser.Parse()
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	expectedEntries := 10000
	if len(lines) != expectedEntries {
		t.Errorf("expected %d lines, got %d", expectedEntries, len(lines))
	}

	// Should process 10k entries in less than 10 seconds
	if duration > 10*time.Second {
		t.Errorf("parsing took %v, expected < 10s", duration)
	}

	t.Logf("Parsed %d entries in %v (%.0f entries/sec)", len(lines), duration, float64(len(lines))/duration.Seconds())
}

// Helper function to create temporary M3U file for testing
func createTempM3U(t *testing.T, content string) string {
	t.Helper()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.m3u")

	err := os.WriteFile(tmpFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	return tmpFile
}
