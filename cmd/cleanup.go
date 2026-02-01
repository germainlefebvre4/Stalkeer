package main

import (
	"fmt"
	"os"

	"github.com/glefebvre/stalkeer/internal/config"
	"github.com/glefebvre/stalkeer/internal/downloader"
	"github.com/spf13/cobra"
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Clean up orphaned temp download files",
	Long: `Scan the temporary directory and remove orphaned download directories
that are older than the retention period (default: 24 hours).

Orphaned temp files can occur when downloads are interrupted or the application
crashes before completing the move to the final destination.`,
	Run: func(cmd *cobra.Command, args []string) {
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		retentionHours, _ := cmd.Flags().GetInt("retention-hours")

		cfg := config.Get()

		fmt.Println("=== Temp File Cleanup ===")
		if dryRun {
			fmt.Println("Mode: DRY RUN (no files will be deleted)")
		}

		tempDir := cfg.Downloads.TempDir
		if tempDir == "" {
			tempDir = os.TempDir()
		}

		fmt.Printf("Temp directory: %s\n", tempDir)
		fmt.Printf("Retention: %d hours\n\n", retentionHours)

		err := downloader.CleanupOrphanedTempFiles(downloader.CleanupOptions{
			TempDir:        cfg.Downloads.TempDir,
			RetentionHours: retentionHours,
			DryRun:         dryRun,
		})

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error during cleanup: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("\nCleanup complete!")
	},
}
