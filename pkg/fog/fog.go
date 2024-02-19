package fog

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/Notifiarr/fogwillow/pkg/buf"
)

type packet struct {
	data []byte
	size int
	addr *net.UDPAddr
}

var (
	ErrInvalidPacket = fmt.Errorf("invalid packet")
	ErrBadPassword   = fmt.Errorf("bad password")
)

// packetListener gets raw packets from the UDP socket and sends them to another go routine.
func (c *Config) packetListener(idx uint) {
	c.Printf("Starting UDP packet listener %d.", idx)

	var err error

	for count := uint64(0); ; count++ {
		packet := &packet{data: make([]byte, c.BufferPacket)}

		packet.size, packet.addr, err = c.sock.ReadFromUDP(packet.data)
		if errors.Is(err, net.ErrClosed) {
			// This happens on normal shutdown.
			c.Printf("Closing UDP packet listener %d.", idx)
			return
		} else if err != nil {
			// This is probably rare.
			c.Errorf("Reading UDP socket %d: %v", idx, err)
			continue
		}

		c.Debugf("Thread %d got packet %d from %s at %d bytes; buffer: %d/%d",
			idx, count, packet.addr, packet.size, len(c.packets), cap(c.packets))

		c.packets <- packet
	}
}

// packetProcessor receives packets from packetReader using a buffered channel.
// This procedure launches the packet handler.
func (c *Config) packetProcessor(idx uint) {
	c.Printf("Starting UDP packet processor %d.", idx)
	defer c.Printf("Closing UDP packet processor %d.", idx)

	for packet := range c.packets {
		c.metrics.Packets.Inc()
		packet.Handler(c)
	}
}

// Handler is invoked by packetListener for every received packet.
// This is where the packet is parsed and stored into memory for later flushing to disk.
func (p *packet) Handler(config *Config) {
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
	filePath := settings.Filepath(config.OutputPath)
	fileBuffer := config.willow.Get(filePath)

	if fileBuffer == nil {
		// Create a new fileBuffer.
		fileBuffer = buf.NewBuffer(filePath, body, config)
		// Save the new file buffer in memory.
		config.willow.Set(fileBuffer)
	} else if _, err := fileBuffer.Write(body); err != nil { // Append directly to existing buffer.
		config.Errorf("Adding %d bytes to buffer (%d) for %s", p.size, fileBuffer.Len(), filePath)
	}

	switch {
	case settings.Delete():
		config.metrics.Deletes.Inc()
		config.willow.Delete(filePath)
		config.willow.RmRfDir(fileBuffer, buf.FlusOpts{})
	case settings.Truncate():
		config.metrics.Truncate.Inc()
		fallthrough
	case settings.Flush():
		config.metrics.Flushes.Inc()
		config.willow.Delete(filePath)                // remove from memory
		config.willow.Flush(fileBuffer, buf.FlusOpts{ // write to disk
			Truncate: settings.Truncate(),
			Type:     "eof",
		})
	}
}

// parse the packet into structured data.
func (p *packet) parse() (Settings, []byte, error) {
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

	settings := make(Settings, settingCount)
	lastline := newline + 1 // +1 to remove the \n
	// Parse each line 1 at a time and add them to the settings map.
	for ; settingCount > 0; settingCount-- {
		newline = bytes.IndexByte(p.data[lastline:], '\n')
		if newline < 0 {
			return nil, nil, fmt.Errorf("%w with %d settings from %s (newline/lastline: %d/%d): missing first newline",
				ErrInvalidPacket, settingCount+len(settings), p.addr.IP, newline, lastline)
		}
		// Split the setting line on = to get name and value.
		settingVal := strings.SplitN(string(p.data[lastline:newline+lastline]), "=", 2) //nolint:gomnd
		if len(settingVal) != 2 {                                                       //nolint:gomnd
			return nil, nil, fmt.Errorf("%w with %d settings from %s (newline/lastline: %d/%d): setting '%s' missing equal",
				ErrInvalidPacket, settingCount+len(settings), p.addr.IP, newline, lastline, settingVal[0])
		}
		// Set the name and value.
		settings.Set(settingVal[0], settingVal[1])
		// Increment lastline and repeat.
		lastline += newline + 1 // +1 to remove the \n
	}

	return settings, p.data[lastline:p.size], nil
}

// check the packet for valid settings.
func (p *packet) check(settings Settings, confPassword string) error {
	if !settings.HasFilepath() {
		return fmt.Errorf("%w from %s with %d settings and no filepath",
			ErrInvalidPacket, p.addr.IP, len(settings))
	}

	if !settings.Password(confPassword) {
		return fmt.Errorf("%w from %s with %d settings", ErrBadPassword, p.addr.IP, len(settings))
	}

	return nil
}
