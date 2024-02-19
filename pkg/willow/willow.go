// Package willow provides a memory buffer to flush files to disk.
package willow

import (
	"time"

	"github.com/Notifiarr/fogwillow/pkg/buf"
	"golift.io/cnfg"
)

// Defaults for the module.
const (
	DefaultFileSysBuffer = 1024 * 10
	DefaultFlushInterval = 16 * time.Second
)

// Config is the input data needed to make a Willow.
type Config struct {
	// How often to scan entire memory map for flushable files.
	GroupInterval cnfg.Duration `toml:"group_interval" xml:"group_interval"`
	// How old a file must be to flush it away.
	FlushInterval cnfg.Duration `toml:"flush_interval" xml:"flush_interval"`
	// How large to make the channel buffer for file system changes.
	BufferFileSys uint `toml:"buffer_file_sys" xml:"buffer_file_sys"`
	// How many threads to run for file system changes.
	Writers uint `toml:"writers" xml:"writers"`
	// These allow this module to produce metrics.
	Expires  func()              `toml:"-" xml:"-"`
	AddBytes func(bytes float64) `toml:"-" xml:"-"`
	IncFiles func()              `toml:"-" xml:"-"`
}

// Willow is the working struct for this module. Get one from, NeWillow().
type Willow struct {
	config *Config
	memory map[string]*buf.FileBuffer // The file buffer memory map.
	delCh  chan string                // This channel is used by Delete().
	askCh  chan string                // This channel it used by Get().
	repCh  chan *buf.FileBuffer       // This is the response channel for Get().
	setCh  chan *buf.FileBuffer       // This channel is used by Set().
	fsOp   chan *flush                // This channel is used to make file system changes.
	fsDone chan struct{}              // This channel is used to close fs workers.
}

const (
	// initial allocation for the file buffer map.
	memoryMapSize = 100
	expiredLog    = "exp"
)

// NeWillow returns a new, configured Willow pointer.
func NeWillow(config *Config) *Willow {
	config.setup()

	return &Willow{
		config: config,
		memory: make(map[string]*buf.FileBuffer, memoryMapSize),
		askCh:  make(chan string),
		delCh:  make(chan string),
		repCh:  make(chan *buf.FileBuffer),
		setCh:  make(chan *buf.FileBuffer),
		fsOp:   make(chan *flush, config.BufferFileSys),
		fsDone: make(chan struct{}),
	}
}

func (c *Config) setup() {
	const divideBy = 4

	if c.FlushInterval.Duration <= 0 {
		c.FlushInterval.Duration = DefaultFlushInterval
	}

	if c.GroupInterval.Duration <= 0 {
		c.GroupInterval.Duration = c.FlushInterval.Duration / divideBy
	}

	if c.Writers < 1 {
		c.Writers = 1
	}

	if c.BufferFileSys < 1 {
		c.BufferFileSys = DefaultFileSysBuffer
	}

	if c.AddBytes == nil {
		c.AddBytes = func(_ float64) {}
	}

	if c.IncFiles == nil {
		c.IncFiles = func() {}
	}

	if c.Expires == nil {
		c.Expires = func() {}
	}
}

// Start begins the willow memory hole processor.
func (w *Willow) Start() {
	for i := w.config.Writers; i > 0; i-- {
		go w.fileSystemWriter()
	}

	go w.memoryHole()
}

// Stop the willow processor. Waits for all files to flush to disk.
// Do not use this Willow again after you call Stop(). Make a new one.
func (w *Willow) Stop() {
	w.stopMemoryHole()
	w.stopSystemWriter()
}
