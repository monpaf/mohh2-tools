package patch

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"mohh2-tools/internal/utils"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var PatchCfg PatchConfig

func PackRvz(output string) {
	fmt.Println("Building image...")
	if _, err := os.Stat(output); err == nil {
		os.Remove(output)
	}
	out, err := exec.Command("cmd", "/C", filepath.Join("tools", "DolphinTool.exe"), "convert", "-i", filepath.Join(utils.TempDir, "temp.iso"), "-o", output, "-f", "rvz", "-b", "131072", "-c", "zstd", "-l", "5").CombinedOutput()
	if err != nil {
		fmt.Println(string(out))
	}
}

func PackIso(output string) {
	fmt.Println("Building image...")
	out, err := exec.Command("cmd", "/C", filepath.Join("tools", "wit.exe"), "cp", filepath.Join(utils.TempDir, "data"), output, "-o").CombinedOutput()
	if err != nil {
		fmt.Println(string(out))
	}
}

type Region int

const (
	EU_MULTI Region = iota
	EU
	US
	AU
	JP
	US_BETA
	ALL
)

func getRegion(f *os.File, file *File) (Region, error) {
	regions := []*AddressValue{file.EUMulti, file.EU, file.US, file.AU, file.JP, file.USBeta}
	var foundRegion bool
	for i, r := range regions {
		if r != nil {
			foundRegion = true
			if _, err := f.Seek(int64(r.Address), 0); err != nil {
				return 0, fmt.Errorf("Error seeking game.dol")
			}
			value := r.Value
			var result int
			if value <= 0xFF {
				var tmp uint8
				binary.Read(f, binary.BigEndian, &tmp)
				result = int(tmp)
			} else if value <= 0xFFFF {
				var tmp uint16
				binary.Read(f, binary.BigEndian, &tmp)
				result = int(tmp)
			} else {
				var tmp uint32
				binary.Read(f, binary.BigEndian, &tmp)
				result = int(tmp)
			}

			if result == value {
				return Region(i), nil
			}
		}
	}
	if foundRegion {
		return 0, fmt.Errorf("Could not find region")
	}
	return 6, nil
}

func getRegionChanges(changes Change, region Region) (*[]RegionChange, error) {
	var regionChanges *[]RegionChange
	switch region {
	case EU_MULTI:
		regionChanges = changes.EUMulti
	case EU:
		regionChanges = changes.EU
	case US:
		regionChanges = changes.US
	case AU:
		regionChanges = changes.AU
	case JP:
		regionChanges = changes.JP
	case US_BETA:
		regionChanges = changes.USBeta
	case ALL:
		regionChanges = changes.All
	default:
		return nil, fmt.Errorf("Invalid region")
	}

	if regionChanges == nil {
		return nil, fmt.Errorf("No changes for region")
	}

	return regionChanges, nil
}

func applyBoolPatch(patch Patch, value bool) {
	if value {
		fmt.Printf("Applying patch %s\n", patch.Flag.Name)
	} else {
		fmt.Printf("Removing patch %s\n", patch.Flag.Name)
	}

	for _, changes := range patch.Changes {
		if changes.File.Path == "" {
			continue
		}

		filePath := filepath.Join(utils.TempDir, "data", changes.File.Path)
		f, err := os.OpenFile(filePath, os.O_RDWR, 0644)
		if err != nil && (strings.HasPrefix(changes.File.Path, "DATA/") || strings.HasPrefix(changes.File.Path, "DATA\\")) {
			strippedPath := strings.TrimPrefix(strings.TrimPrefix(changes.File.Path, "DATA/"), "DATA\\")
			filePath = filepath.Join(utils.TempDir, "data", strippedPath)
			f, err = os.OpenFile(filePath, os.O_RDWR, 0644)
		}
		if err != nil {
			fmt.Println(err)
			return
		}

		defer f.Close()

		region, err := getRegion(f, changes.File)
		if err != nil {
			fmt.Println(err)
			return
		}

		regionChanges, err := getRegionChanges(changes, region)
		if err != nil {
			fmt.Println(err)
			continue
		}

		for _, change := range *regionChanges {
			if change.Byte != nil {
				replaceByte(f, change, !value)
			} else if change.Bytes != nil {
				replaceBytes(f, change, !value)
			}
		}
	}
}

func applyIntPatch(patch Patch, value int, restore bool) {
	if value < patch.Flag.Min || value > patch.Flag.Max {
		fmt.Println("Value must be between", patch.Flag.Min, "and", patch.Flag.Max)
		return
	}
	if restore {
		fmt.Printf("Removing patch %s\n", patch.Flag.Name)
	} else {
		fmt.Printf("Applying patch %s with value %d\n", patch.Flag.Name, value)
	}

	for _, changes := range patch.Changes {
		if changes.File.Path == "" {
			continue
		}

		filePath := filepath.Join(utils.TempDir, "data", changes.File.Path)
		f, err := os.OpenFile(filePath, os.O_RDWR, 0644)
		if err != nil && (strings.HasPrefix(changes.File.Path, "DATA/") || strings.HasPrefix(changes.File.Path, "DATA\\")) {
			strippedPath := strings.TrimPrefix(strings.TrimPrefix(changes.File.Path, "DATA/"), "DATA\\")
			filePath = filepath.Join(utils.TempDir, "data", strippedPath)
			f, err = os.OpenFile(filePath, os.O_RDWR, 0644)
		}
		if err != nil {
			fmt.Println(err)
			return
		}

		defer f.Close()

		region, err := getRegion(f, changes.File)
		if err != nil {
			fmt.Println(err)
			return
		}

		regionChanges, err := getRegionChanges(changes, region)
		if err != nil {
			fmt.Println(err)
			continue
		}

		for _, change := range *regionChanges {
			if change.Byte == nil && change.Bytes == nil {
				replaceByteWithValue(f, change, byte(value))
			} else {
				if change.Byte != nil {
					replaceByte(f, change, restore)
				} else if change.Bytes != nil {
					replaceBytes(f, change, restore)
				}
			}
		}
	}
}

func ApplyPatches(cmd *cobra.Command, removeAll bool) {
	for _, patch := range PatchCfg.Patches {
		if !removeAll && !cmd.Flags().Lookup(patch.Flag.Name).Changed {
			continue
		}

		if patch.Flag.Type == "bool" {
			if removeAll {
				applyBoolPatch(patch, false)
			} else {
				value, err := cmd.Flags().GetBool(patch.Flag.Name)
				if err != nil {
					fmt.Println(err)
				}
				applyBoolPatch(patch, value)
			}
		} else if patch.Flag.Type == "int" {
			if removeAll {
				applyIntPatch(patch, patch.Flag.Default, true)
			} else {
				value, err := cmd.Flags().GetInt(patch.Flag.Name)
				if err != nil {
					fmt.Println(err)
				}
				applyIntPatch(patch, value, false)
			}
		}

	}
}

type Flag struct {
	Name        string `yaml:"name"`
	Shorthand   string `yaml:"shorthand"`
	Description string `yaml:"description"`
	Type        string `yaml:"type"`
	Default     int    `yaml:"default"`
	Min         int    `yaml:"min"`
	Max         int    `yaml:"max"`
}

type AddressValue struct {
	Address int `yaml:"address"`
	Value   int `yaml:"value"`
}

type File struct {
	Path    string        `yaml:"path"`
	EUMulti *AddressValue `yaml:"eu_multi"`
	EU      *AddressValue `yaml:"eu"`
	US      *AddressValue `yaml:"us"`
	AU      *AddressValue `yaml:"au"`
	JP      *AddressValue `yaml:"jp"`
	USBeta  *AddressValue `yaml:"us_beta"`
}

type ByteChange struct {
	Original    *byte `yaml:"original"`
	Replacement *byte `yaml:"replacement"`
}

type BytesChange struct {
	Original    string  `yaml:"original"`
	Replacement *string `yaml:"replacement"`
}

type RegionChange struct {
	Address int          `yaml:"address"`
	Bytes   *BytesChange `yaml:"bytes"`
	Byte    *ByteChange  `yaml:"byte"`
}

type Change struct {
	File    *File           `yaml:"file"`
	EUMulti *[]RegionChange `yaml:"eu_multi"`
	EU      *[]RegionChange `yaml:"eu"`
	US      *[]RegionChange `yaml:"us"`
	AU      *[]RegionChange `yaml:"au"`
	JP      *[]RegionChange `yaml:"jp"`
	USBeta  *[]RegionChange `yaml:"us_beta"`
	All     *[]RegionChange `yaml:"all"`
}

type Patch struct {
	Flag    *Flag    `yaml:"flag"`
	Changes []Change `yaml:"changes"`
}

type PatchConfig struct {
	Flags   []Flag  `yaml:"flags"`
	Patches []Patch `yaml:"patches"`
}

func ReadPatchConfig() {
	data, err := os.ReadFile("patches.yml")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	err = yaml.Unmarshal(data, &PatchCfg)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func writeAtAddress(file *os.File, address int64, data []byte) {
	_, err := file.Seek(address, 0)
	if err != nil {
		fmt.Println(err)
	}

	_, err = file.Write(data)
	if err != nil {
		fmt.Println(err)
	}
}

func replaceByteWithValue(file *os.File, change RegionChange, replacement byte) {
	writeAtAddress(file, int64(change.Address), []byte{replacement})
}

func replaceByte(file *os.File, change RegionChange, restore bool) {
	if restore {
		writeAtAddress(file, int64(change.Address), []byte{*change.Byte.Original})
	} else {
		writeAtAddress(file, int64(change.Address), []byte{*change.Byte.Replacement})
	}
}

func replaceBytes(file *os.File, change RegionChange, restore bool) {
	var data []byte
	var err error
	if restore {
		data, err = hex.DecodeString(change.Bytes.Original)
	} else {
		data, err = hex.DecodeString(*change.Bytes.Replacement)
	}
	if err != nil {
		fmt.Println(err)
		return
	}
	writeAtAddress(file, int64(change.Address), data)
}
