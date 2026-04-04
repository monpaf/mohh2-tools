package extract

import (
	"fmt"
	"mohh2-tools/internal/utils"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func ExtractToGameDir(input string) {
	Extract(input)
	var err = os.Rename(filepath.Join(utils.TempDir, "data"), filepath.Join(utils.GameDir(), strings.TrimSuffix(filepath.Base(input), filepath.Ext(input))))
	if err != nil {
		fmt.Println("Error moving files to game_files directory", err)
		os.Exit(1)
	}
	CleanUp()
	fmt.Println("Files extracted to game_files directory")
}

func Extract(input string) {
	if _, err := os.Stat(filepath.Join("tools", "DolphinTool.exe")); os.IsNotExist(err) {
		utils.DownloadDolphinTool()
	}

	var witTools = []string{"wit.exe", "cygwin1.dll", "cygcrypto-1.1.dll", "cygncursesw-10.dll", "cygz.dll"}
	for _, tool := range witTools {
		if _, err := os.Stat(filepath.Join("tools", tool)); os.IsNotExist(err) {
			utils.DownloadWitTools()
			break
		}
	}

	utils.MakeTempDir()

	if filepath.Ext(input) == ".rvz" {
		fmt.Println("Converting RVZ to ISO...")
		out, err := exec.Command("cmd", "/C", filepath.Join("tools", "DolphinTool.exe"), "convert", "-i", input, "-o", filepath.Join(utils.TempDir, "temp.iso"), "-f", "iso").CombinedOutput()
		if err != nil {
			CleanUp()
			fmt.Println(string(out))
			os.Exit(1)
		}
		extractIso(filepath.Join(utils.TempDir, "temp.iso"))
	} else if filepath.Ext(input) == ".iso" || filepath.Ext(input) == ".wbfs" {
		extractIso(input)
	}
}

func extractIso(input string) {
	fmt.Println("Extracting image...")
	out, err := exec.Command("cmd", "/C", filepath.Join("tools", "wit.exe"), "x", input, filepath.Join(utils.TempDir, "data")).CombinedOutput()
	if err != nil {
		CleanUp()
		fmt.Println(string(out))
		os.Exit(1)
	}
	if _, err := os.Stat(filepath.Join(utils.TempDir, "data")); os.IsNotExist(err) {
		CleanUp()
		fmt.Println("Error extracting image")
		os.Exit(1)
	}
}

func CleanUp() {
	if utils.TempDir != "" {
		os.RemoveAll(utils.TempDir)
	}
}
