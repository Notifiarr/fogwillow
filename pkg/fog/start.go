package fog

import (
	"fmt"
	"log"
	"net"
	"time"

	"golift.io/cnfg"
	"golift.io/cnfgfile"
)

const (
	DefaultFlushInterval = 16 * time.Second
	DefaultListenAddr    = ":12345"
	DefaultOutputPath    = "/tmp"
	DefaultFileMode      = 0o644
	DefaultDirMode       = 0o755
	DefaultUDPBuffer     = 1024 * 1024
	DefaultPacketBuffer  = 1024 * 100
	DefaultChannelBuffer = 1024 * 10
)

type Config struct {
	GroupInterval cnfg.Duration `toml:"group_interval" xml:"group_interval"`
	FlushInterval cnfg.Duration `toml:"flush_interval" xml:"flush_interval"`
	OutputPath    string        `toml:"output_path"    xml:"output_path"`
	ListenAddr    string        `toml:"listen_addr"    xml:"listen_addr"`
	LogFile       string        `toml:"log_file"       xml:"log_file"`
	LogFileMB     uint          `toml:"log_file_mb"    xml:"log_file_mb"`
	LogFiles      uint          `toml:"log_files"      xml:"log_files"`
	BufferUDP     uint          `toml:"buffer_udp"     xml:"buffer_udp"`
	BufferPacket  uint          `toml:"buffer_packet"  xml:"buffer_packet"`
	BufferChannel uint          `toml:"buffer_channel" xml:"buffer_channel"`
	Debug         bool          `toml:"debug"          xml:"debug"`
	log           *log.Logger
	packets       chan *packet
	sock          *net.UDPConn
}

// LoadConfigFile does what its name implies.
func LoadConfigFile(path string) (*Config, error) {
	config := &Config{
		BufferChannel: DefaultChannelBuffer,
		BufferPacket:  DefaultPacketBuffer,
		BufferUDP:     DefaultUDPBuffer,
		FlushInterval: cnfg.Duration{Duration: DefaultFlushInterval},
		OutputPath:    DefaultOutputPath,
		ListenAddr:    DefaultListenAddr,
	}
	defer config.setup()

	if err := cnfgfile.Unmarshal(config, path); err != nil {
		return nil, fmt.Errorf("failed to parse configuration: %w", err)
	}

	if _, err := cnfg.UnmarshalENV(config, "FW"); err != nil {
		return nil, fmt.Errorf("failed to parse environment: %w", err)
	}

	return config, nil
}

func (c *Config) Start() error {
	if err := c.setupSocket(); err != nil {
		return err
	}

	go c.packetListener()
	go c.packetReader()

	return nil
}

// setup makes sure configurations are sound and sane.
func (c *Config) setup() {
	const divideBy = 4

	if c.GroupInterval.Duration == 0 {
		c.GroupInterval.Duration = c.FlushInterval.Duration / divideBy
	}

	if c.BufferChannel == 0 {
		c.BufferChannel = DefaultChannelBuffer
	}

	c.packets = make(chan *packet, c.BufferChannel)
}

func (c *Config) setupSocket() error {
	addr, err := net.ResolveUDPAddr("udp", c.ListenAddr)
	if err != nil {
		return fmt.Errorf("invalid listen_addr provided: %w", err)
	}

	if c.sock, err = net.ListenUDP("udp", addr); err != nil {
		return fmt.Errorf("unable to use provided listen_addr: %w", err)
	}

	if err := c.sock.SetReadBuffer(int(c.BufferUDP)); err != nil {
		return fmt.Errorf("unable to set socket read buffer %d: %w", c.BufferUDP, err)
	}

	return nil
}

func (c *Config) Shutdown() {
	c.sock.Close()
	close(c.packets)
}
