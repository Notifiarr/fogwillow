package fog

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Notifiarr/fogwillow/pkg/buf"
	"github.com/Notifiarr/fogwillow/pkg/willow"
)

type packet struct {
	data  []byte
	size  int
	addr  *net.UDPAddr
	count uint64
}

// List of settings we recognize from packet parsing.
const (
	setPassword = "password"
	setTruncate = "truncate"
	setFilepath = "filepath"
	setDelete   = "delete"
	setFlush    = "flush"
)

var (
	ErrInvalidPacket = fmt.Errorf("invalid packet")
	ErrBadPassword   = fmt.Errorf("bad password")
)

// packetListener gets raw packets from the UDP socket and sends them to another go routine.
func (c *Config) packetListener(idx uint) {
	c.Printf("Starting UDP packet listener %d.", idx)

	var err error

	for count := uint64(0); ; count++ {
		packet := &packet{data: make([]byte, c.BufferPacket), count: count}

		packet.size, packet.addr, err = c.sock.ReadFromUDP(packet.data)
		if errors.Is(err, net.ErrClosed) {
			// This happens on normal shutdown.
			c.Printf("Closing UDP packet listener %d: %v", idx, err)
			return
		} else if err != nil {
			// This is probably rare.
			c.Errorf("Reading UDP socket %d: %v", idx, err)
			continue
		}

		c.Debugf("Thread %d got packet %d from %s at %d bytes; buffer: %d/%d",
			idx, packet.count, packet.addr, packet.size, len(c.packets), cap(c.packets))

		c.packets <- packet
	}
}

// packetProcessor receives packets from packetReader using a buffered channel.
// This procedure launches the packet handler.
func (c *Config) packetProcessor(idx uint) {
	c.Printf("Starting UDP packet processor %d.", idx)
	defer c.Printf("Closing UDP packet listener %d.", idx)

	for packet := range c.packets {
		packet.Handler(c, c.willow)
	}
}

// Handler is invoked by packetListener for every received packet.
// This is where the packet is parsed and stored into memory for later flushing to disk.
func (p *packet) Handler(config *Config, memory *willow.Willow) {
	settings, body, err := p.parse()
	if err != nil {
		config.Errorf("%v", err)
		return
	}

	if err := p.check(settings, config.Password); err != nil {
		config.Errorf("%v", err)
		return
	}

	// Combine our base path with the filename path provided in the packet.
	filePath := settings[setFilepath].PrefixPath(config.OutputPath)
	fileBuffer := memory.Get(filePath)

	if fileBuffer == nil {
		// Create a new fileBuffer.
		fileBuffer = buf.NewBuffer(filePath, body, config)
		// Save the new file buffer in memory.
		memory.Set(fileBuffer)
	} else if _, err := fileBuffer.Write(body); err != nil { // Append directly to existing buffer.
		config.Errorf("Adding %d bytes to buffer (%d) for %s", p.size, fileBuffer.Len(), filePath)
	}

	if settings[setDelete].True() {
		go fileBuffer.RmRfDir()
	} else if trunc := settings[setTruncate].True(); trunc || settings[setFlush].True() {
		fileBuffer.Flush(buf.FlusOpts{Truncate: trunc}) // write to disk
		memory.Delete(filePath)                         // remove from memory
	}
}

// parse the packet into structured data.
//
//nolint:gomnd
func (p *packet) parse() (map[string]setting, []byte, error) {
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

	settings := make(map[string]setting, settingCount)
	lastline := newline + 1 // +1 to remove the \n
	// Parse each line 1 at a time and add them to the settings map.
	for ; settingCount > 0; settingCount-- {
		newline = bytes.IndexByte(p.data[lastline:], '\n')
		if newline < 0 {
			return nil, nil, fmt.Errorf("%w with %d settings from %s (newline/lastline: %d/%d): missing first newline",
				ErrInvalidPacket, settingCount+len(settings), p.addr.IP, newline, lastline)
		}
		// Split the setting line on = to get name and value.
		settingVal := strings.SplitN(string(p.data[lastline:newline+lastline]), "=", 2)
		if len(settingVal) != 2 {
			return nil, nil, fmt.Errorf("%w with %d settings from %s (newline/lastline: %d/%d): setting '%s' missing equal",
				ErrInvalidPacket, settingCount+len(settings), p.addr.IP, newline, lastline, settingVal[0])
		}
		// Set the name and value, increment lastline and repeat.
		settings[settingVal[0]] = setting(settingVal[1])
		lastline += newline + 1 // +1 to remove the \n
	}

	return settings, p.data[lastline:p.size], nil
}

// check the packet for valid settings.
func (p *packet) check(settings map[string]setting, password string) error {
	if settings[setFilepath].Empty() {
		return fmt.Errorf("%w from %s with %d settings and no filepath",
			ErrInvalidPacket, p.addr.IP, len(settings))
	}

	if password != "" && !settings[setPassword].Equals(password) {
		return fmt.Errorf("%w from %s with %d settings", ErrBadPassword, p.addr.IP, len(settings))
	}

	return nil
}

// setting lets us bind cool methods to our string settings.
type setting string

// PrefixPath trims and appends a root path to a setting path.
// Only really useful for the 'filepath' setting.
func (s setting) PrefixPath(path string) string {
	return filepath.Join(path, strings.TrimPrefix(string(s), path))
}

// Equals returns true if the setting is equal to this value.
func (s setting) Equals(value string) bool {
	return string(s) == value
}

// True returns true if the setting is a "true" string.
func (s setting) True() bool {
	return string(s) == "true"
}

// Empty returns true if the setting is blank or nonexistent.
func (s setting) Empty() bool {
	return string(s) == ""
}
