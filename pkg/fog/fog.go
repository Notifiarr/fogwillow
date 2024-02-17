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

var ErrInvalidPacket = fmt.Errorf("invalid packet")

// Handler is invoked by packetListener for every received packet.
// This is where the packet is parsed and stored into memory for later flushing to disk.
func (p *packet) Handler(config *Config, memory *willow.Willow) {
	settings, body, err := p.parse()
	if err != nil {
		config.Errorf("%v", err)
		return
	}

	// Combine our base path with the filename path provided in the packet.
	filePath := filepath.Join(config.OutputPath, settings["filepath"])
	fileBuffer := memory.Get(filePath)

	if fileBuffer == nil {
		// Create a new fileBuffer.
		fileBuffer = buf.NewBuffer(filePath, body, config)
		// Save the new file buffer in memory.
		memory.Set(fileBuffer)
	} else if _, err := fileBuffer.Write(body); err != nil { // Append directly to existing buffer.
		config.Errorf("Adding %d bytes to buffer (%d) for %s", p.size, fileBuffer.Len(), filePath)
	}

	if trunc := settings["truncate"] == "true"; trunc || settings["flush"] == "true" {
		fileBuffer.Flush(buf.FlusOpts{Truncate: trunc}) // write to disk
		memory.Delete(filePath)                         // remove from memory
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
