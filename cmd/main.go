package main

import (
	"fmt"
	"os"

	"github.com/glefebvre/stalkeer/internal/config"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "stalkeer",
	Short: "Stalkeer parses M3U playlists and downloads missing media items",
	Long: `Stalkeer reads M3U playlist files, stores media information in PostgreSQL,
and downloads missing items from Radarr and Sonarr via direct links.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Stalkeer is running...")
		// TODO: Implement main application logic
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of Stalkeer",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Stalkeer v0.1.0")
	},
}

var configFile string

func init() {
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file (default is ./config.yml)")
	cobra.OnInitialize(initConfig)
	rootCmd.AddCommand(versionCmd)
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
