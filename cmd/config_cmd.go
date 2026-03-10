package main

import (
	"fmt"

	"github.com/glefebvre/stalkeer/internal/config"
	"github.com/spf13/cobra"
)

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

func init() {
	configCmd.Flags().Bool("show-secrets", false, "reveal password fields")
	rootCmd.AddCommand(configCmd)
}
