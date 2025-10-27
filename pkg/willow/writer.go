package willow

import (
	"time"

	"github.com/Notifiarr/fogwillow/pkg/buf"
)

type flush struct {
	*buf.FileBuffer
	buf.FlusOpts
	delete bool
}

const hundred = 100

// FSLen returns the number of file buffers waiting in the buffered channel.
func (w *Willow) FSLen() int {
	return len(w.fsOp)
}

// RmRfDir deletes a file buffer path from disk, recursively, through a buffered channel.
func (w *Willow) RmRfDir(buf *buf.FileBuffer, opt buf.FlusOpts) {
	w.fsOp <- &flush{FileBuffer: buf, FlusOpts: opt, delete: true}
}

// Flush a file to disk through a buffered channel.
func (w *Willow) Flush(buf *buf.FileBuffer, opt buf.FlusOpts) {
	w.fsOp <- &flush{FileBuffer: buf, FlusOpts: opt}
}

// stopSystemWriter closes all the threads created by fileSystemWriter().
func (w *Willow) stopSystemWriter() {
	defer func() {
		close(w.fsDone)
		w.fsOp = nil
		w.fsDone = nil
	}()

	close(w.fsOp)

	for i := w.config.Writers; i > 0; i-- {
		<-w.fsDone // Wait for files to write.
	}
}

// fileSystemWriter runs in 1 or more go routines to write files to the file system.
func (w *Willow) fileSystemWriter(idx uint) {
	w.config.Printf("Starting file system writer %d.", idx)

	defer func() { // Signal Stop() we're done.
		w.config.Printf("Closing file system writer %d.", idx)
		w.fsDone <- struct{}{} //nolint:wsl_v5
	}()

	var start time.Time

	for file := range w.fsOp {
		start = time.Now()

		if file.delete {
			w.deleteFile(file.FileBuffer, start)
		} else {
			w.flushFile(file, start)
		}

		if w.config.BufferPool { // Reset the buffer, so it can be reused.
			file.Reset(100 << 10) //nolint:mnd // Limit reused file-buffers to 100KB.
		}
	}
}

func (w *Willow) deleteFile(file *buf.FileBuffer, start time.Time) {
	w.config.Debugf("Deleting recursively: %s", file.Path)
	w.config.Ages.WithLabelValues("delete").Observe(start.Sub(file.FirstWrite).Seconds())

	err := file.RmRfDir()
	if err != nil {
		w.config.Errorf("%v", err)
	}

	w.config.Durs.WithLabelValues("delete").Observe(time.Since(start).Seconds())
	w.config.Printf("Deleted %s in %s", file.Path, time.Since(start))
}

func (w *Willow) flushFile(file *flush, start time.Time) {
	w.config.Ages.WithLabelValues("file").Observe(start.Sub(file.FirstWrite).Seconds())

	size, err := file.Flush(file.FlusOpts)
	if err != nil {
		w.config.Errorf("%v", err)
	}

	w.config.Durs.WithLabelValues("file").Observe(time.Since(start).Seconds())
	w.config.Bytes.Add(float64(size))

	word := "Wrote"
	if file.Truncate {
		word = "Truncated"
	}

	w.config.Printf("%s (%s) [buf:%d/%d,files:%d] %d bytes in %s (%d writes, age: %s) to '%s'",
		word, file.Type, len(w.fsOp), cap(w.fsOp), len(w.memory), size, time.Since(start).Round(hundred*time.Microsecond),
		file.Writes, time.Since(file.FirstWrite).Round(hundred*time.Microsecond), file.Path)

	if file.Type == expiredLog {
		w.config.Expires.Inc()
	}
}
