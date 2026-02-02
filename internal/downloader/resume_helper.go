package downloader

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/glefebvre/stalkeer/internal/config"
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
		normalized := normalizeContentType(*opts.ContentType)
		if normalized != "" {
			var filtered []models.DownloadInfo
			for _, download := range downloads {
				if hasContentType(&download, normalized) {
					filtered = append(filtered, download)
				}
			}
			downloads = filtered
		}
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

	cfg := config.Get()
	parallel := opts.Parallel
	if parallel <= 0 {
		parallel = cfg.Downloads.MaxParallel
	}
	if parallel <= 0 {
		parallel = 3
	}

	if opts.DryRun {
		for _, download := range downloads {
			if err := rh.logDownloadPlan(&download, cfg, opts); err != nil {
				stats.Skipped++
			}
		}
		stats.EndTime = time.Now()
		return stats, nil
	}

	jobs, jobInfo, skipped := rh.buildDownloadJobs(downloads, cfg, opts)
	stats.Skipped += skipped
	if len(jobs) == 0 {
		log.Info("no eligible downloads to resume")
		stats.EndTime = time.Now()
		return stats, nil
	}

	log.WithFields(map[string]interface{}{
		"count":       len(jobs),
		"parallel":    parallel,
		"skipped":     stats.Skipped,
		"contentType": opts.ContentType,
	}).Info("starting resume downloads")

	parallelDownloader := NewParallelWithDownloader(rh.downloader, parallel)
	results := parallelDownloader.DownloadBatch(ctx, jobs)

	for result := range results {
		info, ok := jobInfo[result.JobID]
		if !ok {
			stats.Failed++
			log.WithFields(map[string]interface{}{
				"job_id": result.JobID,
			}).Warn("received result for unknown job")
			continue
		}

		if result.Error != nil {
			stats.Failed++
			log.WithFields(map[string]interface{}{
				"download_id": info.downloadID,
				"title":       info.displayName,
				"error":       result.Error,
			}).Error("resume download failed", result.Error)
			continue
		}

		stats.Resumed++
		log.WithFields(map[string]interface{}{
			"download_id": info.downloadID,
			"title":       info.displayName,
			"file_path":   result.Result.FilePath,
			"file_size":   result.Result.FileSize,
		}).Info("resume download completed")
	}

	stats.EndTime = time.Now()
	return stats, nil
}

type resumeJobInfo struct {
	downloadID  uint
	displayName string
}

func (rh *ResumeHelper) buildDownloadJobs(downloads []models.DownloadInfo, cfg *config.Config, opts ResumeOptions) ([]DownloadJob, map[int]resumeJobInfo, int) {
	log := logger.AppLogger()
	jobs := make([]DownloadJob, 0, len(downloads))
	jobInfo := make(map[int]resumeJobInfo, len(downloads))
	skipped := 0

	for _, download := range downloads {
		processedLine := selectProcessedLine(&download, opts.ContentType)
		if processedLine == nil {
			statsMessage := "skipping download without matching processed line"
			if opts.Verbose {
				log.WithFields(map[string]interface{}{
					"download_id": download.ID,
					"status":      download.Status,
				}).Warn(statsMessage)
			}
			skipped++
			continue
		}

		if processedLine.LineURL == nil || *processedLine.LineURL == "" {
			if opts.Verbose {
				log.WithFields(map[string]interface{}{
					"download_id": download.ID,
					"line_id":     processedLine.ID,
				}).Warn("skipping download with empty URL")
			}
			skipped++
			continue
		}

		baseDestPath, displayName, err := rh.buildBaseDestPath(cfg, processedLine, &download)
		if err != nil {
			if opts.Verbose {
				log.WithFields(map[string]interface{}{
					"download_id": download.ID,
					"line_id":     processedLine.ID,
					"error":       err,
				}).Warn("skipping download due to path build error")
			}
			skipped++
			continue
		}

		jobID := len(jobs) + 1
		jobs = append(jobs, DownloadJob{
			ID: jobID,
			Options: DownloadOptions{
				URL:             *processedLine.LineURL,
				BaseDestPath:    baseDestPath,
				TempDir:         cfg.Downloads.TempDir,
				ProcessedLineID: processedLine.ID,
				OnProgress:      rh.buildProgressLogger(download.ID, displayName, opts.Verbose),
			},
		})
		jobInfo[jobID] = resumeJobInfo{
			downloadID:  download.ID,
			displayName: displayName,
		}
	}

	return jobs, jobInfo, skipped
}

func (rh *ResumeHelper) buildBaseDestPath(cfg *config.Config, line *models.ProcessedLine, download *models.DownloadInfo) (string, string, error) {
	if line.ContentType == models.ContentTypeMovies {
		if line.Movie != nil {
			path := buildMovieBasePath(cfg.Downloads.MoviesPath, line.Movie.TMDBTitle, line.Movie.TMDBYear)
			return path, fmt.Sprintf("%s (%d)", line.Movie.TMDBTitle, line.Movie.TMDBYear), nil
		}
	}

	if line.ContentType == models.ContentTypeTVShows {
		if line.TVShow != nil && line.TVShow.Season != nil && line.TVShow.Episode != nil {
			path := buildTVShowBasePath(cfg.Downloads.TVShowsPath, line.TVShow.TMDBTitle, line.TVShow.TMDBYear, *line.TVShow.Season, *line.TVShow.Episode)
			return path, fmt.Sprintf("%s (%d) - S%02dE%02d", line.TVShow.TMDBTitle, line.TVShow.TMDBYear, *line.TVShow.Season, *line.TVShow.Episode), nil
		}
	}

	if download.DownloadPath != nil && *download.DownloadPath != "" {
		return strings.TrimSuffix(*download.DownloadPath, filepath.Ext(*download.DownloadPath)), filepath.Base(*download.DownloadPath), nil
	}

	return "", "", fmt.Errorf("missing metadata for destination path")
}

func (rh *ResumeHelper) buildProgressLogger(downloadID uint, displayName string, verbose bool) func(int64, int64) {
	if !verbose {
		return nil
	}

	log := logger.AppLogger()
	var lastUpdate time.Time
	return func(downloaded, total int64) {
		if total <= 0 {
			return
		}
		now := time.Now()
		if now.Sub(lastUpdate) < 2*time.Second {
			return
		}
		lastUpdate = now
		progress := float64(downloaded) / float64(total) * 100
		log.WithFields(map[string]interface{}{
			"download_id": downloadID,
			"title":       displayName,
			"progress":    fmt.Sprintf("%.1f%%", progress),
			"downloaded":  downloaded,
			"total":       total,
		}).Info("resume download progress")
	}
}

func (rh *ResumeHelper) logDownloadPlan(download *models.DownloadInfo, cfg *config.Config, opts ResumeOptions) error {
	log := logger.AppLogger()
	processedLine := selectProcessedLine(download, opts.ContentType)
	if processedLine == nil {
		if opts.Verbose {
			log.WithFields(map[string]interface{}{
				"download_id": download.ID,
			}).Warn("no processed line available for download")
		}
		return fmt.Errorf("no processed line")
	}

	baseDestPath, displayName, err := rh.buildBaseDestPath(cfg, processedLine, download)
	if err != nil {
		if opts.Verbose {
			log.WithFields(map[string]interface{}{
				"download_id": download.ID,
				"error":       err,
			}).Warn("unable to build destination path")
		}
		return err
	}

	fields := map[string]interface{}{
		"download_id": download.ID,
		"status":      download.Status,
		"title":       displayName,
		"url":         valueOrEmpty(processedLine.LineURL),
		"dest":        baseDestPath,
	}
	if download.BytesDownloaded != nil && download.TotalBytes != nil {
		progress := float64(*download.BytesDownloaded) / float64(*download.TotalBytes) * 100
		fields["progress"] = fmt.Sprintf("%.1f%%", progress)
	}

	log.WithFields(fields).Info("resume download candidate")
	return nil
}

func selectProcessedLine(download *models.DownloadInfo, contentType *string) *models.ProcessedLine {
	if download == nil || len(download.ProcessedLines) == 0 {
		return nil
	}

	normalized := ""
	if contentType != nil {
		normalized = normalizeContentType(*contentType)
	}

	for i := range download.ProcessedLines {
		line := &download.ProcessedLines[i]
		if normalized != "" && string(line.ContentType) != normalized {
			continue
		}
		return line
	}

	return nil
}

func hasContentType(download *models.DownloadInfo, contentType string) bool {
	if download == nil || contentType == "" {
		return true
	}
	for i := range download.ProcessedLines {
		if string(download.ProcessedLines[i].ContentType) == contentType {
			return true
		}
	}
	return false
}

func normalizeContentType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "movies", "movie", "radarr":
		return string(models.ContentTypeMovies)
	case "tvshows", "tvshow", "sonarr", "tv":
		return string(models.ContentTypeTVShows)
	default:
		return ""
	}
}

func valueOrEmpty(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
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
