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
		w.fsDone <- struct{}{}
	}()

	var (
		err   error
		size  int
		start time.Time
		word  string
	)

	for file := range w.fsOp {
		start = time.Now()

		if file.delete {
			w.config.Debugf("Deleting recursively: %s", file.Path)
			w.config.Ages.WithLabelValues("delete").Observe(start.Sub(file.FirstWrite).Seconds())

			if err = file.RmRfDir(); err != nil {
				w.config.Errorf("%v", err)
			}

			w.config.Durs.WithLabelValues("delete").Observe(time.Since(start).Seconds())
			w.config.Printf("Deleted %s in %s", file.Path, time.Since(start))

			continue
		}

		w.config.Ages.WithLabelValues("file").Observe(start.Sub(file.FirstWrite).Seconds())

		if size, err = file.Flush(file.FlusOpts); err != nil {
			w.config.Errorf("%v", err)
		}

		w.config.Durs.WithLabelValues("file").Observe(time.Since(start).Seconds())
		w.config.Bytes.Add(float64(size))

		if word = "Wrote"; file.Truncate {
			word = "Truncated"
		}

		w.config.Printf("%s (%s) %d bytes in %s (%d writes, age: %s) to '%s'",
			word, file.Type, size, time.Since(start).Round(time.Millisecond),
			file.Writes, time.Since(file.FirstWrite).Round(time.Millisecond), file.Path)

		if file.Type == expiredLog {
			w.config.Expires.Inc()
		}
	}
}
