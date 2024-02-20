// Package buf provides a small file buffer that can be flushed to disk.
// Used by willow package.
package buf

import (
	"bytes"
	"fmt"
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
	FirstWrite time.Time
	Writes     uint
	mu         sync.Mutex
	buf        *bytes.Buffer
	Path       string
}

// NewBuffer returns a new FileBuffer read to use.
func NewBuffer(path string, data []byte) *FileBuffer {
	return &FileBuffer{
		FirstWrite: time.Now(),
		Writes:     1,
		buf:        bytes.NewBuffer(data),
		Path:       path,
	}
}

// Write sends content to the file buffer and increments the write counter.
// We added a mutex that makes this thread safe.
func (f *FileBuffer) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Writes++

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
func (f *FileBuffer) RmRfDir() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := os.RemoveAll(f.Path); err != nil {
		return fmt.Errorf("deleting file buffer: %s: %w", f.Path, err)
	}

	return nil
}

// Flush writes the file buffer to disk.
func (f *FileBuffer) Flush(opts FlusOpts) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(f.Path), DirMode); err != nil {
		return 0, fmt.Errorf("creating dir for %s: %w", f.Path, err)
	}

	fileFlag := os.O_APPEND | os.O_CREATE | os.O_WRONLY
	if opts.Truncate {
		fileFlag = os.O_TRUNC | os.O_CREATE | os.O_WRONLY
	}

	file, err := os.OpenFile(f.Path, fileFlag, FileMode)
	if err != nil {
		return 0, fmt.Errorf("opening or creating file %s: %w", f.Path, err)
	}
	defer file.Close()

	size, err := file.Write(f.buf.Bytes())
	if err != nil {
		// Since all we do is print an info message, we do not need to return here.
		// Consider that if you add more logic after this stanza.
		return 0, fmt.Errorf("writing file '%s' content: %w", f.Path, err)
	}

	return size, nil
}
