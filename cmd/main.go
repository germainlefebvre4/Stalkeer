package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/glefebvre/stalkeer/internal/api"
	"github.com/glefebvre/stalkeer/internal/config"
	"github.com/glefebvre/stalkeer/internal/database"
	"github.com/glefebvre/stalkeer/internal/dryrun"
	"github.com/glefebvre/stalkeer/internal/logger"
	"github.com/glefebvre/stalkeer/internal/processor"
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

		// Initialize logger
		log := logger.Default()
		log.Info(fmt.Sprintf("Starting Stalkeer API server on %s:%d", address, port))

		// Initialize database
		if err := database.Initialize(); err != nil {
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
		// Determine file path
		var filePath string
		if len(args) > 0 {
			filePath = args[0]
		} else {
			cfg := config.Get()
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

		fmt.Printf("Processing M3U file: %s\n", filePath)
		if force {
			fmt.Println("Force mode: will re-process existing entries")
		}
		if limit > 0 {
			fmt.Printf("Processing limit: %d entries\n", limit)
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
		fmt.Println("Running database migrations...")

		if err := database.Initialize(); err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing database: %v\n", err)
			os.Exit(1)
		}
		defer database.Close()

		fmt.Println("Database migrations completed successfully")
	},
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

	// Dry-run command flags
	dryrunCmd.Flags().Int("limit", 100, "maximum number of items to analyze")

	// Config command flags
	configCmd.Flags().Bool("show-secrets", false, "reveal password fields")

	cobra.OnInitialize(initConfig)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(processCmd)
	rootCmd.AddCommand(dryrunCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(migrateCmd)
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
