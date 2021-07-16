package main

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bson/imgimporter/workset"

	"github.com/rwcarlsen/goexif/exif"
)

/// mediaScanner - the part that scans media and creates a copy work list

type mediaScanner struct {
	workset.WorkSet
	destDir      string
	list         []copyItem
	dirs         sync.Map
	filesScanned uint32
	newFiles     uint32
}

// Add path to dir if it doesn't exist
func (m *mediaScanner) checkDir(path string) {
	if _, found := m.dirs.Load(path); !found {
		if !DirExists(path) {
			m.dirs.Store(path, true)
		}
	}
}


// because path.Base() doesn't work correctly on Windows
func basename(file string) string {
     s := strings.Split(file, string(filepath.Separator))
     return s[len(s)-1]
}


// Scan media and collect files to copy
func (m *mediaScanner) scan(fileList []string, destDir string, nConc int) ([]copyItem, sync.Map) {

	m.destDir = destDir
	m.dirs = sync.Map{}
	m.list = []copyItem{}
	m.filesScanned = 0
	m.newFiles = 0

	// Convert []string to []interface{}
	list := make([]interface{}, len(fileList))
	for k, v := range fileList {
		list[k] = v
	}

	m.Work(list, scanConc,
		fmt.Sprintf("Scanning %d media files", len(fileList)),
		func(fileList []interface{}, start int, len int) {

			var localCopyList []copyItem

			for i := start; i < start+len; i++ {
				atomic.AddUint32(&m.filesScanned, 1)

				file := filepath.FromSlash(fileList[i].(string))

				// First check using file modification date.  If it already exists, we consider it copied.
				// This is just a quick check to skip previously copied files.  If the file modification date
				// differs from the Exif creation date, then we consider the latter authoritative, so
				// in this peculiar case we experience a slower media scan rate for these particular files.
				fileCreated, err := GetFileModDate(file)
				if _, _, copied := m.alreadyCopied(file, fileCreated); copied {
					continue
				}

				created, err := GetExifCreateDate(file)
				if err != nil {
					// No valid EXIF, so not a tagged file format
					continue
				}

				if dir, to, copied := m.alreadyCopied(file, created); !copied {
					toCopy := copyItem{
						from: file,
						to:   to,
					}

					localCopyList = append(localCopyList, toCopy)

					// See if we need to create the parent directories also
					m.checkDir(dir)
					atomic.AddUint32(&m.newFiles, 1)
				}

				// Update progress, if needed
				m.Progress()
			}

			// Finalize by saving results
			m.Finalize(func() {
				m.list = append(m.list, localCopyList...)
			})
		},
		func() string {
			if m.newFiles != 0 {
				return fmt.Sprintf("%d/%d - %d new - in %.1fs", m.filesScanned, len(fileList),
					m.newFiles, m.Runtime().Seconds())
			} else {
				return fmt.Sprintf("%d/%d in %.1fs", m.filesScanned, len(fileList),
					m.Runtime().Seconds())
			}
		})

	return m.list, m.dirs
}

// Check if file exists given a specific creation date
// Returns destination directory path, destination file name, and whether it exists already
func (m* mediaScanner) alreadyCopied(file string, created time.Time) (string, string, bool) {
	dir := fmt.Sprintf("%s/%04d/%04d-%02d-%02d", m.destDir, created.Year(),
		created.Year(), created.Month(), created.Day())

	ext := strings.ToLower(path.Ext(file))
	if subDir, found := subDirByType[ext]; found {
		dir += "/" + subDir
	}

	to := filepath.FromSlash(fmt.Sprintf("%s/%s", dir, basename(file)))
	dir = filepath.FromSlash(dir)

	return dir, to, FileExists(to);
}

// Get fle creation time of a media file
func GetFileModDate(fname string) (time.Time, error) {
	info, err := os.Stat(fname)
	if err != nil {
		return time.Now(), errors.New(fmt.Sprintf("Unable to stat file: %s", err.Error()))
	}
	return info.ModTime(), nil
}

// Get creation time of a media file from its EXIF info
func GetExifCreateDate(fname string) (time.Time, error) {
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
