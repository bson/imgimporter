package main

import (
	"fmt"
	"io"
	"os"
	"sync/atomic"

	"github.com/bson/imgimporter/workset"
)

/// fileCopier - the part that copies files

type fileCopier struct {
	workset.WorkSet

	bytesCopied uint64
	filesCopied uint32
	filesFailed uint32
}

func (f *fileCopier) copy(list []copyItem, nConc int) error {
	f.bytesCopied = 0
	f.filesCopied = 0
	f.filesFailed = 0

	workList := make([]interface{}, len(list))
	for k, v := range list {
		workList[k] = v
	}

	f.Work(workList, copyConc,
		fmt.Sprintf("Copying %d new media files", len(list)),
		func(list []interface{}, start int, len int) {
			for i := start; i < start+len; i++ {
				item := list[i].(copyItem)
				fFrom, err := os.Open(item.from)
				if err != nil {
					f.Errorf("Unable to import %s: %s", item.from, err.Error())
					atomic.AddUint32(&f.filesFailed, 1)
					continue
				}
				defer fFrom.Close()

				fTo, err := os.Create(item.to)
				if err != nil {
					f.Errorf("Unable to import to %s: %s", item.to, err.Error())
					atomic.AddUint32(&f.filesFailed, 1)
					continue
				}
				defer fTo.Close()

				nBytes, err := io.Copy(fTo, fFrom)
				if err != nil {
					f.Errorf("Failed to import %s: %s", item.from, err.Error())
					atomic.AddUint32(&f.filesFailed, 1)
					continue
				}

				atomic.AddUint64(&f.bytesCopied, uint64(nBytes))
				atomic.AddUint32(&f.filesCopied, 1)

				f.Progress()
			}

			f.Finalize(func() {})
		},
		// Progress
		func() string {
			dur := f.Runtime()
			MB := f.bytesCopied / 1024 / 1024
			MBps := float64(MB) / dur.Seconds()

			return fmt.Sprintf("%d/%d - %vMB in %.1fs (%.1fMB/s)",
				f.filesCopied, len(workList), MB, dur.Seconds(), MBps)
		})

	return nil
}
