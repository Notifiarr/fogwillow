package fog

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/Notifiarr/fogwillow/pkg/buf"
)

type packet struct {
	data *[]byte
	size int
	addr *net.UDPAddr
	*Config
}

// Errors for the packet parser.
var (
	ErrInvalidPacket = errors.New("invalid packet")
	ErrBadPassword   = errors.New("bad password")
)

func (c *Config) newSlice() any {
	p := make([]byte, c.BufferPacket)
	return &p
}

func (c *Config) getSlice() *[]byte {
	return c.bytesPool.Get().(*[]byte) //nolint:forcetypeassert
}

// packetListener gets raw packets from the UDP socket and sends them to another go routine.
func (c *Config) packetListener(idx uint) {
	defer c.listenerWg.Done()

	c.Printf("Starting UDP packet listener %d, max packet size: %d bytes", idx, c.BufferPacket)

	var err error

	for count := uint64(0); ; count++ {
		packet := &packet{data: c.getSlice(), Config: c}

		packet.size, packet.addr, err = c.sock.ReadFromUDP(*packet.data)
		if errors.Is(err, net.ErrClosed) {
			// This happens on normal shutdown.
			c.Printf("Closing UDP packet listener %d.", idx)
			return
		} else if err != nil {
			// This is probably rare.
			c.Errorf("Reading UDP socket %d: %v", idx, err)
			c.bytesPool.Put(packet.data)

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
	defer c.processorWg.Done()

	c.Printf("Starting UDP packet processor %d, channel size: %d", idx, len(c.packets))
	defer c.Printf("Closing UDP packet processor %d.", idx)

	for packet := range c.packets {
		c.metrics.Packets.Inc()
		packet.Handler()

		if c.BufferPool {
			// Put the packet's byte slice back into the pool for reuse.
			c.bytesPool.Put(packet.data)
		}
	}
}

// Handler is invoked by packetListener for every received packet.
// This is where the packet is parsed and stored into memory for later flushing to disk.
func (p *packet) Handler() {
	settings := settingsPool.Get().(Settings) //nolint:forcetypeassert
	defer settings.resetAndReturn()

	body, err := p.parse(settings)
	if err != nil {
		p.Errorf("%v", err)
		return
	}

	err = p.check(settings, p.Password)
	if err != nil {
		p.Errorf("%v", err)
		return
	}

	// Combine our base path with the filename path provided in the packet.
	filePath := settings.Filepath(p.OutputPath)
	fileBuffer := p.getOrWrite(filePath, body)

	switch {
	case settings.Delete():
		// Remove the file buffer from our memory map.
		p.willow.Delete(filePath)
		// Remove the file or folder from disk, recursively.
		p.willow.RmRfDir(fileBuffer, buf.FlusOpts{})
	case settings.Truncate():
		p.metrics.Truncate.Inc()
		fallthrough
	case settings.Flush():
		p.metrics.Flushes.Inc()
		p.willow.Delete(filePath)                // remove from memory
		p.willow.Flush(fileBuffer, buf.FlusOpts{ // write to disk
			Truncate: settings.Truncate(),
			Type:     "eof",
		})
	}
}

// getOrWrite returns the file buffer for the given path, writing body into it.
// If no buffer exists for the path it creates one via TrySet; if another
// processor wins the race for the same path the candidate is discarded and
// body is written into the winning buffer instead.
func (p *packet) getOrWrite(filePath string, body []byte) *buf.FileBuffer {
	if fileBuffer := p.willow.Get(filePath); fileBuffer != nil {
		_, err := fileBuffer.Write(body)
		if err != nil {
			p.Errorf("Adding %d bytes to buffer (%d) for %s", p.size, fileBuffer.Len(), filePath)
		}

		return fileBuffer
	}

	// No existing buffer; create a candidate with body already written.
	candidate := p.newBuf(filePath, body)

	// Atomically store it. TrySet returns nil on success, or the winning buffer
	// when another processor stored the same path between our Get and TrySet.
	existing := p.willow.TrySet(candidate)
	if existing == nil {
		return candidate
	}

	// Lost the race: discard the candidate and write body into the winner.
	if p.BufferPool {
		candidate.ReturnToPool()
	}

	_, err := existing.Write(body)
	if err != nil {
		p.Errorf("Adding %d bytes to buffer (%d) for %s", p.size, existing.Len(), filePath)
	}

	return existing
}

// parse populates settings from the packet header and returns the body.
// settings must be an empty map obtained from the caller (typically via settingsPool).
func (p *packet) parse(settings Settings) ([]byte, error) {
	newline := bytes.IndexByte(*p.data, '\n')
	if newline < 0 {
		return nil, fmt.Errorf("%w from %s (first newline at %d)", ErrInvalidPacket, p.addr.IP, newline)
	}

	// Turn the first line into a number. That number tells us how many more lines to parse. Usually 1 to 3.
	settingCount, err := strconv.Atoi(string((*p.data)[0:newline]))
	if err != nil {
		return nil, fmt.Errorf("%w: from %s (first newline at %d, prior value: %s)",
			err, p.addr.IP, newline, string((*p.data)[0:newline]))
	}

	lastline := newline + 1 // +1 to remove the \n
	// Parse each setting line: "key=value\n"
	for range settingCount {
		newline = bytes.IndexByte((*p.data)[lastline:], '\n')
		if newline < 0 {
			return nil, fmt.Errorf("%w with %d settings from %s (newline/lastline: %d/%d): missing first newline",
				ErrInvalidPacket, settingCount, p.addr.IP, newline, lastline)
		}

		// Split on '=' without converting the line to a string first.
		line := (*p.data)[lastline : newline+lastline]
		key, val, ok := bytes.Cut(line, []byte("="))

		if !ok {
			return nil, fmt.Errorf("%w with %d settings from %s (newline/lastline: %d/%d): setting '%s' missing equal",
				ErrInvalidPacket, settingCount, p.addr.IP, newline, lastline, line)
		}

		settings.Set(string(key), val)

		lastline += newline + 1 // +1 to remove the \n
	}

	return (*p.data)[lastline:p.size], nil
}

// check the packet for valid settings.
func (p *packet) check(settings Settings, confPassword string) error {
	if !settings.ValidFilepath() {
		return fmt.Errorf("%w from %s with %d settings and invalid filepath: %s",
			ErrInvalidPacket, p.addr.IP, len(settings), settings.Filepath(""))
	}

	if !settings.Password(confPassword) {
		return fmt.Errorf("%w from %s with %d settings", ErrBadPassword, p.addr.IP, len(settings))
	}

	return nil
}
