package m3udownloader

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glefebvre/stalkeer/internal/logger"
)

func setupTestArchiveManager(t *testing.T) (*ArchiveManager, string) {
	t.Helper()

	archiveDir := t.TempDir()
	log := logger.NewWithLevelAndFormat("info", "text")
	am := NewArchiveManager(archiveDir, log)

	return am, archiveDir
}

func TestArchiveFile(t *testing.T) {
	am, _ := setupTestArchiveManager(t)

	// Create test source file
	sourceDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "test.m3u")
	content := []byte("#EXTM3U\n#EXTINF:-1,Test\nhttp://example.com/stream")

	if err := os.WriteFile(sourcePath, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Archive the file
	archivePath, err := am.ArchiveFile(sourcePath)
	if err != nil {
		t.Fatalf("ArchiveFile failed: %v", err)
	}

	// Verify archive exists
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		t.Error("Archive file does not exist")
	}

	// Verify content matches
	archivedContent, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("Failed to read archived file: %v", err)
	}

	if string(archivedContent) != string(content) {
		t.Error("Archived content does not match source")
	}

	// Verify filename format (playlist_YYYYMMDD_HHMMSS.m3u)
	filename := filepath.Base(archivePath)
	if len(filename) < 20 || filepath.Ext(filename) != ".m3u" {
		t.Errorf("Invalid archive filename format: %s", filename)
	}
}

func TestListArchiveFiles(t *testing.T) {
	am, archiveDir := setupTestArchiveManager(t)

	// Create test archives with different timestamps
	testFiles := []string{
		"playlist_20240101_120000.000000.m3u",
		"playlist_20240102_120000.000000.m3u",
		"playlist_20240103_120000.000000.m3u",
	}

	content := []byte("#EXTM3U\n")
	for i, testFilename := range testFiles {
		path := filepath.Join(archiveDir, testFilename)
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		// Sleep to ensure different modification times
		if i < len(testFiles)-1 {
			time.Sleep(10 * time.Millisecond)
		}
	}

	// List archives
	archives, err := am.ListArchiveFiles()
	if err != nil {
		t.Fatalf("ListArchiveFiles failed: %v", err)
	}

	// Verify count
	if len(archives) != len(testFiles) {
		t.Errorf("Expected %d archives, got %d", len(testFiles), len(archives))
	}

	// Verify sorted by modification time (newest first)
	for i := 0; i < len(archives)-1; i++ {
		if archives[i].ModTime.Before(archives[i+1].ModTime) {
			t.Error("Archives not sorted by modification time (newest first)")
		}
	}
}

func TestListArchiveFiles_EmptyDirectory(t *testing.T) {
	am, _ := setupTestArchiveManager(t)

	archives, err := am.ListArchiveFiles()
	if err != nil {
		t.Fatalf("ListArchiveFiles failed: %v", err)
	}

	if len(archives) != 0 {
		t.Errorf("Expected 0 archives in empty directory, got %d", len(archives))
	}
}

func TestListArchiveFiles_NonExistentDirectory(t *testing.T) {
	log := logger.NewWithLevelAndFormat("info", "text")
	am := NewArchiveManager("/nonexistent/path", log)

	archives, err := am.ListArchiveFiles()
	if err != nil {
		t.Fatalf("ListArchiveFiles should not fail for non-existent directory: %v", err)
	}

	if len(archives) != 0 {
		t.Errorf("Expected 0 archives, got %d", len(archives))
	}
}

func TestRotateArchive(t *testing.T) {
	am, archiveDir := setupTestArchiveManager(t)

	// Create 10 test archives
	content := []byte("#EXTM3U\n")
	for i := 0; i < 10; i++ {
		tempPath := filepath.Join(archiveDir, "temp.m3u")
		if err := os.WriteFile(tempPath, content, 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		// Archive it to get proper timestamp
		if _, err := am.ArchiveFile(tempPath); err != nil {
			t.Fatalf("Failed to archive file: %v", err)
		}
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// Rotate to keep only 5
	retentionCount := 5
	if err := am.RotateArchive(retentionCount); err != nil {
		t.Fatalf("RotateArchive failed: %v", err)
	}

	// Verify only 5 remain
	archives, err := am.ListArchiveFiles()
	if err != nil {
		t.Fatalf("ListArchiveFiles failed: %v", err)
	}

	if len(archives) != retentionCount {
		t.Errorf("Expected %d archives after rotation, got %d", retentionCount, len(archives))
	}
}

func TestRotateArchive_ZeroRetention(t *testing.T) {
	am, archiveDir := setupTestArchiveManager(t)

	// Create 3 test archives
	content := []byte("#EXTM3U\n")
	for i := 0; i < 3; i++ {
		tempPath := filepath.Join(archiveDir, "temp.m3u")
		if err := os.WriteFile(tempPath, content, 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		if _, err := am.ArchiveFile(tempPath); err != nil {
			t.Fatalf("Failed to archive file: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Rotate to keep 0 (delete all)
	if err := am.RotateArchive(0); err != nil {
		t.Fatalf("RotateArchive failed: %v", err)
	}

	// Verify all deleted
	archives, err := am.ListArchiveFiles()
	if err != nil {
		t.Fatalf("ListArchiveFiles failed: %v", err)
	}

	if len(archives) != 0 {
		t.Errorf("Expected 0 archives after rotation with 0 retention, got %d", len(archives))
	}
}

func TestRotateArchive_FewerThanRetention(t *testing.T) {
	am, archiveDir := setupTestArchiveManager(t)

	// Create 3 test archives
	content := []byte("#EXTM3U\n")
	for i := 0; i < 3; i++ {
		tempPath := filepath.Join(archiveDir, "temp.m3u")
		if err := os.WriteFile(tempPath, content, 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		if _, err := am.ArchiveFile(tempPath); err != nil {
			t.Fatalf("Failed to archive file: %v", err)
		}
		os.Remove(tempPath) // Clean up temp file
		time.Sleep(10 * time.Millisecond)
	}

	// Rotate to keep 10 (more than we have)
	if err := am.RotateArchive(10); err != nil {
		t.Fatalf("RotateArchive failed: %v", err)
	}

	// Verify all 3 remain
	archives, err := am.ListArchiveFiles()
	if err != nil {
		t.Fatalf("ListArchiveFiles failed: %v", err)
	}

	if len(archives) != 3 {
		t.Errorf("Expected 3 archives, got %d", len(archives))
	}
}

func TestGetLatestArchive(t *testing.T) {
	am, archiveDir := setupTestArchiveManager(t)

	// Create 3 test archives
	content := []byte("#EXTM3U\n")
	var lastArchivePath string
	for i := 0; i < 3; i++ {
		tempPath := filepath.Join(archiveDir, "temp.m3u")
		if err := os.WriteFile(tempPath, content, 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		archivePath, err := am.ArchiveFile(tempPath)
		if err != nil {
			t.Fatalf("Failed to archive file: %v", err)
		}
		lastArchivePath = archivePath
		time.Sleep(10 * time.Millisecond)
	}

	// Get latest archive
	latest, err := am.GetLatestArchive()
	if err != nil {
		t.Fatalf("GetLatestArchive failed: %v", err)
	}

	if latest.Path != lastArchivePath {
		t.Errorf("Expected latest archive %s, got %s", lastArchivePath, latest.Path)
	}
}

func TestGetLatestArchive_NoArchives(t *testing.T) {
	am, _ := setupTestArchiveManager(t)

	_, err := am.GetLatestArchive()
	if err == nil {
		t.Error("Expected error for no archives, got nil")
	}
}

func TestCleanupArchive(t *testing.T) {
	am, archiveDir := setupTestArchiveManager(t)

	// Create 5 test archives
	content := []byte("#EXTM3U\n")
	for i := 0; i < 5; i++ {
		tempPath := filepath.Join(archiveDir, "temp.m3u")
		if err := os.WriteFile(tempPath, content, 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		if _, err := am.ArchiveFile(tempPath); err != nil {
			t.Fatalf("Failed to archive file: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Cleanup all
	if err := am.CleanupArchive(); err != nil {
		t.Fatalf("CleanupArchive failed: %v", err)
	}

	// Verify all deleted
	archives, err := am.ListArchiveFiles()
	if err != nil {
		t.Fatalf("ListArchiveFiles failed: %v", err)
	}

	if len(archives) != 0 {
		t.Errorf("Expected 0 archives after cleanup, got %d", len(archives))
	}
}

func TestGetArchiveDir(t *testing.T) {
	expectedDir := "/test/archive/dir"
	log := logger.NewWithLevelAndFormat("info", "text")
	am := NewArchiveManager(expectedDir, log)

	if am.GetArchiveDir() != expectedDir {
		t.Errorf("Expected archive dir %s, got %s", expectedDir, am.GetArchiveDir())
	}
}
