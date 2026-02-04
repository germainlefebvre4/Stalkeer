package m3udownloader

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/glefebvre/stalkeer/internal/logger"
)

// ArchiveManager handles M3U file archiving and rotation
type ArchiveManager struct {
	archiveDir string
	logger     *logger.Logger
}

// ArchiveInfo contains information about an archived M3U file
type ArchiveInfo struct {
	Path      string
	Name      string
	ModTime   time.Time
	SizeBytes int64
}

// NewArchiveManager creates a new archive manager
func NewArchiveManager(archiveDir string, log *logger.Logger) *ArchiveManager {
	return &ArchiveManager{
		archiveDir: archiveDir,
		logger:     log,
	}
}

// ArchiveFile creates a timestamped copy of the M3U file in the archive directory
func (am *ArchiveManager) ArchiveFile(sourcePath string) (string, error) {
	// Ensure archive directory exists
	if err := os.MkdirAll(am.archiveDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create archive directory: %w", err)
	}

	// Generate timestamped filename
	timestamp := time.Now().Format("20060102_150405.000000")
	archiveName := fmt.Sprintf("playlist_%s.m3u", timestamp)
	archivePath := filepath.Join(am.archiveDir, archiveName)

	// Copy file to archive
	if err := am.copyFile(sourcePath, archivePath); err != nil {
		return "", fmt.Errorf("failed to copy file to archive: %w", err)
	}

	am.logger.WithFields(map[string]interface{}{
		"source":  sourcePath,
		"archive": archivePath,
	}).Info("M3U file archived")

	return archivePath, nil
}

// copyFile copies a file from src to dst
func (am *ArchiveManager) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	// Sync to ensure data is written
	if err := destFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync destination file: %w", err)
	}

	return nil
}

// ListArchiveFiles returns a list of archived M3U files sorted by modification time (newest first)
func (am *ArchiveManager) ListArchiveFiles() ([]ArchiveInfo, error) {
	// Check if archive directory exists
	if _, err := os.Stat(am.archiveDir); os.IsNotExist(err) {
		return []ArchiveInfo{}, nil
	}

	// Read directory
	entries, err := os.ReadDir(am.archiveDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read archive directory: %w", err)
	}

	var archives []ArchiveInfo
	for _, entry := range entries {
		// Skip directories and non-M3U files
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(strings.ToLower(name), "playlist_") || !strings.HasSuffix(strings.ToLower(name), ".m3u") {
			continue
		}

		// Get file info
		path := filepath.Join(am.archiveDir, name)
		info, err := entry.Info()
		if err != nil {
			am.logger.WithFields(map[string]interface{}{
				"path":  path,
				"error": err,
			}).Warn("Failed to get file info")
			continue
		}

		archives = append(archives, ArchiveInfo{
			Path:      path,
			Name:      name,
			ModTime:   info.ModTime(),
			SizeBytes: info.Size(),
		})
	}

	// Sort by modification time (newest first)
	sort.Slice(archives, func(i, j int) bool {
		return archives[i].ModTime.After(archives[j].ModTime)
	})

	return archives, nil
}

// RotateArchive deletes old archive files beyond the retention count
func (am *ArchiveManager) RotateArchive(retentionCount int) error {
	if retentionCount < 0 {
		return fmt.Errorf("retention count must be non-negative")
	}

	archives, err := am.ListArchiveFiles()
	if err != nil {
		return err
	}

	// If we have fewer archives than the retention count, nothing to delete
	if len(archives) <= retentionCount {
		am.logger.WithFields(map[string]interface{}{
			"archive_count":   len(archives),
			"retention_count": retentionCount,
		}).Debug("No archives to delete")
		return nil
	}

	// Delete archives beyond retention count
	toDelete := archives[retentionCount:]
	var deletionErrors []error

	for _, archive := range toDelete {
		if err := os.Remove(archive.Path); err != nil {
			am.logger.WithFields(map[string]interface{}{
				"path":  archive.Path,
				"error": err,
			}).Warn("Failed to delete archive file")
			deletionErrors = append(deletionErrors, err)
		} else {
			am.logger.WithFields(map[string]interface{}{
				"path": archive.Path,
			}).Info("Deleted old archive file")
		}
	}

	if len(deletionErrors) > 0 {
		return fmt.Errorf("failed to delete %d archive files", len(deletionErrors))
	}

	am.logger.WithFields(map[string]interface{}{
		"deleted_count":   len(toDelete),
		"remaining_count": retentionCount,
	}).Info("Archive rotation completed")

	return nil
}

// GetLatestArchive returns the most recent archived M3U file
func (am *ArchiveManager) GetLatestArchive() (*ArchiveInfo, error) {
	archives, err := am.ListArchiveFiles()
	if err != nil {
		return nil, err
	}

	if len(archives) == 0 {
		return nil, fmt.Errorf("no archived files found")
	}

	// Return the first one (newest)
	return &archives[0], nil
}

// CleanupArchive removes all archived files (use with caution)
func (am *ArchiveManager) CleanupArchive() error {
	archives, err := am.ListArchiveFiles()
	if err != nil {
		return err
	}

	var deletionErrors []error
	for _, archive := range archives {
		if err := os.Remove(archive.Path); err != nil {
			am.logger.WithFields(map[string]interface{}{
				"path":  archive.Path,
				"error": err,
			}).Warn("Failed to delete archive file")
			deletionErrors = append(deletionErrors, err)
		}
	}

	if len(deletionErrors) > 0 {
		return fmt.Errorf("failed to delete %d archive files", len(deletionErrors))
	}

	am.logger.WithFields(map[string]interface{}{
		"deleted_count": len(archives),
	}).Info("Archive cleanup completed")

	return nil
}

// GetArchiveDir returns the archive directory path
func (am *ArchiveManager) GetArchiveDir() string {
	return am.archiveDir
}
