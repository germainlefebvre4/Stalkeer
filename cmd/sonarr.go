package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/glefebvre/stalkeer/internal/config"
	"github.com/glefebvre/stalkeer/internal/database"
	"github.com/glefebvre/stalkeer/internal/downloader"
	"github.com/glefebvre/stalkeer/internal/external/sonarr"
	"github.com/glefebvre/stalkeer/internal/logger"
	"github.com/glefebvre/stalkeer/internal/matcher"
	"github.com/glefebvre/stalkeer/internal/models"
	"github.com/glefebvre/stalkeer/internal/retry"
	"github.com/spf13/cobra"
)

var sonarrCmd = &cobra.Command{
	Use:   "sonarr",
	Short: "Download missing TV episodes from Sonarr",
	Long: `Fetch missing TV episodes from Sonarr, match them against the local database using TMDB metadata,
and download matched items from M3U playlist stream URLs.`,
	Run: func(cmd *cobra.Command, args []string) {
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		limit, _ := cmd.Flags().GetInt("limit")
		parallel, _ := cmd.Flags().GetInt("parallel")
		force, _ := cmd.Flags().GetBool("force")
		verbose, _ := cmd.Flags().GetBool("verbose")
		seriesID, _ := cmd.Flags().GetInt("series-id")

		// Load configuration
		if err := config.Load(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
			os.Exit(1)
		}
		cfg := config.Get()

		// Override configuration
		if parallel <= 0 {
			parallel = cfg.Downloads.MaxParallel
		}

		// Initialize loggers
		logger.InitializeLoggers(cfg.GetAppLogLevel(), cfg.GetDatabaseLogLevel())

		// Validate configuration
		if cfg.Sonarr.URL == "" || cfg.Sonarr.APIKey == "" {
			fmt.Fprintln(os.Stderr, "Error: Sonarr URL and API key must be configured")
			os.Exit(1)
		}

		fmt.Println("=== Sonarr Download Command ===")
		if dryRun {
			fmt.Println("Mode: DRY RUN (no downloads will occur)")
		}
		fmt.Printf("Sonarr URL: %s\n", cfg.Sonarr.URL)
		if seriesID > 0 {
			fmt.Printf("Series ID filter: %d\n", seriesID)
		}
		if limit > 0 {
			fmt.Printf("Limit: %d episodes\n", limit)
		}
		fmt.Printf("Parallel downloads: %d\n", parallel)
		fmt.Println()

		// Initialize database
		if err := database.Initialize(); err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing database: %v\n", err)
			os.Exit(1)
		}
		defer database.Close()

		// Create Sonarr client
		sonarrClient := sonarr.New(sonarr.Config{
			BaseURL: cfg.Sonarr.URL,
			APIKey:  cfg.Sonarr.APIKey,
			Timeout: time.Duration(cfg.Downloads.Timeout) * time.Second,
			RetryConfig: retry.Config{
				MaxAttempts:       cfg.Downloads.RetryAttempts,
				InitialBackoff:    2 * time.Second,
				MaxBackoff:        30 * time.Second,
				BackoffMultiplier: 2.0,
				JitterFraction:    0.1,
			},
		})

		// Fetch missing episodes
		fmt.Println("Fetching missing episodes from Sonarr...")
		ctx := context.Background()
		missingEpisodes, err := sonarrClient.GetMissingEpisodes(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching missing episodes: %v\n", err)
			os.Exit(1)
		}

		// Filter by series ID if specified
		if seriesID > 0 {
			filtered := make([]sonarr.Episode, 0)
			for _, ep := range missingEpisodes {
				if ep.SeriesID == seriesID {
					filtered = append(filtered, ep)
				}
			}
			missingEpisodes = filtered
		}

		fmt.Printf("Found %d missing episodes in Sonarr\n\n", len(missingEpisodes))

		if len(missingEpisodes) == 0 {
			fmt.Println("No missing episodes to download!")
			return
		}

		// Apply limit
		if limit > 0 && limit < len(missingEpisodes) {
			missingEpisodes = missingEpisodes[:limit]
			fmt.Printf("Processing first %d episodes\n\n", limit)
		}

		// Match and download
		stats := struct {
			Total      int
			Matched    int
			NotFound   int
			Downloaded int
			Failed     int
			Skipped    int
		}{
			Total: len(missingEpisodes),
		}

		db := database.Get()
		dl := downloader.New(
			time.Duration(cfg.Downloads.Timeout)*time.Second,
			cfg.Downloads.RetryAttempts,
		)

		// We need to fetch series info for each episode
		seriesCache := make(map[int]*sonarr.Series)

		for i, episode := range missingEpisodes {
			// Get series info
			series, ok := seriesCache[episode.SeriesID]
			if !ok {
				s, err := sonarrClient.GetSeriesDetails(ctx, episode.SeriesID)
				if err != nil {
					fmt.Printf("[%d/%d] Error fetching series %d: %v\n", i+1, len(missingEpisodes), episode.SeriesID, err)
					stats.Failed++
					continue
				}
				series = s
				seriesCache[episode.SeriesID] = series
			}

			fmt.Printf("[%d/%d] Processing: %s S%02dE%02d - %s\n",
				i+1, len(missingEpisodes), series.Title, episode.SeasonNumber, episode.EpisodeNumber, episode.Title)

			// Match against database using TVDB ID from Sonarr
			dbShow, _, confidence, err := matcher.MatchTVShowByTVDB(
				db, series.TvdbID, 0, series.Title, episode.SeasonNumber, episode.EpisodeNumber,
			)

			if err != nil {
				if verbose {
					fmt.Printf("  Not found in database (TVDB ID: %d, S%02dE%02d)\n",
						series.TvdbID, episode.SeasonNumber, episode.EpisodeNumber)
				}
				stats.NotFound++
				continue
			}

			fmt.Printf("  Matched: %s S%02dE%02d - Confidence: %d%%\n",
				dbShow.TMDBTitle, *dbShow.Season, *dbShow.Episode, confidence)
			stats.Matched++

			// Check if already downloaded (unless force)
			if !force {
				var downloadedCount int64
				db.Model(&models.ProcessedLine{}).
					Where("tv_show_id = ? AND state = ?", dbShow.ID, models.StateDownloaded).
					Count(&downloadedCount)
				if downloadedCount > 0 {
					if verbose {
						fmt.Println("  Already downloaded (use --force to re-download)")
					}
					stats.Skipped++
					continue
				}
			}

			// Get quality-ordered download candidates
			candidates, err := matcher.FindTVShowDownloadCandidates(db, dbShow.ID)
			if err != nil {
				fmt.Printf("  Failed to get candidates: %v\n", err)
				stats.Failed++
				continue
			}

			if len(candidates) == 0 {
				if verbose {
					fmt.Println("  No stream URL available")
				}
				stats.Skipped++
				continue
			}

			if dryRun {
				c := candidates[0]
				res := "unknown"
				if c.Resolution != nil {
					res = *c.Resolution
				}
				fmt.Printf("  Would download (%s): %s\n", res, valueOrEmpty(c.LineURL))
				stats.Downloaded++
				continue
			}

			// Download - use configured local path + series folder from Sonarr
			baseDestPath := filepath.Join(
				cfg.Downloads.TVShowsPath,
				filepath.Base(series.Path),
				fmt.Sprintf("Season %02d", episode.SeasonNumber),
				fmt.Sprintf("%s - S%02dE%02d", sanitizeFilename(series.Title), episode.SeasonNumber, episode.EpisodeNumber),
			)

			downloaded := false
			for j, candidate := range candidates {
				if candidate.LineURL == nil || *candidate.LineURL == "" {
					continue
				}

				res := "unknown"
				if candidate.Resolution != nil {
					res = *candidate.Resolution
				}
				fmt.Printf("  -> attempt %d/%d (%s): %s\n", j+1, len(candidates), res, *candidate.LineURL)

				var lastUpdate time.Time
				startTime := time.Now()
				result, dlErr := dl.Download(ctx, downloader.DownloadOptions{
					URL:             *candidate.LineURL,
					BaseDestPath:    baseDestPath,
					TempDir:         cfg.Downloads.TempDir,
					ProcessedLineID: candidate.ID,
					OnProgress: func(dlBytes, total int64) {
						if total > 0 {
							now := time.Now()
							if now.Sub(lastUpdate) >= 1*time.Second {
								pct := float64(dlBytes) / float64(total) * 100
								elapsed := now.Sub(startTime)
								speed := float64(dlBytes) / elapsed.Seconds()
								remaining := time.Duration(0)
								if speed > 0 {
									remaining = time.Duration(float64(total-dlBytes)/speed) * time.Second
								}
								fmt.Printf("\r  Progress: %.1f%% - %s / %s - Elapsed: %v - Remaining: %v",
									pct, formatBytes(dlBytes), formatBytes(total),
									elapsed.Round(time.Second), remaining.Round(time.Second))
								lastUpdate = now
							}
						}
					},
				})

				if dlErr != nil {
					fmt.Printf("\n  Download failed: %v\n", dlErr)
					db.Model(&candidate).Update("state", models.StateFailed)
					continue
				}

				fmt.Printf("\n  Downloaded: %s (%.2f MB)\n", result.FilePath, float64(result.FileSize)/(1024*1024))
				downloaded = true
				stats.Downloaded++
				break
			}

			if !downloaded {
				stats.Failed++
			}
		}

		// Print summary
		fmt.Println("\n=== Download Summary ===")
		fmt.Printf("Total episodes:   %d\n", stats.Total)
		fmt.Printf("Matched:          %d\n", stats.Matched)
		fmt.Printf("Not found:        %d\n", stats.NotFound)
		if dryRun {
			fmt.Printf("Would download:   %d\n", stats.Downloaded)
		} else {
			fmt.Printf("Downloaded:       %d\n", stats.Downloaded)
		}
		fmt.Printf("Failed:           %d\n", stats.Failed)
		fmt.Printf("Skipped:          %d\n", stats.Skipped)
	},
}

func init() {
	sonarrCmd.Flags().Bool("dry-run", false, "preview matches without downloading")
	sonarrCmd.Flags().Int("limit", 0, "maximum number of episodes to process (0 = no limit)")
	sonarrCmd.Flags().Int("parallel", 0, "number of concurrent downloads")
	sonarrCmd.Flags().Bool("force", false, "re-download existing files")
	sonarrCmd.Flags().BoolP("verbose", "v", false, "verbose output")
	sonarrCmd.Flags().Int("series-id", 0, "filter to specific Sonarr series ID")
	sonarrCmd.Flags().Bool("resume", false, "resume incomplete downloads before fetching new episodes")
	rootCmd.AddCommand(sonarrCmd)
}
