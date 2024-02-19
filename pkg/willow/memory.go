package willow

import (
	"time"

	"github.com/Notifiarr/fogwillow/pkg/buf"
)

// Len returns the number of file buffers in the map.
func (w *Willow) Len() int {
	return len(w.memory)
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

// Delete a file buffer from memory.
func (w *Willow) Delete(path string) {
	w.delCh <- path
}

// memoryHole runs in a single go routine, so keep it lean.
func (w *Willow) memoryHole() {
	groups := time.NewTicker(w.config.GroupInterval.Duration)

	defer func() {
		w.config.Printf("Writing %d files before exit.", len(w.memory))
		w.washer(time.Time{}, true) // Clear out all the files when we exit.
		groups.Stop()
		close(w.repCh) // signal Stop() we are done.
	}()

	for {
		select {
		case buf := <-w.setCh:
			w.memory[buf.Path] = buf
		case path := <-w.delCh:
			delete(w.memory, path)
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
		}
	}
}

func (w *Willow) stopMemoryHole() {
	defer func() {
		close(w.setCh)
		close(w.delCh)
		w.setCh = nil
		w.delCh = nil
		w.askCh = nil
		w.repCh = nil
		w.memory = nil
	}()

	close(w.askCh)
	<-w.repCh
}
