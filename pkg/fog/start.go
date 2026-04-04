// Package fog is the main package for the Fogwillow application.
package fog

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/Notifiarr/fogwillow/pkg/api"
	"github.com/Notifiarr/fogwillow/pkg/buf"
	"github.com/Notifiarr/fogwillow/pkg/httpserver"
	"github.com/Notifiarr/fogwillow/pkg/metrics"
	"github.com/Notifiarr/fogwillow/pkg/willow"
	"golift.io/cnfg"
	"golift.io/cnfgfile"
)

// Some application defaults.
const (
	DefaultOutputPath   = "/tmp"
	DefaultUDPBuffer    = 1 << 22 // 4MB
	DefaultPacketBuffer = 1 << 18 // 256KB
	DefaultChanBuffer   = 1 << 15 // 32 thousand
	MaxStupidValue      = uint(9999999)
)

// Config is the input _and_ running data.
type Config struct {
	*willow.Config
	HTTPServer   *httpserver.Config `toml:"http_server"   xml:"http_server"`
	Password     string             `toml:"password"      xml:"password"`
	OutputPath   string             `toml:"output_path"   xml:"output_path"`
	ListenAddr   string             `toml:"listen_addr"   xml:"listen_addr"`
	LogFile      string             `toml:"log_file"      xml:"log_file"`
	LogFileMB    uint               `toml:"log_file_mb"   xml:"log_file_mb"`
	LogFiles     uint               `toml:"log_files"     xml:"log_files"`
	BufferUDP    uint               `toml:"buffer_udp"    xml:"buffer_udp"`
	BufferPacket uint               `toml:"buffer_packet" xml:"buffer_packet"`
	BufferChan   uint               `toml:"buffer_chan"   xml:"buffer_chan"`
	Listeners    uint               `toml:"listeners"     xml:"listeners"`
	Processors   uint               `toml:"processors"    xml:"processors"`
	Debug        bool               `toml:"debug"         xml:"debug"`
	log          *log.Logger
	packets      chan *packet
	sock         *net.UDPConn
	willow       *willow.Willow
	metrics      *metrics.Metrics
	httpSrv      *httpserver.Server
	newBuf       func(path string, data []byte) *buf.FileBuffer
	bytesPool    sync.Pool
	listenerWg   sync.WaitGroup
	processorWg  sync.WaitGroup
}

// LoadConfigFile does what its name implies.
func LoadConfigFile(path string) (*Config, error) {
	config := &Config{
		OutputPath:   DefaultOutputPath,
		HTTPServer:   httpserver.DefaultConfig(),
		BufferUDP:    DefaultUDPBuffer,
		BufferPacket: DefaultPacketBuffer,
		BufferChan:   DefaultChanBuffer,
		Config:       &willow.Config{BufferPool: true},
	}

	err := cnfgfile.Unmarshal(config, path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse configuration: %w", err)
	}

	_, err = cnfg.UnmarshalENV(config, "FW")
	if err != nil {
		return nil, fmt.Errorf("failed to parse environment: %w", err)
	}

	config.setup()
	config.setupLogs()
	config.printConfig()

	return config, nil
}

// Start the applications.
func (c *Config) Start() error {
	err := c.setupSocket()
	if err != nil {
		return err
	}

	c.willow.Start()

	for idx := range c.Processors {
		c.processorWg.Add(1)

		go c.packetProcessor(idx)
	}

	for idx := range c.Listeners {
		c.listenerWg.Add(1)

		go c.packetListener(idx)
	}

	fogAPI := api.New(c.OutputPath, c.Password)
	c.httpSrv = httpserver.New(c.HTTPServer, fogAPI.Register)

	err = c.httpSrv.ListenAndServe()
	if err != nil {
		return fmt.Errorf("web server failed: %w", err)
	}

	return nil
}

// setup makes sure configurations are sound and sane.
func (c *Config) setup() {
	// Protect uint->int64 conversions.
	if c.LogFileMB > MaxStupidValue {
		c.LogFileMB = MaxStupidValue
	}

	if c.LogFiles > MaxStupidValue {
		c.LogFiles = MaxStupidValue
	}

	if c.HTTPServer.AccessLogMB > MaxStupidValue {
		c.HTTPServer.AccessLogMB = MaxStupidValue
	}

	if c.HTTPServer.AccessLogFiles > MaxStupidValue {
		c.HTTPServer.AccessLogFiles = MaxStupidValue
	}

	if c.Processors < 1 {
		c.Processors = 1
	} else if c.Processors > MaxStupidValue {
		c.Processors = MaxStupidValue
	}

	if c.Listeners < 1 {
		c.Listeners = 1
	} else if c.Listeners > MaxStupidValue {
		c.Listeners = MaxStupidValue
	}

	if c.newBuf = buf.NewBuffer; c.BufferPool {
		c.newBuf = buf.NewBufferFromPool
	}

	c.bytesPool = sync.Pool{New: c.newSlice}
	c.packets = make(chan *packet, c.BufferChan)
	c.metrics = metrics.Get(metrics.Funcs{
		InMemory: func() float64 { return float64(c.willow.Len()) },
		ChanBuff: func() float64 { return float64(len(c.packets)) },
		FileBuff: func() float64 { return float64(c.willow.FSLen()) },
	})
	c.Logger = c
	c.Metrics = c.metrics
	c.willow = willow.NeWillow(c.Config)

	if c.HTTPServer == nil {
		c.HTTPServer = httpserver.DefaultConfig()
	}

	c.HTTPServer.Setup()
}

func (c *Config) setupSocket() error {
	addr, err := net.ResolveUDPAddr("udp", c.ListenAddr)
	if err != nil {
		return fmt.Errorf("invalid udp listen_addr: %w", err)
	}

	c.sock, err = net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("unable to use udp listen_addr: %w", err)
	}

	err = c.sock.SetReadBuffer(int(c.BufferUDP)) //nolint:gosec
	if err != nil {
		return fmt.Errorf("unable to set socket read buffer %d: %w", c.BufferUDP, err)
	}

	return nil
}

// Shutdown stops the application.
func (c *Config) Shutdown() error {
	// Stop accepting packets; wait for all listeners to finish
	// before closing the channel to avoid a send-on-closed-channel panic.
	c.sock.Close()
	c.listenerWg.Wait()
	close(c.packets)
	// Wait for all processors to finish their current Handler() call before
	// stopping, so no goroutine calls our methods after shutdown.
	c.processorWg.Wait()
	// Flush all file buffers to disk.
	c.willow.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	// This stops the main loop and the program exits.
	return c.httpSrv.Shutdown(ctx) //nolint:wrapcheck
}
