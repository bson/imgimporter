package workset

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

const progressInterval = 300 // ms

type WorkFunc func(list []interface{}, start int, len int)
type ProgressFunc func() string

type WorkSet struct {
	Lock    sync.Mutex
	Changed *sync.Cond

	finished int
	started  time.Time

	what     string

	progFunc ProgressFunc
	progress time.Time
	pStr     string
}

// Wait for workers to finish
func (w *WorkSet) wait(n int) {
	w.Lock.Lock()
	for w.finished < n {
		w.Changed.Wait()
	}
	w.Lock.Unlock()
}

// Update status string
func (w *WorkSet) updateStatus(newStatus string) {
	fmt.Print(strings.Repeat("\b", len(w.pStr)))
	fmt.Print(newStatus)
	if len(w.pStr) > len(newStatus) {
		fmt.Print(strings.Repeat(" ", len(w.pStr)-len(newStatus)))
		fmt.Print(strings.Repeat("\b", len(w.pStr)-len(newStatus)))
	}
	w.pStr = newStatus
}

// Launch workers
func (w *WorkSet) Work(list []interface{}, nConc int, what string, workFunc WorkFunc,
	progress ProgressFunc) error {

	w.progFunc = progress

	if w.Changed == nil {
		w.Changed = sync.NewCond(&w.Lock)
	}

	w.started = time.Now()

	chunkStart := 0
	chunkLen := (len(list) + nConc - 1) / nConc

	w.finished = 0

	w.what = what
	fmt.Printf("%s - ", what)
	w.pStr = ""
	w.updateStatus(w.progFunc())
	w.progress = time.Now().Add(time.Duration(progressInterval * time.Millisecond))

	for i := 0; i < nConc; i++ {
		if chunkStart+chunkLen > len(list) {
			chunkLen = len(list) - chunkStart
		}
		go workFunc(list, chunkStart, chunkLen)
		chunkStart += chunkLen
	}

	w.wait(nConc)

	w.updateStatus(w.progFunc())
	fmt.Print("\n")

	return nil
}

// Update progress, if due
func (w *WorkSet) Progress() {
	w.Lock.Lock()
	defer w.Lock.Unlock()

	if time.Now().After(w.progress) {
		w.progress = time.Now().Add(time.Duration(progressInterval * time.Millisecond))
		w.updateStatus(w.progFunc())
	}
}

// Print error message
func (w *WorkSet) Errorf(format string, args ...interface{}) {
	fmt.Print(strings.Repeat("\b", len(w.what) + len(w.pStr) + 3))
	errLen, _ := fmt.Printf(format, args...)
	if remainder := len(w.what) + len(w.pStr) + 3 - errLen; remainder > 0 {
		fmt.Print(strings.Repeat(" ", remainder))
	}
	fmt.Printf("\n%s - %s", w.what, w.pStr)
}

// Finalize work
func (w *WorkSet) Finalize(finalizer func()) {
	w.Lock.Lock()

	finalizer()

	w.finished++
	w.Changed.Signal()
	w.Lock.Unlock()
}

// Return duration of ongoing work
func (w *WorkSet) Runtime() time.Duration {
	return time.Now().Sub(w.started)
}
