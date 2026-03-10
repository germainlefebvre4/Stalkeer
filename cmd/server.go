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
	"github.com/glefebvre/stalkeer/internal/logger"
	"github.com/glefebvre/stalkeer/internal/shutdown"
	"github.com/spf13/cobra"
)

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

func init() {
	serverCmd.Flags().IntP("port", "p", 8080, "port to run the server on")
	serverCmd.Flags().StringP("address", "a", "0.0.0.0", "address to bind the server to")
	rootCmd.AddCommand(serverCmd)
}
