// Package buf provides a small file buffer that can be flushed to disk.
// Used by willow package.
package buf

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	FileMode = 0o664
	DirMode  = 0o755
)

// FileBuffer holds a file before it gets flushed to disk.
type FileBuffer struct {
	Logger
	FirstWrite time.Time
	writes     uint
	mu         sync.Mutex
	buf        *bytes.Buffer
	Path       string
}

// Logger lets this sub module print messages.
type Logger interface {
	Errorf(msg string, v ...interface{})
	Printf(msg string, v ...interface{})
	Debugf(msg string, v ...interface{})
}

// NewBuffer returns a new FileBuffer read to use.
func NewBuffer(path string, data []byte, logger Logger) *FileBuffer {
	if logger == nil {
		panic("NewBuffer() cannot take a nil logger")
	}

	return &FileBuffer{
		Logger:     logger,
		FirstWrite: time.Now(),
		writes:     1,
		buf:        bytes.NewBuffer(data),
		Path:       path,
	}
}

// Write sends content to the file buffer and increments the write counter.
// We added a mutex that makes this thread safe.
func (f *FileBuffer) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writes++

	return f.buf.Write(p) //nolint:wrapcheck
}

func (f *FileBuffer) Len() int {
	return f.buf.Len()
}

// FlusOpts allows passing data into the file flusher.
type FlusOpts struct {
	// Type is arbitrary data that probably just gets logged.
	Type string
	// Delete the file contents before writing?
	Truncate bool
}

// RmRfDir deletes the path in the fileBuffer. This is dangerous and destructive.
func (f *FileBuffer) RmRfDir() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.Debugf("Deleting recursively: %s", f.Path)

	start := time.Now()

	if err := os.RemoveAll(f.Path); err != nil {
		f.Errorf("Deleting path %s: %v", f.Path, err)
		return
	}

	f.Printf("Deleted %s in %s", f.Path, time.Since(start))
}

// Flush writes the file buffer to disk.
func (f *FileBuffer) Flush(opts FlusOpts) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(f.Path), DirMode); err != nil {
		// We could return here, but let's try to write the file anyway?
		f.Errorf("Creating dir for %s: %v", f.Path, err)
	}

	word, fileFlag := "Wrote", os.O_APPEND|os.O_CREATE|os.O_WRONLY
	if opts.Truncate {
		word, fileFlag = "Truncated", os.O_TRUNC|os.O_CREATE|os.O_WRONLY
	}

	file, err := os.OpenFile(f.Path, fileFlag, FileMode)
	if err != nil {
		f.Errorf("Opening or creating file %s: %v", f.Path, err)
		return 0
	}
	defer file.Close()

	size, err := file.Write(f.buf.Bytes())
	if err != nil {
		// Since all we do is print an info message, we do not need to return here.
		// Consider that if you add more logic after this stanza.
		f.Errorf("Writing file '%s' content: %v", f.Path, err)
	}

	f.Printf("%s (%s) %d bytes (%d writes, age: %s) to '%s'",
		word, opts.Type, size, f.writes, time.Since(f.FirstWrite.Round(time.Second)), f.Path)

	return size
}
