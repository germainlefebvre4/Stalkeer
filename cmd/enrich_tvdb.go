package main

import (
	"fmt"
	"os"

	"github.com/glefebvre/stalkeer/internal/config"
	"github.com/glefebvre/stalkeer/internal/database"
	"github.com/glefebvre/stalkeer/internal/external/tmdb"
	"github.com/glefebvre/stalkeer/internal/processor"
	"github.com/spf13/cobra"
)

var enrichTVDBCmd = &cobra.Command{
	Use:   "enrich-tvdb",
	Short: "Backfill missing TVDB IDs on Movie and TVShow records",
	Long: `Query all Movie and TVShow records that have a TMDB ID but no TVDB ID,
fetch their External IDs from the TMDB API, and update the database.
TVShow records are deduplicated by TMDB ID to minimise API calls.`,
	Run: func(cmd *cobra.Command, args []string) {
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		limit, _ := cmd.Flags().GetInt("limit")
		verbose, _ := cmd.Flags().GetBool("verbose")

		if err := config.Load(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
			os.Exit(1)
		}
		cfg := config.Get()

		if !cfg.TMDB.Enabled || cfg.TMDB.APIKey == "" {
			fmt.Fprintln(os.Stderr, "Error: TMDB integration is disabled or API key is not configured")
			os.Exit(1)
		}

		if err := database.Initialize(); err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing database: %v\n", err)
			os.Exit(1)
		}
		defer database.Close()

		tmdbClient := tmdb.NewClient(tmdb.Config{
			APIKey:            cfg.TMDB.APIKey,
			Language:          cfg.TMDB.Language,
			RequestsPerSecond: cfg.TMDB.RequestsPerSecond,
		})

		db := database.Get()
		opts := processor.EnrichTVDBOptions{
			DryRun:  dryRun,
			Limit:   limit,
			Verbose: verbose,
		}

		if dryRun {
			fmt.Println("Dry-run mode: no database writes will occur")
		}
		fmt.Println("Starting TVDB ID backfill...")

		stats, err := processor.EnrichMissingTVDBIDs(db, tmdbClient, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error during backfill: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("\n=== Backfill Summary ===")
		fmt.Printf("Processed: %d\n", stats.Processed)
		fmt.Printf("Updated:   %d\n", stats.Updated)
		fmt.Printf("Skipped:   %d (no TVDB entry on TMDB)\n", stats.Skipped)
		fmt.Printf("Errors:    %d\n", stats.Errors)
	},
}

func init() {
	enrichTVDBCmd.Flags().Bool("dry-run", false, "preview records that would be updated without writing to database")
	enrichTVDBCmd.Flags().Int("limit", 0, "maximum number of records to process (0 = no limit)")
	enrichTVDBCmd.Flags().BoolP("verbose", "v", false, "verbose output")
	rootCmd.AddCommand(enrichTVDBCmd)
}
