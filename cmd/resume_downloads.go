package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/glefebvre/stalkeer/internal/config"
	"github.com/glefebvre/stalkeer/internal/database"
	"github.com/glefebvre/stalkeer/internal/downloader"
	"github.com/glefebvre/stalkeer/internal/logger"
	"github.com/glefebvre/stalkeer/internal/shutdown"
	"github.com/spf13/cobra"
)

var resumeDownloadsCmd = &cobra.Command{
	Use:   "resume-downloads",
	Short: "Resume incomplete or failed downloads",
	Long: `Resume downloads that were interrupted or failed. This command identifies
downloads in pending, downloading, paused, or failed states and attempts to resume them.

The command will:
- Query the database for incomplete downloads
- Clean up stale locks from crashed processes
- Attempt to resume partial downloads where supported
- Retry failed downloads (respecting max retry limits)
- Report progress and statistics

Use --dry-run to preview which downloads would be resumed without actually downloading.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Get flags
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		limit, _ := cmd.Flags().GetInt("limit")
		parallel, _ := cmd.Flags().GetInt("parallel")
		maxRetries, _ := cmd.Flags().GetInt("max-retries")
		cleanStaleLocks, _ := cmd.Flags().GetBool("clean-stale-locks")
		verbose, _ := cmd.Flags().GetBool("verbose")
		service, _ := cmd.Flags().GetString("service")

		// Load configuration
		if err := config.Load(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
			os.Exit(1)
		}
		cfg := config.Get()

		// Initialize loggers with configured levels and format
		logger.InitializeLoggersWithFormat(cfg.GetAppLogLevel(), cfg.GetDatabaseLogLevel(), cfg.Logging.Format)
		log := logger.AppLogger()

		if verbose {
			log.Info("resume-downloads command started")
		}

		// Initialize database
		if err := database.Initialize(); err != nil {
			log.WithFields(map[string]interface{}{
				"error": err,
			}).Error("failed to initialize database", err)
			os.Exit(1)
		}

		// Create shutdown handler
		shutdownHandler := shutdown.New(30 * time.Second)
		ctx := context.Background()

		// Register database cleanup
		shutdownHandler.Register(func(ctx context.Context) error {
			log.Debug("closing database connection")
			return database.Close()
		})

		// Create downloader and state manager
		dl := downloader.New(
			time.Duration(cfg.Downloads.Timeout)*time.Second,
			cfg.Downloads.RetryAttempts,
		)
		stateManager := dl.GetStateManager()

		// Clean up stale locks if requested
		if cleanStaleLocks {
			log.Info("cleaning up stale locks...")
			if err := stateManager.CleanupStaleLocks(ctx); err != nil {
				log.WithFields(map[string]interface{}{
					"error": err,
				}).Error("failed to cleanup stale locks", err)
			}
		}

		// Create resume helper
		helper := downloader.NewResumeHelper(stateManager, dl)

		// Build resume options
		opts := downloader.ResumeOptions{
			MaxRetries: maxRetries,
			Limit:      limit,
			Parallel:   parallel,
			DryRun:     dryRun,
			Verbose:    verbose,
		}

		// Filter by service if specified
		if service != "" && service != "all" {
			normalized, err := normalizeServiceFilter(service)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid service filter: %v\n", err)
				os.Exit(1)
			}
			opts.ContentType = &normalized
		}

		// Resume downloads
		stats, err := helper.ResumeDownloads(ctx, opts)
		if err != nil {
			log.WithFields(map[string]interface{}{
				"error": err,
			}).Error("failed to resume downloads", err)
			os.Exit(1)
		}

		// Print statistics
		helper.PrintStats(stats)

		if dryRun {
			log.Info("dry-run mode - no downloads were performed")
		}

		// Trigger graceful shutdown
		shutdownHandler.Shutdown()

		if verbose {
			log.Info("resume-downloads command completed")
		}
	},
}

func init() {
	resumeDownloadsCmd.Flags().Bool("dry-run", false, "preview downloads without executing")
	resumeDownloadsCmd.Flags().Int("limit", 0, "maximum number of downloads to process (0 = no limit)")
	resumeDownloadsCmd.Flags().Int("parallel", 0, "number of concurrent downloads")
	resumeDownloadsCmd.Flags().Int("max-retries", 0, "maximum retry attempts (downloads exceeding this will be skipped)")
	resumeDownloadsCmd.Flags().Bool("clean-stale-locks", true, "clean up stale download locks before resuming")
	resumeDownloadsCmd.Flags().BoolP("verbose", "v", false, "verbose output")
	resumeDownloadsCmd.Flags().String("service", "all", "filter by service type: all, radarr, sonarr")
}

func normalizeServiceFilter(service string) (string, error) {
	switch strings.ToLower(service) {
	case "radarr":
		return "movies", nil
	case "sonarr":
		return "tvshows", nil
	case "movies", "tvshows":
		return service, nil
	default:
		return "", fmt.Errorf("supported values: all, radarr, sonarr")
	}
}
