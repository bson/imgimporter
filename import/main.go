package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rwcarlsen/goexif/exif"
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

// Check if path exists and is directory
func DirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsDir()
}

// Check if path exists and is file
func FileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && (info.Mode()&os.ModeType) == 0
}

func GetCreateDate(fname string) (time.Time, error) {
	f, err := os.Open(fname)
	defer f.Close()

	if err != nil {
		return time.Now(), errors.New(fmt.Sprintf("Unable to open file: %s", err.Error()))
	}

	ex, err := exif.Decode(f)
	if err != nil {
		return time.Now(), errors.New(fmt.Sprintf("Unable to decode EXIF: %s", err.Error()))
	}

	t, err := ex.DateTime()
	if err != nil {
		return time.Now(), errors.New(fmt.Sprintf("Unable to obtain EXIF origin time: %s", err.Error()))
	}

	return t, nil
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

/// mediaScanner - the part that scans media and creates a copy work list

type mediaScanner struct {
	worker  Worker
	destDir string
	list    []copyItem
	dirs    sync.Map
}

func (m *mediaScanner) init() {
	m.worker.Init()
}

// Add path to dir if it doesn't exist
func (m *mediaScanner) checkDir(path string) {
	if _, found := m.dirs.Load(path); !found {
		if !DirExists(path) {
			m.dirs.Store(path, true)
		}
	}
}

// Scan media and collect files to copy
func (m *mediaScanner) scan(fileList []string, destDir string, nConc int) ([]copyItem, sync.Map) {

	m.destDir = destDir
	m.dirs = sync.Map{}
	m.list = []copyItem{}

	// Convert []string to []interface{}
	list := make([]interface{}, len(fileList))
	for k, v := range fileList {
		list[k] = v
	}

	m.worker.Work(list, scanConc,
		func(fileList []interface{}, start int, len int) {

			var localCopyList []copyItem

			for i := start; i < start+len; i++ {
				file := fileList[i].(string)
				created, err := GetCreateDate(file)
				if err != nil {
					// No valid EXIF, so not a tagged file format
					continue
				}

				dir := fmt.Sprintf("%s/%04d/%04d-%02d-%02d", m.destDir, created.Year(),
					created.Year(), created.Month(), created.Day())

				ext := strings.ToLower(path.Ext(file))
				if subDir, found := subDirByType[ext]; found {
					dir += "/" + subDir
				}

				to := fmt.Sprintf("%s/%s", dir, path.Base(file))

				if !FileExists(to) {
					toCopy := copyItem{
						from: file,
						to:   to,
					}

					localCopyList = append(localCopyList, toCopy)

					// See if we need to create the parent directories also
					m.checkDir(dir)
				}
			}

			// Finalize by saving results
			m.worker.Finalize(func() {
				m.list = append(m.list, localCopyList...)
			})
		})

	return m.list, m.dirs
}

/// fileCopier - the part that copies files

type fileCopier struct {
	worker Worker

	bytesCopied uint64
	filesCopied uint32
	filesFailed uint32
}

func (f *fileCopier) init() {
	f.worker.Init()
}

func (f *fileCopier) copy(list []copyItem, nConc int) error {
	f.bytesCopied = 0
	f.filesCopied = 0
	f.filesFailed = 0

	workList := make([]interface{}, len(list))
	for k, v := range list {
		workList[k] = v
	}

	f.worker.Work(workList, copyConc,
		func(list []interface{}, start int, len int) {
			for i := start; i < start+len; i++ {
				item := list[i].(copyItem)
				fmt.Println(item.from, item.to)
				fFrom, err := os.Open(item.from)
				if err != nil {
					fmt.Printf("Unable to import %s: %s\n", item.from, err.Error())
					atomic.AddUint32(&f.filesFailed, 1)
					continue
				}
				defer fFrom.Close()

				fTo, err := os.Create(item.to)
				if err != nil {
					fmt.Printf("Unable to import to %s: %s\n", item.to, err.Error())
					atomic.AddUint32(&f.filesFailed, 1)
					continue
				}
				defer fTo.Close()

				nBytes, err := io.Copy(fTo, fFrom)
				if err != nil {
					fmt.Printf("Failed to import %s: %s\n", item.from, err.Error())
					atomic.AddUint32(&f.filesFailed, 1)
					continue
				}

				atomic.AddUint64(&f.bytesCopied, uint64(nBytes))
				atomic.AddUint32(&f.filesCopied, 1)
			}

			f.worker.Finalize(func() {})
		})

	return nil
}

func (f *fileCopier) printStats() {
	if f.filesFailed != 0 {
		fmt.Printf("\n%d files failed to copy due to errors\n", f.filesFailed)
	}

	dur := f.worker.Duration()
	MB := f.bytesCopied / 1024 / 1024
	MBps := float64(MB) / dur.Seconds()

	fmt.Printf("\nCopied %d files/%vMB in %.1fs (%.1fMB/s)\n",
		f.filesCopied, MB, math.Mod(dur.Seconds(), 60), MBps)
}

func main() {
	var scanner mediaScanner

	scanner.init()

	source := fmt.Sprintf("%s/%s", defaultVol, imgDir)
	homeDir, _ := os.UserHomeDir()
	dest := fmt.Sprintf("%s/Pictures", homeDir)

	fmt.Printf("Importing from %s to %s\n", source, dest)

	if len(os.Args) >= 2 {
		source = os.Args[1]
	}
	if len(os.Args) >= 3 {
		dest = os.Args[2]
	}

	var fileList []string
	if err := addFileList(source, &fileList); err != nil {
		fmt.Printf("Failed to scan source file tree: %s\n", err.Error())
		os.Exit(1)
	}

	fmt.Printf("Checking %d files on card\n", len(fileList))

	list, dirs := scanner.scan(fileList, dest, scanConc)

	fmt.Printf("Copying %d new media files\n", len(list))

	// Create directories
	dirs.Range(func(dir interface{}, _ interface{}) bool {
		if err := os.MkdirAll(dir.(string), 0777); err != nil {
			fmt.Printf("Unable to create directory %s: %s\n", dir.(string), err.Error())
			return false
		}
		return true
	})

	var copier fileCopier
	copier.init()

	if err := copier.copy(list, copyConc); err != nil {
		fmt.Printf("File copy failed: %s\n", err.Error())
		os.Exit(1)
	}

	copier.printStats()
}
