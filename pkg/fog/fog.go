package fog

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type packet struct {
	data  []byte
	size  int
	addr  *net.UDPAddr
	count uint
}

type fileBuffer struct {
	*bytes.Buffer
	path       string
	config     *Config
	firstWrite time.Time
	writes     uint
}

type fileMemMap map[string]*fileBuffer

// packetReader gets raw packets from the UDP socket and sends them to another go routine using a buffered channel.
func (c *Config) packetReader() {
	var err error

	for count := uint(0); ; count++ {
		packet := &packet{data: make([]byte, c.BufferPacket), count: count}

		packet.size, packet.addr, err = c.sock.ReadFromUDP(packet.data)
		if errors.Is(err, net.ErrClosed) {
			// This happens on normal shutdown.
			c.Printf("Closing UDP packet reader: %v", err)
			return
		} else if err != nil {
			// This is probably rare.
			c.Errorf("Reading UDP socket: %v", err)
			continue
		}

		c.Debugf("Got packet %d from %s at %d bytes; channel buffer: %d ",
			packet.count, packet.addr, packet.size, len(c.packets))

		c.packets <- packet
	}
}

// packetListener receives packets from packetReader using a buffered channel.
// This function also maintains the temporary in-memory buffer for all packets.
func (c *Config) packetListener() {
	defer c.Printf("Closing UDP packet listener")
	// How often do we scan all rows and check for expired items?
	groups := time.NewTicker(c.GroupInterval.Duration)
	defer groups.Stop()

	memory := make(fileMemMap)

	for {
		select {
		case now := <-groups.C:
			c.cleanMemory(now, memory)
		case packet, ok := <-c.packets:
			if !ok {
				return
			}

			packet.Handler(c, memory)
		}
	}
}

func (c *Config) cleanMemory(now time.Time, memory fileMemMap) {
	for path, file := range memory {
		if now.Sub(file.firstWrite) < c.FlushInterval.Duration {
			continue
		}

		file.Flush()
		delete(memory, path)
	}
}

var ErrInvalidPacket = fmt.Errorf("invalid packet")

// Handler is invoked by packetListener for every received packet.
// This is where the packet is parsed and stored into memory for later flushing to disk.
func (p *packet) Handler(config *Config, memory fileMemMap) {
	settings, body, err := p.parse()
	if err != nil {
		config.Errorf("%v", err)
		return
	}

	// Combine our base path with the filename path provided in the packet.
	filePath := filepath.Join(config.OutputPath, settings["filepath"])

	if settings["flush"] == "true" {
		defer func() {
			memory[filePath].Flush()
			delete(memory, filePath)
		}()
	}

	if memory[filePath] == nil {
		// This creates the initial buffer, and never returns an error.
		memory[filePath] = &fileBuffer{
			Buffer:     bytes.NewBuffer(body),
			path:       filePath,
			config:     config,
			firstWrite: time.Now(),
			writes:     1,
		}

		return
	}

	// We can use write count to create metrics.
	memory[filePath].writes++
	// If a buffer already exists, this appends directly to it.
	if _, err := memory[filePath].Write(body); err != nil {
		config.Errorf("Adding %d bytes to buffer (%d) for %s", p.size, memory[filePath].Len(), filePath)
	}
}

//nolint:gomnd
func (p *packet) parse() (map[string]string, []byte, error) {
	newline := bytes.IndexByte(p.data, '\n')
	if newline < 0 {
		return nil, nil, fmt.Errorf("%w from %s (first newline at %d)", ErrInvalidPacket, p.addr.IP, newline)
	}

	// Turn the first line into a number. That number tells us how many more lines to parse. Usually 1 to 3.
	settingCount, err := strconv.Atoi(string(p.data[0:newline]))
	if err != nil {
		return nil, nil, fmt.Errorf("%w: from %s (first newline at %d, prior value: %s)",
			err, p.addr.IP, newline, string(p.data[0:newline]))
	}

	settings := make(map[string]string, settingCount)
	lastline := newline + 1 // +1 to remove the \n
	// Parse each line 1 at a time and add them to the settings map.
	for ; settingCount > 0; settingCount-- {
		newline = bytes.IndexByte(p.data[lastline:], '\n')
		if newline < 0 {
			return nil, nil, fmt.Errorf("%w with %d settings from %s (newline/lastline: %d/%d): missing first newline",
				ErrInvalidPacket, settingCount+len(settings), p.addr.IP, newline, lastline)
		}
		// Split the setting line on = to get name and value.
		setting := strings.SplitN(string(p.data[lastline:newline+lastline]), "=", 2)
		if len(setting) != 2 {
			return nil, nil, fmt.Errorf("%w with %d settings from %s (newline/lastline: %d/%d): setting '%s' missing equal",
				ErrInvalidPacket, settingCount+len(settings), p.addr.IP, newline, lastline, setting[0])
		}
		// Set the name and value, increment lastline and repeat.
		settings[setting[0]] = setting[1]
		lastline += newline + 1 // +1 to remove the \n
	}

	if settings["filepath"] == "" {
		return nil, nil, fmt.Errorf("%w from %s with %d settings and no filepath (newline/lastline: %d/%d) %s",
			ErrInvalidPacket, p.addr.IP, len(settings), newline, lastline, settings)
	}

	return settings, p.data[lastline:p.size], nil
}

// Flush writes the file buffer to disk.
func (f *fileBuffer) Flush() {
	if err := os.MkdirAll(filepath.Dir(f.path), DefaultDirMode); err != nil {
		// We could return here, but let's try to write the file anyway?
		f.config.Errorf("Creating dir for %s: %v", f.path, err)
	}

	file, err := os.OpenFile(f.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, DefaultFileMode)
	if err != nil {
		f.config.Errorf("Opening or creating file %s: %v", f.path, err)
		return
	}
	defer file.Close()

	size, err := file.Write(f.Bytes())
	if err != nil {
		// Since all we do is print an info message, we do not need to return here.
		// Consider that if you add more logic after this stanza.
		f.config.Errorf("Writing file '%s' content: %v", f.path, err)
	}

	f.config.Printf("Wrote %d bytes (%d lines) to '%s'", size, f.writes, f.path)
}
