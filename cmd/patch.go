package cmd

import (
	"fmt"
	"mohh2-tools/internal/extract"
	"mohh2-tools/internal/patch"
	"mohh2-tools/internal/utils"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	output    string
	format    string
	removeAll bool
)

// patchCmd represents the patch command
var patchCmd = &cobra.Command{
	Use:   "patch",
	Short: "Apply patches to the game. Patches are defined in patches.yml",
	Run: func(cmd *cobra.Command, args []string) {
		extract.Extract(input)
		patch.ApplyPatches(cmd, removeAll)

		if format == "iso" || format == "wbfs" {
			patch.PackIso(output)
		} else {
			patch.PackIso(filepath.Join(utils.TempDir, "temp.iso"))
			patch.PackRvz(output)
		}
		extract.CleanUp()
	},
}

func init() {
	imageCmd.AddCommand(patchCmd)

	patchCmd.Flags().StringVarP(&output, "output", "o", "", "Output file")
	patchCmd.MarkFlagRequired("output")
	patchCmd.Flags().StringVarP(&format, "format", "f", "iso", "Output format (iso, wbfs, rvz)")
	patchCmd.Flags().BoolVarP(&removeAll, "remove-all", "r", false, "Remove all patches")

	patch.ReadPatchConfig()

	for _, flag := range patch.PatchCfg.Flags {

		duplicate := false
		if patchCmd.Flags().Lookup(flag.Name) != nil {
			fmt.Println("Duplicate flag name found:", flag.Name)
			duplicate = true
		}

		if duplicate {
			fmt.Println("Please remove the duplicate flags from patches.yml")
			os.Exit(1)
		}

		if flag.Type == "int" {
			patchCmd.Flags().Int(flag.Name, flag.Default, flag.Description)
		} else if flag.Type == "bool" {
			patchCmd.Flags().Bool(flag.Name, false, flag.Description)
		}
	}
}
