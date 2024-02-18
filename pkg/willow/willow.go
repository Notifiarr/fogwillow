// Package willow provides a memory buffer to flush files to disk.
package willow

import (
	"time"

	"github.com/Notifiarr/fogwillow/pkg/buf"
	"golift.io/cnfg"
)

// Config is the input data needed to make a Willow.
type Config struct {
	// How often to scan entire memory map for flushable files.
	GroupInterval cnfg.Duration `toml:"group_interval" xml:"group_interval"`
	// How old a file must be to flush it away.
	FlushInterval cnfg.Duration `toml:"flush_interval" xml:"flush_interval"`
}

// Willow is the working struct for this module. Get one from, NeWillow().
type Willow struct {
	config *Config
	memory map[string]*buf.FileBuffer // The file buffer memory map.
	delCh  chan string                // This channel is used by Delete().
	askCh  chan string                // This channel it used by Get().
	repCh  chan *buf.FileBuffer       // This is the response channel for Get().
	setCh  chan *buf.FileBuffer       // This channel is used by Set().
}

// initial allocation for the file buffer map.
const memoryMapSize = 300

// NeWillow returns a new, configured Willow pointer.
func NeWillow(config *Config) *Willow {
	return &Willow{
		config: config,
		memory: make(map[string]*buf.FileBuffer, memoryMapSize),
		askCh:  make(chan string),
		delCh:  make(chan string),
		repCh:  make(chan *buf.FileBuffer),
		setCh:  make(chan *buf.FileBuffer),
	}
}

// Start begins the willow memory hole processor.
func (w *Willow) Start() {
	go w.memoryHole()
}

// Stop the willow processor. Do not use it again after you call this. Make a new one.
func (w *Willow) Stop() {
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
	defer close(w.repCh) // signal Stop() we are done.

	groups := time.NewTicker(w.config.GroupInterval.Duration)
	defer groups.Stop()

	for {
		select {
		case buf := <-w.setCh:
			w.memory[buf.Path] = buf
		case path := <-w.delCh:
			delete(w.memory, path)
		case now := <-groups.C:
			w.washer(now)
		case path, ok := <-w.askCh:
			if !ok {
				return
			}

			w.repCh <- w.memory[path]
		}
	}
}

// washer is called on an interval by memoryHole().
func (w *Willow) washer(now time.Time) {
	for path, file := range w.memory {
		if now.Sub(file.FirstWrite) < w.config.FlushInterval.Duration {
			continue
		}

		go file.Flush(buf.FlusOpts{Type: "exp", FollowUp: func() { w.Delete(path) }})
	}
}
