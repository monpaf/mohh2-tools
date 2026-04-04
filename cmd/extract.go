package cmd

import (
	"mohh2-tools/internal/extract"

	"github.com/spf13/cobra"
)

// extractCmd represents the extract command
var extractCmd = &cobra.Command{
	Use:   "extract",
	Short: "Extracts all game data from the disc image (ISO/WBFS/RVZ) into the game_files directory",
	Run: func(cmd *cobra.Command, args []string) {
		extract.ExtractToGameDir(input)
	},
}

func init() {
	imageCmd.AddCommand(extractCmd)
}
