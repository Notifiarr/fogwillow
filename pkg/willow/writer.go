package willow

import "github.com/Notifiarr/fogwillow/pkg/buf"

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
func (w *Willow) fileSystemWriter() {
	defer func() { // Signal Stop() we're done.
		w.fsDone <- struct{}{}
	}()

	var size int

	for file := range w.fsOp {
		if file.delete {
			file.RmRfDir(file.FlusOpts)
			continue
		}

		size = file.Flush(file.FlusOpts)
		w.config.AddBytes(float64(size))
		w.config.IncFiles()

		if file.Type == expiredLog {
			w.config.Expires()
		}
	}
}
