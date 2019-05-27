package main

import (
	"sync"
	"time"
)

type WorkFunc func(list []interface{}, start int, len int)

type Worker struct {
	Lock    sync.Mutex
	Changed *sync.Cond

	finished int
	started  time.Time
	ended    time.Time
}

func (w *Worker) Init() {
	w.Changed = sync.NewCond(&w.Lock)
}

// Wait for workers to finish
func (w *Worker) wait(n int) {
	w.Lock.Lock()
	for w.finished < n {
		w.Changed.Wait()
	}
	w.Lock.Unlock()
}

// Launch workers
func (w *Worker) Work(list []interface{}, nConc int, workFunc WorkFunc) error {
	w.started = time.Now()

	chunkStart := 0
	chunkLen := (len(list) + nConc - 1) / nConc

	w.finished = 0

	for i := 0; i < nConc; i++ {
		if chunkStart+chunkLen > len(list) {
			chunkLen = len(list) - chunkStart
		}
		go workFunc(list, chunkStart, chunkLen)
		chunkStart += chunkLen
	}

	w.wait(nConc)

	w.ended = time.Now()
	return nil
}

// Finalize work
func (w *Worker) Finalize(finalizer func()) {
	w.Lock.Lock()

	finalizer()

	w.finished++
	w.Changed.Signal()
	w.Lock.Unlock()
}

// Return duration of work
func (w *Worker) Duration() time.Duration {
	return w.ended.Sub(w.started)
}
