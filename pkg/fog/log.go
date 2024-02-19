package fog

import (
	"log"
	"os"

	"golift.io/rotatorr"
	"golift.io/rotatorr/timerotator"
)

// This file is for the fogwillow log file. Not logs it processes from the network.

const (
	logFileMode = 0o644
	megabyte    = 1024 * 1024
)

// setupLogs starts the logs rotation and sets logger output to the configured file(s).
func (c *Config) setupLogs() {
	if c.LogFile == "" {
		c.log = log.New(os.Stderr, "", log.LstdFlags)
		return
	}

	var rotator *rotatorr.Logger

	// This ensures panics write to the log file.
	postRotate := func(_, _ string) { os.Stderr = rotator.File }
	defer postRotate("", "")

	rotator = rotatorr.NewMust(&rotatorr.Config{
		Filepath: c.LogFile,
		FileSize: int64(c.LogFileMB * megabyte),
		FileMode: logFileMode,
		Rotatorr: &timerotator.Layout{
			FileCount:  int(c.LogFiles),
			PostRotate: postRotate,
		},
	})
	c.log = log.New(rotator, "", log.LstdFlags)
	log.SetOutput(rotator)
}

// Debugf writes log lines... to stdout and/or a file.
func (c *Config) Debugf(msg string, v ...interface{}) {
	if !c.Debug {
		return
	}

	c.log.Printf("[DEBUG] "+msg, v...)
}

// Printf writes log lines... to stdout and/or a file.
func (c *Config) Printf(msg string, v ...interface{}) {
	c.log.Printf("[INFO] "+msg, v...)
}

// Errorf writes log lines... to stdout and/or a file.
func (c *Config) Errorf(msg string, v ...interface{}) {
	c.metrics.Errors.Inc()
	c.log.Printf("[ERROR] "+msg, v...)
}

// printConfig logs the current configuration information.
func (c *Config) printConfig() {
	c.Printf("=> Fog Willow Starting, pid: %d", os.Getpid())
	c.Printf("=> Listen Address / Password: %s / %v", c.ListenAddr, c.Password != "")
	c.Printf("=> Output Path: %s", c.OutputPath)
	c.Printf("=> Intervals; Flush/Group: %s/%s", c.FlushInterval, c.GroupInterval)
	c.Printf("=> Buffers; UDP/Packet/Chan/FS: %d/%d/%d/%d", c.BufferUDP, c.BufferPacket, c.BufferChan, c.BufferFileSys)
	c.Printf("=> Threads; Listen/Process/Writer: %d/%d/%d", c.Listeners, c.Processors, c.Writers)

	if c.LogFile != "" {
		c.Printf("=> Log File: %s (count: %d, size: %dMB), debug: %v", c.LogFile, c.LogFiles, c.LogFileMB, c.Debug)
	} else {
		c.Printf("=> No Log File, debug: %v", c.Debug)
	}
}
