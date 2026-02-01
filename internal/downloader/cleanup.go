package downloader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/glefebvre/stalkeer/internal/logger"
)

const (
	tempDirPrefix         = "stalkeer-download-"
	defaultRetentionHours = 24
)

// CleanupOptions holds configuration for temp directory cleanup
type CleanupOptions struct {
	TempDir        string
	RetentionHours int
	DryRun         bool
}

// CleanupOrphanedTempFiles removes old orphaned temp download directories
func CleanupOrphanedTempFiles(opts CleanupOptions) error {
	log := logger.AppLogger()

	// Use OS temp if not specified
	tempDir := opts.TempDir
	if tempDir == "" {
		tempDir = os.TempDir()
	}

	// Default retention
	if opts.RetentionHours == 0 {
		opts.RetentionHours = defaultRetentionHours
	}

	cutoffTime := time.Now().Add(-time.Duration(opts.RetentionHours) * time.Hour)

	log.Info(fmt.Sprintf("Scanning for orphaned temp files in: %s", tempDir))
	log.Info(fmt.Sprintf("Retention period: %d hours (removing files older than %s)",
		opts.RetentionHours, cutoffTime.Format(time.RFC3339)))

	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return fmt.Errorf("failed to read temp directory: %w", err)
	}

	var removed, skipped int
	for _, entry := range entries {
		// Only process directories matching our pattern
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), tempDirPrefix) {
			continue
		}

		dirPath := filepath.Join(tempDir, entry.Name())

		// Get directory info
		info, err := entry.Info()
		if err != nil {
			log.Warn(fmt.Sprintf("Failed to stat %s: %v", dirPath, err))
			continue
		}

		// Check if older than retention period
		if info.ModTime().After(cutoffTime) {
			skipped++
			continue
		}

		if opts.DryRun {
			log.Info(fmt.Sprintf("[DRY RUN] Would remove: %s (age: %s)",
				dirPath, time.Since(info.ModTime()).Round(time.Hour)))
			removed++
		} else {
			// Remove the directory and all contents
			if err := os.RemoveAll(dirPath); err != nil {
				log.Error(fmt.Sprintf("Failed to remove %s", dirPath), err)
			} else {
				log.Info(fmt.Sprintf("Removed orphaned temp directory: %s (age: %s)",
					dirPath, time.Since(info.ModTime()).Round(time.Hour)))
				removed++
			}
		}
	}

	log.Info(fmt.Sprintf("Cleanup complete: %d removed, %d skipped (too recent)", removed, skipped))
	return nil
}
