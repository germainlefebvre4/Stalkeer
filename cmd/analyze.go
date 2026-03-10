package main

import (
	"fmt"
	"os"

	"github.com/glefebvre/stalkeer/internal/config"
	"github.com/glefebvre/stalkeer/internal/dryrun"
	"github.com/spf13/cobra"
)

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

func init() {
	dryrunCmd.Flags().Int("limit", 100, "maximum number of items to analyze")
	rootCmd.AddCommand(dryrunCmd)
}
