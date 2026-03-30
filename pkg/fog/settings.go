package fog

import (
	"bytes"
	"path/filepath"
	"sync"
)

// settingsPool reuses Settings maps across packets to avoid a per-packet allocation.
var settingsPool = sync.Pool{ //nolint:gochecknoglobals
	New: func() any { return make(Settings, 4) }, //nolint:mnd
}

// List of settings the app recognizes from packet parsing.
const (
	Password = "password" // must match app password.
	Truncate = "truncate" // if true, truncate the file.
	Filepath = "filepath" // must be set on every request.
	Delete   = "delete"   // will delete an entire tree it true.
	Flush    = "flush"    // flush the file immediately.
)

// Settings are parsed configuration settings from incoming packets.
type Settings map[string]setting

// setting holds a raw value byte slice pointing directly into the packet buffer.
// It is only valid for the lifetime of the Handler() call that owns the packet.
type setting []byte

// Set a setting in the map. val is stored as-is with no copy.
func (s Settings) Set(key string, val []byte) {
	s[key] = setting(val)
}

// resetAndReturn clears all keys and returns the map to the pool for reuse.
func (s Settings) resetAndReturn() {
	for k := range s {
		delete(s, k)
	}

	settingsPool.Put(s)
}

// ValidFilepath returns false if there is no file path setting or it contains bad stuff.
func (s Settings) ValidFilepath() bool {
	return len(s[Filepath]) > 0 && !bytes.Contains(s[Filepath], []byte("../"))
}

// Filepath trims and appends a root path to a setting path.
func (s Settings) Filepath(path string) string {
	filePath := s[Filepath]
	// Use a zero-alloc string comparison to trim the path prefix if present.
	if len(filePath) >= len(path) && string(filePath[:len(path)]) == path {
		filePath = filePath[len(path):]
	}

	return filepath.Join(path, string(filePath))
}

// Truncate returns true if the truncate flag is enabled.
func (s Settings) Truncate() bool {
	return s[Truncate].True()
}

// Flush returns true if the flush flag is enabled.
func (s Settings) Flush() bool {
	return s[Flush].True()
}

// Delete returns true if the delete flag is enabled.
func (s Settings) Delete() bool {
	return s[Delete].True()
}

// Password returns true if the password setting matches the one provided.
// string([]byte) == string is optimized by the compiler to not allocate.
func (s Settings) Password(confPassword string) bool {
	return string(s[Password]) == confPassword
}

// True returns true if the setting value equals "true".
// string([]byte) == string literal is optimized by the compiler to not allocate.
func (s setting) True() bool {
	return string(s) == "true"
}
