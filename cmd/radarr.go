package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/glefebvre/stalkeer/internal/config"
	"github.com/glefebvre/stalkeer/internal/database"
	"github.com/glefebvre/stalkeer/internal/downloader"
	"github.com/glefebvre/stalkeer/internal/external/radarr"
	"github.com/glefebvre/stalkeer/internal/logger"
	"github.com/glefebvre/stalkeer/internal/matcher"
	"github.com/glefebvre/stalkeer/internal/models"
	"github.com/glefebvre/stalkeer/internal/retry"
	"github.com/spf13/cobra"
)

var radarrCmd = &cobra.Command{
	Use:   "radarr",
	Short: "Download missing movies from Radarr",
	Long: `Fetch missing movies from Radarr, match them against the local database using TMDB metadata,
and download matched items from M3U playlist stream URLs.`,
	Run: func(cmd *cobra.Command, args []string) {
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		limit, _ := cmd.Flags().GetInt("limit")
		parallel, _ := cmd.Flags().GetInt("parallel")
		force, _ := cmd.Flags().GetBool("force")
		verbose, _ := cmd.Flags().GetBool("verbose")

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
		if cfg.Radarr.URL == "" || cfg.Radarr.APIKey == "" {
			fmt.Fprintln(os.Stderr, "Error: Radarr URL and API key must be configured")
			os.Exit(1)
		}

		fmt.Println("=== Radarr Download Command ===")
		if dryRun {
			fmt.Println("Mode: DRY RUN (no downloads will occur)")
		}
		fmt.Printf("Radarr URL: %s\n", cfg.Radarr.URL)
		if limit > 0 {
			fmt.Printf("Limit: %d movies\n", limit)
		}
		fmt.Printf("Parallel downloads: %d\n", parallel)
		fmt.Println()

		// Initialize database
		if err := database.Initialize(); err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing database: %v\n", err)
			os.Exit(1)
		}
		defer database.Close()

		// Create Radarr client
		radarrClient := radarr.New(radarr.Config{
			BaseURL: cfg.Radarr.URL,
			APIKey:  cfg.Radarr.APIKey,
			Timeout: time.Duration(cfg.Downloads.Timeout) * time.Second,
			RetryConfig: retry.Config{
				MaxAttempts:       cfg.Downloads.RetryAttempts,
				InitialBackoff:    2 * time.Second,
				MaxBackoff:        30 * time.Second,
				BackoffMultiplier: 2.0,
				JitterFraction:    0.1,
			},
		})

		// Fetch missing movies
		fmt.Println("Fetching missing movies from Radarr...")
		ctx := context.Background()
		missingMovies, err := radarrClient.GetMissingMovies(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching missing movies: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Found %d missing movies in Radarr\n\n", len(missingMovies))

		if len(missingMovies) == 0 {
			fmt.Println("No missing movies to download!")
			return
		}

		// Apply limit
		if limit > 0 && limit < len(missingMovies) {
			missingMovies = missingMovies[:limit]
			fmt.Printf("Processing first %d movies\n\n", limit)
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
			Total: len(missingMovies),
		}

		db := database.Get()
		dl := downloader.New(
			time.Duration(cfg.Downloads.Timeout)*time.Second,
			cfg.Downloads.RetryAttempts,
		)

		for i, movie := range missingMovies {
			fmt.Printf("[%d/%d] Processing: %s (%d)\n", i+1, len(missingMovies), movie.Title, movie.Year)

			// Match against database using TVDB ID as primary key, falling back to TMDB ID then fuzzy title/year
			dbMovie, _, confidence, err := matcher.MatchMovieByTVDB(
				db, movie.TvdbID, movie.TMDBID, movie.Title, movie.Year,
			)

			if err != nil {
				if verbose {
					fmt.Printf("  Not found in database (TMDB ID: %d)\n", movie.TMDBID)
				}
				stats.NotFound++
				continue
			}

			fmt.Printf("  Matched: %s (%d) - Confidence: %d%%\n", dbMovie.TMDBTitle, dbMovie.TMDBYear, confidence)
			stats.Matched++

			// Backfill TVDB ID from Radarr if missing in the database
			if movie.TvdbID != 0 && dbMovie.TVDBID == nil {
				tvdbID := movie.TvdbID
				if err := db.Model(&dbMovie).Update("tvdb_id", tvdbID).Error; err == nil {
					dbMovie.TVDBID = &tvdbID
					if verbose {
						fmt.Printf("  Backfilled tvdb_id=%d from Radarr\n", tvdbID)
					}
				}
			}

			// Check if already downloaded (unless force)
			if !force {
				var downloadedCount int64
				db.Model(&models.ProcessedLine{}).
					Where("movie_id = ? AND state = ?", dbMovie.ID, models.StateDownloaded).
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
			candidates, err := matcher.FindMovieDownloadCandidates(db, dbMovie.ID)
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

			// Download - use movie.Path from Radarr as the authoritative root so that
			// movies assigned to secondary root folders land in the correct directory.
			baseDestPath, usedFallback := buildRadarrDestPath(
				movie.Path, cfg.Downloads.MoviesPath, movie.Title, movie.Year,
			)
			if usedFallback {
				fmt.Printf("  Warning: movie.Path is empty for %q, falling back to movies_path\n", movie.Title)
			}

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
								fmt.Printf("\r  Progress: %.1f%% (%d / %d bytes)", pct, dlBytes, total)
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
		fmt.Printf("Total movies:     %d\n", stats.Total)
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
	radarrCmd.Flags().Bool("dry-run", false, "preview matches without downloading")
	radarrCmd.Flags().Int("limit", 0, "maximum number of movies to process (0 = no limit)")
	radarrCmd.Flags().Int("parallel", 0, "number of concurrent downloads")
	radarrCmd.Flags().Bool("force", false, "re-download existing files")
	radarrCmd.Flags().BoolP("verbose", "v", false, "verbose output")
	radarrCmd.Flags().Bool("resume", false, "resume incomplete downloads before fetching new items")
	rootCmd.AddCommand(radarrCmd)
}
