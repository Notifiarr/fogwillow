package fog

import (
	"log"
	"os"

	"golift.io/rotatorr"
	"golift.io/rotatorr/timerotator"
)

// SetupLogs starts the logs rotation and sets logger output to the configured file(s).
// You must call this before calling Start to setup logs, or things will panic.
//
//nolint:gomnd
func (c *Config) SetupLogs() {
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
		FileSize: int64(c.LogFileMB * 1024 * 1024),
		FileMode: 0o644,
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
	c.log.Printf("[ERROR] "+msg, v...)
}

// PrintConfig logs the current configuration information.
func (c *Config) PrintConfig() {
	c.Printf("=> Fogwillow Starting, pid: %d", os.Getpid())
	c.Printf("=> Listen Address: %s", c.ListenAddr)
	c.Printf("=> Output Path: %s", c.OutputPath)
	c.Printf("=> Flush Interval: %s", c.FlushInterval)
	c.Printf("=> Buffers; UDP/Packet/Channel: %d/%d/%d", c.BufferUDP, c.BufferPacket, c.BufferChannel)

	if c.LogFile != "" {
		c.Printf("=> Log File: %s (count: %d, size: %dMB)", c.LogFile, c.LogFiles, c.LogFileMB)
	} else {
		c.Printf("=> No Log File")
	}
}
