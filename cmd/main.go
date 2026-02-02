package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/glefebvre/stalkeer/internal/api"
	"github.com/glefebvre/stalkeer/internal/config"
	"github.com/glefebvre/stalkeer/internal/database"
	"github.com/glefebvre/stalkeer/internal/downloader"
	"github.com/glefebvre/stalkeer/internal/dryrun"
	"github.com/glefebvre/stalkeer/internal/external/radarr"
	"github.com/glefebvre/stalkeer/internal/external/sonarr"
	"github.com/glefebvre/stalkeer/internal/logger"
	"github.com/glefebvre/stalkeer/internal/matcher"
	"github.com/glefebvre/stalkeer/internal/processor"
	"github.com/glefebvre/stalkeer/internal/retry"
	"github.com/glefebvre/stalkeer/internal/shutdown"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "stalkeer",
	Short: "Stalkeer parses M3U playlists and downloads missing media items",
	Long: `Stalkeer reads M3U playlist files, stores media information in PostgreSQL,
and downloads missing items from Radarr and Sonarr via direct links.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Stalkeer - M3U Playlist Parser and Media Downloader")
		fmt.Println("Run 'stalkeer --help' for usage information")
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of Stalkeer",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Stalkeer v0.1.0")
	},
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the REST API server",
	Long: `Start the REST API server to expose endpoints for querying and managing
parsed M3U playlist data. The server provides endpoints for items, movies, TV shows,
filters, and statistics.`,
	Run: func(cmd *cobra.Command, args []string) {
		port, _ := cmd.Flags().GetInt("port")
		address, _ := cmd.Flags().GetString("address")

		// Load configuration
		if err := config.Load(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
			os.Exit(1)
		}
		cfg := config.Get()

		// Initialize loggers with configured levels and format
		logger.InitializeLoggersWithFormat(cfg.GetAppLogLevel(), cfg.GetDatabaseLogLevel(), cfg.Logging.Format)
		log := logger.AppLogger()

		// Warn about legacy logging configuration
		if cfg.IsUsingLegacyLogging() {
			log.Warn("Using deprecated 'logging.level' configuration. Please migrate to 'logging.app.level' and 'logging.database.level' for better control.")
		}

		log.Info(fmt.Sprintf("Starting Stalkeer API server on %s:%d", address, port))
		log.WithFields(map[string]interface{}{
			"app_log_level": cfg.GetAppLogLevel(),
			"db_log_level":  cfg.GetDatabaseLogLevel(),
			"format":        cfg.Logging.Format,
		}).Info("Logging initialized")

		// Initialize database with retry logic for containerized environments
		log.Info("Connecting to database...")
		if err := database.InitializeWithRetry(5, 3*time.Second); err != nil {
			log.WithFields(map[string]interface{}{
				"error": err,
			}).Error("failed to initialize database", err)
			os.Exit(1)
		}

		log.Info("Database connection established")

		// Create shutdown handler with 30 second timeout
		shutdownHandler := shutdown.New(30 * time.Second)

		// Create and configure server
		server := api.NewServer()

		// Register server shutdown
		shutdownHandler.Register(func(ctx context.Context) error {
			log.Info("Shutting down HTTP server")
			return server.Shutdown(ctx)
		})

		// Register database cleanup
		shutdownHandler.Register(func(ctx context.Context) error {
			log.Info("Closing database connection")
			return database.Close()
		})

		// Start server in goroutine
		serverErr := make(chan error, 1)
		go func() {
			log.Info(fmt.Sprintf("API server listening on http://%s:%d", address, port))
			log.Info(fmt.Sprintf("Health check: http://%s:%d/health", address, port))
			log.Info(fmt.Sprintf("API base URL: http://%s:%d/api/v1", address, port))

			if err := server.Run(port); err != nil && err != http.ErrServerClosed {
				serverErr <- err
			}
		}()

		// Wait for shutdown signal or server error
		select {
		case err := <-serverErr:
			log.WithFields(map[string]interface{}{
				"error": err,
			}).Error("server error", err)
			os.Exit(1)
		case <-time.After(100 * time.Millisecond):
			// Server started successfully, wait for shutdown signal
			shutdownHandler.Wait()
		}

		log.Info("Server shutdown completed")
	},
}

var processCmd = &cobra.Command{
	Use:   "process [m3u-file]",
	Short: "Process M3U file and store to database",
	Long: `Parse M3U playlist file, classify content, and store entries to the database.
This command performs full processing including content type detection and metadata
extraction.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Load configuration
		if err := config.Load(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
			os.Exit(1)
		}
		cfg := config.Get()

		// Initialize loggers with configured levels and format
		logger.InitializeLoggersWithFormat(cfg.GetAppLogLevel(), cfg.GetDatabaseLogLevel(), cfg.Logging.Format)
		log := logger.AppLogger()

		// Warn about legacy logging configuration
		if cfg.IsUsingLegacyLogging() {
			log.Warn("Using deprecated 'logging.level' configuration. Please migrate to 'logging.app.level' and 'logging.database.level' for better control.")
		}

		// Determine file path
		var filePath string
		if len(args) > 0 {
			filePath = args[0]
		} else {
			filePath = cfg.M3U.FilePath
			if filePath == "" {
				fmt.Fprintln(os.Stderr, "Error: m3u file path must be provided either as CLI argument or in config file")
				os.Exit(1)
			}
		}

		// Check if file exists
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error: file '%s' does not exist\n", filePath)
			os.Exit(1)
		}

		force, _ := cmd.Flags().GetBool("force")
		limit, _ := cmd.Flags().GetInt("limit")
		batchSize, _ := cmd.Flags().GetInt("batch-size")
		progress, _ := cmd.Flags().GetInt("progress")
		skipTMDB, _ := cmd.Flags().GetBool("skip-tmdb")
		tmdbLanguage, _ := cmd.Flags().GetString("tmdb-language")

		fmt.Printf("Processing M3U file: %s\n", filePath)
		if force {
			fmt.Println("Force mode: will re-process existing entries")
		}
		if limit > 0 {
			fmt.Printf("Processing limit: %d entries\n", limit)
		}
		if skipTMDB {
			fmt.Println("TMDB enrichment: disabled")
		} else {
			if tmdbLanguage != "" {
				fmt.Printf("TMDB language: %s\n", tmdbLanguage)
			}
		}
		fmt.Println()

		// Initialize database
		if err := database.Initialize(); err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing database: %v\n", err)
			os.Exit(1)
		}
		defer database.Close()

		// Create processor
		proc, err := processor.NewProcessor(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating processor: %v\n", err)
			os.Exit(1)
		}

		// Process the file
		opts := processor.ProcessOptions{
			Force:            force,
			Limit:            limit,
			BatchSize:        batchSize,
			ProgressInterval: progress,
			SkipTMDB:         skipTMDB,
			TMDBLanguage:     tmdbLanguage,
		}

		stats, err := proc.Process(opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error processing file: %v\n", err)
			os.Exit(1)
		}

		// Display statistics
		fmt.Printf("\n=== Processing Complete ===\n")
		fmt.Printf("Total lines in file:  %d\n", stats.TotalLines)
		fmt.Printf("Successfully processed: %d\n", stats.Processed)
		fmt.Printf("Duplicates skipped:   %d\n", stats.DuplicatesFound)
		fmt.Printf("Filtered out:         %d\n", stats.FilteredOut)
		fmt.Printf("Errors:               %d\n", stats.Errors)
		fmt.Printf("\nContent breakdown:\n")
		fmt.Printf("  Movies:        %d\n", stats.Movies)
		fmt.Printf("  TV Shows:      %d\n", stats.TVShows)
		fmt.Printf("  Channels:      %d\n", stats.Channels)
		fmt.Printf("  Uncategorized: %d\n", stats.Uncategorized)

		if !skipTMDB {
			fmt.Printf("\nTMDB Enrichment:\n")
			fmt.Printf("  Matched:       %d\n", stats.TMDBMatched)
			fmt.Printf("  Not found:     %d\n", stats.TMDBNotFound)
			fmt.Printf("  Errors:        %d\n", stats.TMDBErrors)
			if stats.TMDBMatched+stats.TMDBNotFound > 0 {
				matchRate := float64(stats.TMDBMatched) / float64(stats.TMDBMatched+stats.TMDBNotFound) * 100
				fmt.Printf("  Match rate:    %.1f%%\n", matchRate)
			}
		}

		fmt.Printf("\nProcessing time: %v\n", stats.Duration)

		if stats.Errors > 0 {
			fmt.Printf("\nErrors encountered:\n")
			for i, msg := range stats.ErrorMessages {
				if i >= 10 {
					fmt.Printf("  ... and %d more errors\n", len(stats.ErrorMessages)-10)
					break
				}
				fmt.Printf("  - %s\n", msg)
			}
		}

		fmt.Println("\nProcessing completed successfully!")
	},
}

var dryrunCmd = &cobra.Command{
	Use:   "dryrun [m3u-file]",
	Short: "Execute dry-run analysis without database changes",
	Long: `Analyze M3U playlist file and identify potential issues without making
database changes. Useful for validating content before full processing.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var filePath string
		if len(args) > 0 {
			filePath = args[0]
		} else {
			cfg := config.Get()
			filePath = cfg.M3U.FilePath
			if filePath == "" {
				fmt.Fprintln(os.Stderr, "Error: m3u file path must be provided")
				os.Exit(1)
			}
		}

		limit, _ := cmd.Flags().GetInt("limit")

		fmt.Printf("Dry-run analysis of: %s\n", filePath)
		if limit > 0 {
			fmt.Printf("Analysis limit: %d entries\n", limit)
		}

		// Create analyzer and run analysis
		analyzer := dryrun.NewAnalyzer(limit)
		result, err := analyzer.Analyze(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error during dry-run analysis: %v\n", err)
			os.Exit(1)
		}

		// Print summary
		dryrun.PrintSummary(result)

		fmt.Println("\nDry-run analysis completed!")
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Validate and display current configuration",
	Long:  `Display the current configuration settings loaded from config.yml`,
	Run: func(cmd *cobra.Command, args []string) {
		showSecrets, _ := cmd.Flags().GetBool("show-secrets")

		cfg := config.Get()
		fmt.Println("=== Stalkeer Configuration ===")
		fmt.Printf("Database Host: %s\n", cfg.Database.Host)
		fmt.Printf("Database Port: %d\n", cfg.Database.Port)
		fmt.Printf("Database Name: %s\n", cfg.Database.DBName)
		fmt.Printf("Database User: %s\n", cfg.Database.User)
		if showSecrets {
			fmt.Printf("Database Password: %s\n", cfg.Database.Password)
		} else {
			fmt.Printf("Database Password: ********\n")
		}
		fmt.Printf("Database SSL Mode: %s\n", cfg.Database.SSLMode)
		fmt.Printf("\nM3U File Path: %s\n", cfg.M3U.FilePath)
		fmt.Printf("\nLogging Level: %s\n", cfg.Logging.Level)
		fmt.Printf("Logging Format: %s\n", cfg.Logging.Format)
	},
}

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Run database migrations",
	Long:  `Initialize or update database schema to the latest version`,
	Run: func(cmd *cobra.Command, args []string) {
		// Load configuration
		if err := config.Load(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
			os.Exit(1)
		}
		cfg := config.Get()

		// Initialize loggers
		logger.InitializeLoggers(cfg.GetAppLogLevel(), cfg.GetDatabaseLogLevel())

		fmt.Println("Running database migrations...")

		if err := database.Initialize(); err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing database: %v\n", err)
			os.Exit(1)
		}
		defer database.Close()

		fmt.Println("Database migrations completed successfully")
	},
}

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

			// Match against database - try TVDB first, then TMDB
			// Note: Radarr doesn't provide TVDB ID, so we rely on TMDB ID and database TVDB storage
			dbMovie, processedLine, confidence, err := matcher.MatchMovieByTVDB(
				db, 0, movie.TMDBID, movie.Title, movie.Year,
			)

			if err != nil {
				if verbose {
					fmt.Printf("  ❌ Not found in database (TMDB ID: %d)\n", movie.TMDBID)
				}
				stats.NotFound++
				continue
			}

			fmt.Printf("  ✓ Matched: %s (%d) - Confidence: %d%%\n", dbMovie.TMDBTitle, dbMovie.TMDBYear, confidence)
			stats.Matched++

			if processedLine.LineURL == nil || *processedLine.LineURL == "" {
				if verbose {
					fmt.Println("  ⚠ No stream URL available")
				}
				stats.Skipped++
				continue
			}

			// Check if already downloaded (unless force)
			if !force && processedLine.State == "downloaded" {
				if verbose {
					fmt.Println("  ⏭ Already downloaded (use --force to re-download)")
				}
				stats.Skipped++
				continue
			}

			if dryRun {
				fmt.Printf("  🔍 Would download from: %s\n", *processedLine.LineURL)
				stats.Downloaded++
				continue
			}

			// Download
			baseDestPath := filepath.Join(
				cfg.Downloads.MoviesPath,
				fmt.Sprintf("%s (%d)", sanitizeFilename(movie.Title), movie.Year),
				fmt.Sprintf("%s (%d)", sanitizeFilename(movie.Title), movie.Year),
			)
			fileExt := filepath.Ext(*processedLine.LineURL)

			if dryRun {
				fmt.Printf("  🔍 Would download to: %s%s\n", baseDestPath, fileExt)
				stats.Downloaded++
				continue
			}

			fmt.Printf("  ⬇️  Downloading to: %s%s\n", baseDestPath, fileExt)

			var lastUpdate time.Time
			result, err := dl.Download(ctx, downloader.DownloadOptions{
				URL:             *processedLine.LineURL,
				BaseDestPath:    baseDestPath,
				TempDir:         cfg.Downloads.TempDir,
				ProcessedLineID: processedLine.ID,
				OnProgress: func(downloaded, total int64) {
					if total > 0 {
						now := time.Now()
						if now.Sub(lastUpdate) >= 1*time.Second {
							pct := float64(downloaded) / float64(total) * 100
							fmt.Printf("\r  Progress: %.1f%% (%d / %d bytes)", pct, downloaded, total)
							lastUpdate = now
						}
					}
				},
			})

			if err != nil {
				fmt.Printf("\n  ❌ Download failed: %v\n", err)
				stats.Failed++
				continue
			}

			fmt.Printf("\n  ✅ Downloaded: %s (%.2f MB)\n", result.FilePath, float64(result.FileSize)/(1024*1024))
			stats.Downloaded++
		}
		if dryRun {
			fmt.Printf("Would download:   %d\n", stats.Downloaded)
		} else {
			fmt.Printf("Downloaded:       %d\n", stats.Downloaded)
		}
		fmt.Printf("Failed:           %d\n", stats.Failed)
		fmt.Printf("Skipped:          %d\n", stats.Skipped)
	},
}

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
			dbShow, processedLine, confidence, err := matcher.MatchTVShowByTVDB(
				db, series.TvdbID, 0, series.Title, episode.SeasonNumber, episode.EpisodeNumber,
			)

			if err != nil {
				if verbose {
					fmt.Printf("  ❌ Not found in database (TVDB ID: %d, S%02dE%02d)\n",
						series.TvdbID, episode.SeasonNumber, episode.EpisodeNumber)
				}
				stats.NotFound++
				continue
			}

			fmt.Printf("  ✓ Matched: %s S%02dE%02d - Confidence: %d%%\n",
				dbShow.TMDBTitle, *dbShow.Season, *dbShow.Episode, confidence)
			stats.Matched++

			if processedLine.LineURL == nil || *processedLine.LineURL == "" {
				if verbose {
					fmt.Println("  ⚠ No stream URL available")
				}
				stats.Skipped++
				continue
			}

			// Check if already downloaded (unless force)
			if !force && processedLine.State == "downloaded" {
				if verbose {
					fmt.Println("  ⏭ Already downloaded (use --force to re-download)")
				}
				stats.Skipped++
				continue
			}

			if dryRun {
				fmt.Printf("  🔍 Would download from: %s\n", *processedLine.LineURL)
				stats.Downloaded++
				continue
			}

			// Download
			baseDestPath := filepath.Join(
				cfg.Downloads.TVShowsPath,
				fmt.Sprintf("%s (%d)", sanitizeFilename(series.Title), series.Year),
				fmt.Sprintf("Season %01d", episode.SeasonNumber),
				fmt.Sprintf("%s (%d) - S%02dE%02d", sanitizeFilename(series.Title), series.Year, episode.SeasonNumber, episode.EpisodeNumber),
			)
			fileExt := filepath.Ext(*processedLine.LineURL)

			if dryRun {
				fmt.Printf("  🔍 Would download to: %s%s\n", baseDestPath, fileExt)
				stats.Downloaded++
				continue
			}

			fmt.Printf("  ⬇️  Downloading to: %s%s\n", baseDestPath, fileExt)

			var lastUpdate time.Time
			startTime := time.Now()
			result, err := dl.Download(ctx, downloader.DownloadOptions{
				URL:             *processedLine.LineURL,
				BaseDestPath:    baseDestPath,
				TempDir:         cfg.Downloads.TempDir,
				ProcessedLineID: processedLine.ID,
				OnProgress: func(downloaded, total int64) {
					if total > 0 {
						now := time.Now()
						if now.Sub(lastUpdate) >= 1*time.Second {
							pct := float64(downloaded) / float64(total) * 100
							elapsed := now.Sub(startTime)
							speed := float64(downloaded) / elapsed.Seconds()
							remaining := time.Duration(0)
							if speed > 0 {
								remaining = time.Duration(float64(total-downloaded)/speed) * time.Second
							}
							fmt.Printf("\r  Progress: %.1f%% - %s / %s - Elapsed: %v - Remaining: %v",
								pct, formatBytes(downloaded), formatBytes(total),
								elapsed.Round(time.Second), remaining.Round(time.Second))
							lastUpdate = now
						}
					}
				},
			})

			if err != nil {
				fmt.Printf("\n  ❌ Download failed: %v\n", err)
				stats.Failed++
				continue
			}

			fmt.Printf("\n  ✅ Downloaded: %s (%.2f MB)\n", result.FilePath, float64(result.FileSize)/(1024*1024))
			stats.Downloaded++
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

// formatBytes converts bytes to human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// sanitizeFilename removes invalid characters from filenames
func sanitizeFilename(name string) string {
	// Replace invalid filesystem characters with underscores
	replacer := map[rune]rune{
		'/':  '_',
		'\\': '_',
		':':  '_',
		'*':  '_',
		'?':  '_',
		'"':  '_',
		'<':  '_',
		'>':  '_',
		'|':  '_',
	}

	result := []rune(name)
	for i, r := range result {
		if replacement, ok := replacer[r]; ok {
			result[i] = replacement
		}
	}
	return string(result)
}

var configFile string

func init() {
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file (default is ./config.yml)")

	// Server command flags
	serverCmd.Flags().IntP("port", "p", 8080, "port to run the server on")
	serverCmd.Flags().StringP("address", "a", "0.0.0.0", "address to bind the server to")

	// Process command flags
	processCmd.Flags().Bool("force", false, "re-process existing entries")
	processCmd.Flags().Int("limit", 0, "maximum number of items to process (0 = no limit)")
	processCmd.Flags().Int("batch-size", 100, "batch size for database inserts")
	processCmd.Flags().Int("progress", 1000, "show progress every N entries")
	processCmd.Flags().Bool("skip-tmdb", false, "skip TMDB metadata enrichment")
	processCmd.Flags().String("tmdb-language", "", "TMDB API language (e.g., 'en-US', 'fr-FR')")

	// Dry-run command flags
	dryrunCmd.Flags().Int("limit", 100, "maximum number of items to analyze")

	// Config command flags
	configCmd.Flags().Bool("show-secrets", false, "reveal password fields")

	// Radarr command flags
	radarrCmd.Flags().Bool("dry-run", false, "preview matches without downloading")
	radarrCmd.Flags().Int("limit", 0, "maximum number of movies to process (0 = no limit)")
	radarrCmd.Flags().Int("parallel", 0, "number of concurrent downloads")
	radarrCmd.Flags().Bool("force", false, "re-download existing files")
	radarrCmd.Flags().BoolP("verbose", "v", false, "verbose output")
	radarrCmd.Flags().Bool("resume", false, "resume incomplete downloads before fetching new items")

	// Sonarr command flags
	sonarrCmd.Flags().Bool("dry-run", false, "preview matches without downloading")
	sonarrCmd.Flags().Int("limit", 0, "maximum number of episodes to process (0 = no limit)")
	sonarrCmd.Flags().Int("parallel", 0, "number of concurrent downloads")
	sonarrCmd.Flags().Bool("force", false, "re-download existing files")
	sonarrCmd.Flags().BoolP("verbose", "v", false, "verbose output")
	sonarrCmd.Flags().Int("series-id", 0, "filter to specific Sonarr series ID")
	sonarrCmd.Flags().Bool("resume", false, "resume incomplete downloads before fetching new episodes")

	// Cleanup command flags
	cleanupCmd.Flags().Bool("dry-run", false, "preview cleanup without deleting files")
	cleanupCmd.Flags().Int("retention-hours", 24, "delete temp files older than this many hours")

	cobra.OnInitialize(initConfig)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(processCmd)
	rootCmd.AddCommand(dryrunCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(radarrCmd)
	rootCmd.AddCommand(sonarrCmd)
	rootCmd.AddCommand(resumeDownloadsCmd)
	rootCmd.AddCommand(cleanupCmd)
}

func initConfig() {
	// Skip config loading for version command
	if len(os.Args) > 1 && os.Args[1] == "version" {
		return
	}

	if err := config.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
