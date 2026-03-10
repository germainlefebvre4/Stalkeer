package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of Stalkeer",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Stalkeer v0.1.0")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
