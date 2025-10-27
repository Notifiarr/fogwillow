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

// Constants for file and directory modes.
const (
	FileMode = 0o664
	DirMode  = 0o755
)

// bufferPool is a sync.Pool to store and reuse *bytes.Buffer objects.
var bufferPool = sync.Pool{ //nolint:gochecknoglobals
	New: func() any {
		// This function is called when the pool is empty and a new object is requested.
		return &FileBuffer{}
	},
}

// FileBuffer holds a file before it gets flushed to disk.
type FileBuffer struct {
	FirstWrite time.Time
	Writes     uint
	mu         sync.Mutex
	buf        bytes.Buffer
	Path       string
}

// NewBufferFromPool returns a FileBuffer from a pool.
func NewBufferFromPool(path string, data []byte) *FileBuffer {
	buf := bufferPool.Get().(*FileBuffer) //nolint:forcetypeassert
	buf.Path = path
	buf.FirstWrite = time.Now()
	buf.Writes = 1
	buf.buf.Write(data)

	return buf
}

// NewBuffer returns a new FileBuffer read to use.
func NewBuffer(path string, data []byte) *FileBuffer {
	return &FileBuffer{
		FirstWrite: time.Now(),
		Writes:     1,
		buf:        *bytes.NewBuffer(data),
		Path:       path,
	}
}

// Write sends content to the file buffer and increments the write counter.
// We added a mutex that makes this thread safe.
func (f *FileBuffer) Write(data []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.Writes++

	return f.buf.Write(data) //nolint:wrapcheck
}

// Len returns the length of the file buffer.
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

	err := os.RemoveAll(f.Path)
	if err != nil {
		return fmt.Errorf("deleting file buffer: %s: %w", f.Path, err)
	}

	return nil
}

// Reset resets the file buffer to its initial state. It's not thread safe, and should not be called more than once.
// Only call this if you're using the buffer pool: NewBufferFromPool().
func (f *FileBuffer) Reset(maxSize int) {
	if f.buf.Cap() <= maxSize {
		f.buf.Reset()
		bufferPool.Put(f)
	}
}

// Flush writes the file buffer to disk.
func (f *FileBuffer) Flush(opts FlusOpts) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	err := os.MkdirAll(filepath.Dir(f.Path), DirMode)
	if err != nil {
		return 0, fmt.Errorf("creating dir for %s: %w", f.Path, err)
	}

	fileFlag := os.O_APPEND | os.O_CREATE | os.O_WRONLY
	if opts.Truncate {
		fileFlag = os.O_TRUNC | os.O_CREATE | os.O_WRONLY
	}

	file, err := os.OpenFile(f.Path, fileFlag, FileMode) //nolint:gosec
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
