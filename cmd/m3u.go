package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/glefebvre/stalkeer/internal/config"
	"github.com/glefebvre/stalkeer/internal/logger"
	"github.com/glefebvre/stalkeer/internal/m3udownloader"
	"github.com/spf13/cobra"
)

var downloadM3UCmd = &cobra.Command{
	Use:   "m3u-download",
	Short: "Download M3U playlist from remote URL",
	Long: `Download M3U playlist file from the configured URL and save it to the
configured file path. The download is performed atomically and a timestamped
archive copy is created.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Load configuration
		if err := config.Load(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
			os.Exit(1)
		}
		cfg := config.Get()

		// Initialize logger
		logger.InitializeLoggersWithFormat(cfg.GetAppLogLevel(), cfg.GetDatabaseLogLevel(), cfg.Logging.Format)
		log := logger.AppLogger()

		// Get flags
		url, _ := cmd.Flags().GetString("url")
		noArchive, _ := cmd.Flags().GetBool("no-archive")

		// Use URL from flag or config
		if url == "" {
			url = cfg.M3U.Download.URL
		}

		// Validate URL
		if url == "" {
			fmt.Fprintln(os.Stderr, "Error: M3U download URL must be provided via --url flag or m3u.download.url in config")
			os.Exit(1)
		}

		// Validate destination path
		destPath := cfg.M3U.FilePath
		if destPath == "" {
			fmt.Fprintln(os.Stderr, "Error: M3U file path must be configured in m3u.file_path")
			os.Exit(1)
		}

		fmt.Printf("Downloading M3U playlist...\n")
		fmt.Printf("  Source URL:      %s\n", url)
		fmt.Printf("  Destination:     %s\n", destPath)
		if !noArchive {
			fmt.Printf("  Archive dir:     %s\n", cfg.M3U.Download.ArchiveDir)
		}
		fmt.Println()

		// Create downloader
		dl := m3udownloader.NewDownloader(&cfg.M3U.Download, log)

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.M3U.Download.TimeoutSeconds)*time.Second)
		defer cancel()

		// Download
		var err error
		if noArchive {
			err = dl.Download(ctx, url, destPath)
		} else {
			err = dl.DownloadAndArchive(ctx, url, destPath)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "\nError: Download failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("\n✓ M3U playlist downloaded successfully")

		// Show file info
		if info, err := os.Stat(destPath); err == nil {
			fmt.Printf("  Size: %s\n", formatBytes(info.Size()))
			fmt.Printf("  Modified: %s\n", info.ModTime().Format(time.RFC3339))
		}
	},
}

var listM3UArchivesCmd = &cobra.Command{
	Use:   "m3u-list-archives",
	Short: "List archived M3U playlist files",
	Long:  `Display a list of archived M3U playlist files with timestamps and sizes.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Load configuration
		if err := config.Load(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
			os.Exit(1)
		}
		cfg := config.Get()

		// Initialize logger
		logger.InitializeLoggersWithFormat(cfg.GetAppLogLevel(), cfg.GetDatabaseLogLevel(), cfg.Logging.Format)
		log := logger.AppLogger()

		// Create archive manager
		archiveManager := m3udownloader.NewArchiveManager(cfg.M3U.Download.ArchiveDir, log)

		// List archives
		archives, err := archiveManager.ListArchiveFiles()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to list archives: %v\n", err)
			os.Exit(1)
		}

		if len(archives) == 0 {
			fmt.Printf("No archived M3U files found in %s\n", cfg.M3U.Download.ArchiveDir)
			return
		}

		fmt.Printf("Archived M3U files (%s):\n\n", cfg.M3U.Download.ArchiveDir)
		fmt.Printf("%-40s %-12s %s\n", "Filename", "Size", "Modified")
		fmt.Println(strings.Repeat("-", 80))

		for _, archive := range archives {
			fmt.Printf("%-40s %-12s %s\n",
				archive.Name,
				formatBytes(archive.SizeBytes),
				archive.ModTime.Format("2006-01-02 15:04:05"),
			)
		}

		fmt.Printf("\nTotal: %d archived files\n", len(archives))
	},
}

var cleanupM3UArchivesCmd = &cobra.Command{
	Use:   "m3u-cleanup-archives",
	Short: "Clean up old M3U archive files",
	Long:  `Manually trigger rotation of M3U archive files, keeping only the configured retention count.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Load configuration
		if err := config.Load(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
			os.Exit(1)
		}
		cfg := config.Get()

		// Initialize logger
		logger.InitializeLoggersWithFormat(cfg.GetAppLogLevel(), cfg.GetDatabaseLogLevel(), cfg.Logging.Format)
		log := logger.AppLogger()

		// Get retention count from flag or config
		retentionCount, _ := cmd.Flags().GetInt("retention")
		if retentionCount < 0 {
			retentionCount = cfg.M3U.Download.RetentionCount
		}

		// Create archive manager
		archiveManager := m3udownloader.NewArchiveManager(cfg.M3U.Download.ArchiveDir, log)

		// List archives before cleanup
		archives, err := archiveManager.ListArchiveFiles()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to list archives: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Archive directory: %s\n", cfg.M3U.Download.ArchiveDir)
		fmt.Printf("Current files:     %d\n", len(archives))
		fmt.Printf("Retention count:   %d\n", retentionCount)
		fmt.Println()

		if len(archives) <= retentionCount {
			fmt.Println("No cleanup needed - archive count within retention limit")
			return
		}

		toDelete := len(archives) - retentionCount
		fmt.Printf("Files to delete:   %d\n", toDelete)
		fmt.Println()

		// Perform rotation
		if err := archiveManager.RotateArchive(retentionCount); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Cleanup failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("✓ Archive cleanup completed successfully")
	},
}

func init() {
	downloadM3UCmd.Flags().String("url", "", "M3U playlist URL (overrides config)")
	downloadM3UCmd.Flags().Bool("no-archive", false, "skip creating archive copy")

	cleanupM3UArchivesCmd.Flags().Int("retention", -1, "number of archives to keep (default: use config value)")

	rootCmd.AddCommand(downloadM3UCmd)
	rootCmd.AddCommand(listM3UArchivesCmd)
	rootCmd.AddCommand(cleanupM3UArchivesCmd)
}
