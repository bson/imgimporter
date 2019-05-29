package main

import (
	"fmt"
	"io/ioutil"
	"os"
)

const (
	defaultVol = "/Volumes/NIKON Z 7  "
	imgDir     = "DCIM"
	rawSubDir  = "RAWS"
	jpegSubDir = "JPEGS"
	movSubDir  = "MOVIES"
)

// Media scanning and file copy concurrency
const (
	scanConc = 2
	copyConc = 2
)

// Subdir sorting by file type
var subDirByType = map[string]string{
	".nef":  rawSubDir,
	".crw":  rawSubDir,
	".crs":  rawSubDir,
	".dng":  rawSubDir,
	".jpg":  jpegSubDir,
	".jpeg": jpegSubDir,
	".mov":  movSubDir,
}

// Something to copy.  Common to mediaScanner and fileCopier.
type copyItem struct {
	from string
	to   string
}

// Recursively add regular files in dir to list
func addFileList(dir string, list *[]string) error {
	d, err := ioutil.ReadDir(dir)
	if err != nil {
		panic(err)
	}

	for i := range d {
		info := d[i]

		if info.Mode().IsDir() {
			if err := addFileList(fmt.Sprintf("%s/%s", dir, info.Name()), list); err != nil {
				return err
			}
		} else if (info.Mode() & os.ModeType) == 0 {
			*list = append(*list, fmt.Sprintf("%s/%s", dir, info.Name()))
		}
	}

	return nil
}

func main() {
	var scanner mediaScanner

	source := fmt.Sprintf("%s/%s", defaultVol, imgDir)
	homeDir, _ := os.UserHomeDir()
	dest := fmt.Sprintf("%s/Pictures", homeDir)

	if len(os.Args) >= 2 {
		source = os.Args[1]
	}
	if len(os.Args) >= 3 {
		dest = os.Args[2]
	}

	fmt.Printf("Importing from %s to %s\n", source, dest)

	var fileList []string
	if err := addFileList(source, &fileList); err != nil {
		fmt.Printf("Failed to scan source file tree: %s\n", err.Error())
		os.Exit(1)
	}

	if len(fileList) == 0 {
		fmt.Println("No files found.  Nothing to be done here")
		os.Exit(0)
	}

	list, dirs := scanner.scan(fileList, dest, scanConc)

	if len(list) == 0 {
		fmt.Println("Nothing new.  Nothing to be done here")
		os.Exit(0)
	}

	// Create directories
	dirs.Range(func(dir interface{}, _ interface{}) bool {
		if err := os.MkdirAll(dir.(string), 0777); err != nil {
			fmt.Printf("Unable to create directory %s: %s\n", dir.(string), err.Error())
			return false
		}
		return true
	})

	var copier fileCopier

	if err := copier.copy(list, copyConc); err != nil {
		fmt.Printf("File copy failed: %s\n", err.Error())
		os.Exit(1)
	}
}
