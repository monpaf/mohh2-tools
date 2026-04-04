package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "mohh2-tools",
	Short: "Tools for modifying Medal of Honor Heroes 2 for the Wii",
	Long: `A collection of tools for modifying Medal of Honor Heroes 2 for the Wii.
	Currently supports extracting .big and .viv files, applying patches to the game, and extracting the game files from the ISO/WBFS/RVZ.`,
}

func Execute() {
	err := rootCmd.Execute()

	if err != nil {
		os.Exit(1)
	}
}
