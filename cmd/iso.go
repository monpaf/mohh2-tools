package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	input string
)

// imageCmd represents the image command
var imageCmd = &cobra.Command{
	Use:   "image",
	Short: "Extract all game data of ISO/WBFS/RVZ files or apply user defined patches",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if _, err := os.Stat(input); os.IsNotExist(err) {
			fmt.Println("Input file not found")
			return
		}
	},
}

func init() {
	rootCmd.AddCommand(imageCmd)

	imageCmd.PersistentFlags().StringVarP(&input, "input", "i", "", "Input ISO|WBFS|RVZ file")
	imageCmd.MarkPersistentFlagRequired("input")
}
