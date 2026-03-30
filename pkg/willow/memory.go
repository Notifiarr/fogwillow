package willow

import (
	"time"

	"github.com/Notifiarr/fogwillow/pkg/buf"
)

// Len returns the number of file buffers in the map.
func (w *Willow) Len() int {
	return int(w.memLen.Load())
}

// Get a FileBuffer by file name.
func (w *Willow) Get(path string) *buf.FileBuffer {
	w.askCh <- path
	return <-w.repCh
}

// Set a file buffer into memory.
func (w *Willow) Set(buf *buf.FileBuffer) {
	w.setCh <- buf
}

// TrySet atomically stores a new file buffer if the path is not already in memory.
// If the path already exists the existing buffer is returned and the candidate is not stored.
// If the path does not exist nil is returned and the candidate is stored.
func (w *Willow) TrySet(candidate *buf.FileBuffer) *buf.FileBuffer {
	w.tryCh <- candidate
	return <-w.tryRepCh
}

// Delete a file buffer from memory.
func (w *Willow) Delete(path string) {
	w.delCh <- path
}

// memoryHole runs in a single go routine, so keep it lean.
func (w *Willow) memoryHole() {
	groups := time.NewTicker(w.config.GroupInterval.Duration)

	defer func() {
		w.config.Printf("Writing %d+%d files before exit.", len(w.memory), len(w.fsOp))
		w.washer(time.Time{}, true) // Clear out all the files when we exit.
		groups.Stop()
		close(w.repCh) // signal Stop() we are done.
	}()

	for {
		select {
		case buf := <-w.setCh:
			if _, exists := w.memory[buf.Path]; !exists {
				w.memLen.Add(1)
			}

			w.memory[buf.Path] = buf
		case path := <-w.delCh:
			if _, exists := w.memory[path]; exists {
				delete(w.memory, path)
				w.memLen.Add(-1)
			}
		case candidate := <-w.tryCh:
			w.tryRepCh <- w.trySet(candidate)
		case now := <-groups.C:
			w.washer(now, false)
		case path, ok := <-w.askCh:
			if !ok {
				return
			}

			w.repCh <- w.memory[path]
		}
	}
}

// washer is called on an interval by memoryHole().
func (w *Willow) washer(now time.Time, force bool) {
	for path, file := range w.memory {
		if force || now.Sub(file.FirstWrite) >= w.config.FlushInterval.Duration {
			w.Flush(file, buf.FlusOpts{Type: expiredLog})
			delete(w.memory, path)
			w.memLen.Add(-1)
		}
	}
}

// trySet is the internal implementation of TrySet.
// Returns nil when candidate was stored, or the existing buffer when the path was already taken.
func (w *Willow) trySet(candidate *buf.FileBuffer) *buf.FileBuffer {
	if existing, ok := w.memory[candidate.Path]; ok {
		return existing
	}

	w.memory[candidate.Path] = candidate
	w.memLen.Add(1)

	return nil
}

func (w *Willow) stopMemoryHole() {
	defer func() {
		close(w.setCh)
		close(w.delCh)
		close(w.tryCh)
		w.setCh = nil
		w.delCh = nil
		w.askCh = nil
		w.repCh = nil
		w.tryCh = nil
		w.tryRepCh = nil
		w.memory = nil
	}()

	close(w.askCh)
	<-w.repCh
}
