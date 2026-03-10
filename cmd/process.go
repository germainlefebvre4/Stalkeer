package main

import (
	"fmt"
	"os"

	"github.com/glefebvre/stalkeer/internal/config"
	"github.com/glefebvre/stalkeer/internal/database"
	"github.com/glefebvre/stalkeer/internal/logger"
	"github.com/glefebvre/stalkeer/internal/processor"
	"github.com/spf13/cobra"
)

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
		} else if tmdbLanguage != "" {
			fmt.Printf("TMDB language: %s\n", tmdbLanguage)
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

func init() {
	processCmd.Flags().Bool("force", false, "re-process existing entries")
	processCmd.Flags().Int("limit", 0, "maximum number of items to process (0 = no limit)")
	processCmd.Flags().Int("batch-size", 100, "batch size for database inserts")
	processCmd.Flags().Int("progress", 1000, "show progress every N entries")
	processCmd.Flags().Bool("skip-tmdb", false, "skip TMDB metadata enrichment")
	processCmd.Flags().String("tmdb-language", "", "TMDB API language (e.g., 'en-US', 'fr-FR')")
	rootCmd.AddCommand(processCmd)
}
