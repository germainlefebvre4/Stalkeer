package downloader

import (
	"context"
	"fmt"
	"time"

	"github.com/glefebvre/stalkeer/internal/logger"
	"github.com/glefebvre/stalkeer/internal/models"
)

// ResumeStats holds statistics about resume operations
type ResumeStats struct {
	Total     int
	Resumed   int
	Failed    int
	Skipped   int
	StartTime time.Time
	EndTime   time.Time
}

// Duration returns the total duration of the operation
func (rs *ResumeStats) Duration() time.Duration {
	if rs.EndTime.IsZero() {
		return time.Since(rs.StartTime)
	}
	return rs.EndTime.Sub(rs.StartTime)
}

// ResumeOptions holds options for resuming downloads
type ResumeOptions struct {
	MaxRetries  int
	Limit       int
	Parallel    int
	DryRun      bool
	ContentType *string // Filter by content type (movies, tvshows)
	Verbose     bool
}

// ResumeHelper provides shared functionality for resuming downloads
type ResumeHelper struct {
	stateManager *StateManager
	downloader   *Downloader
}

// NewResumeHelper creates a new resume helper
func NewResumeHelper(stateManager *StateManager, downloader *Downloader) *ResumeHelper {
	return &ResumeHelper{
		stateManager: stateManager,
		downloader:   downloader,
	}
}

// GetIncompleteDownloads retrieves incomplete downloads with optional filtering
func (rh *ResumeHelper) GetIncompleteDownloads(ctx context.Context, opts ResumeOptions) ([]models.DownloadInfo, error) {
	log := logger.AppLogger()

	// Get incomplete downloads from state manager
	downloads, err := rh.stateManager.GetIncompleteDownloads(ctx, opts.MaxRetries, opts.Limit)
	if err != nil {
		return nil, err
	}

	// Filter by content type if specified
	if opts.ContentType != nil {
		var filtered []models.DownloadInfo
		for _, download := range downloads {
			// Would need to check associated ProcessedLine content type
			// For now, we'll skip this filtering
			filtered = append(filtered, download)
		}
		downloads = filtered
	}

	if opts.Verbose {
		log.WithFields(map[string]interface{}{
			"count":       len(downloads),
			"max_retries": opts.MaxRetries,
			"limit":       opts.Limit,
		}).Info("found incomplete downloads")
	}

	return downloads, nil
}

// ResumeDownloads attempts to resume incomplete downloads
func (rh *ResumeHelper) ResumeDownloads(ctx context.Context, opts ResumeOptions) (*ResumeStats, error) {
	log := logger.AppLogger()
	stats := &ResumeStats{
		StartTime: time.Now(),
	}

	// Get incomplete downloads
	downloads, err := rh.GetIncompleteDownloads(ctx, opts)
	if err != nil {
		return stats, err
	}

	stats.Total = len(downloads)

	if stats.Total == 0 {
		log.Info("no incomplete downloads found")
		stats.EndTime = time.Now()
		return stats, nil
	}

	log.WithFields(map[string]interface{}{
		"count":   stats.Total,
		"dry_run": opts.DryRun,
	}).Info("processing incomplete downloads")

	if opts.DryRun {
		// In dry-run mode, just list downloads
		for _, download := range downloads {
			rh.logDownloadInfo(&download, opts.Verbose)
		}
		stats.EndTime = time.Now()
		return stats, nil
	}

	// Process downloads
	// For now, we'll log the downloads that would be processed
	// Full implementation would integrate with parallel downloader
	for _, download := range downloads {
		rh.logDownloadInfo(&download, opts.Verbose)
		stats.Skipped++ // Mark as skipped until full implementation
	}

	stats.EndTime = time.Now()
	return stats, nil
}

// logDownloadInfo logs information about a download
func (rh *ResumeHelper) logDownloadInfo(download *models.DownloadInfo, verbose bool) {
	log := logger.AppLogger()

	fields := map[string]interface{}{
		"id":          download.ID,
		"status":      download.Status,
		"retry_count": download.RetryCount,
	}

	if download.BytesDownloaded != nil && download.TotalBytes != nil {
		progress := float64(*download.BytesDownloaded) / float64(*download.TotalBytes) * 100
		fields["progress"] = fmt.Sprintf("%.1f%%", progress)
		fields["bytes_downloaded"] = *download.BytesDownloaded
		fields["total_bytes"] = *download.TotalBytes
	}

	if download.DownloadPath != nil {
		fields["path"] = *download.DownloadPath
	}

	if download.ErrorMessage != nil && *download.ErrorMessage != "" {
		fields["error"] = *download.ErrorMessage
	}

	if verbose {
		log.WithFields(fields).Info("incomplete download")
	} else {
		log.WithFields(map[string]interface{}{
			"id":     download.ID,
			"status": download.Status,
		}).Debug("incomplete download")
	}
}

// PrintStats prints resume statistics
func (rh *ResumeHelper) PrintStats(stats *ResumeStats) {
	log := logger.AppLogger()

	log.WithFields(map[string]interface{}{
		"total":    stats.Total,
		"resumed":  stats.Resumed,
		"failed":   stats.Failed,
		"skipped":  stats.Skipped,
		"duration": stats.Duration().String(),
	}).Info("resume operation completed")
}
