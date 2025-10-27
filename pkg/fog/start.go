// Package fog is the main package for the Fogwillow application.
package fog

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/Notifiarr/fogwillow/pkg/buf"
	"github.com/Notifiarr/fogwillow/pkg/metrics"
	"github.com/Notifiarr/fogwillow/pkg/willow"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golift.io/cnfg"
	"golift.io/cnfgfile"
)

// Some application defaults.
const (
	DefaultOutputPath   = "/tmp"
	DefaultListenAddr   = ":9000"
	DefaultUDPBuffer    = 1024 * 1024
	DefaultPacketBuffer = 1024 * 8
	DefaultChanBuffer   = 1024
)

// Config is the input _and_ running data.
type Config struct {
	*willow.Config
	Password     string `toml:"password"      xml:"password"`
	OutputPath   string `toml:"output_path"   xml:"output_path"`
	ListenAddr   string `toml:"listen_addr"   xml:"listen_addr"`
	LogFile      string `toml:"log_file"      xml:"log_file"`
	LogFileMB    uint   `toml:"log_file_mb"   xml:"log_file_mb"`
	LogFiles     uint   `toml:"log_files"     xml:"log_files"`
	BufferUDP    uint   `toml:"buffer_udp"    xml:"buffer_udp"`
	BufferPacket uint   `toml:"buffer_packet" xml:"buffer_packet"`
	BufferChan   uint   `toml:"buffer_chan"   xml:"buffer_chan"`
	Listeners    uint   `toml:"listeners"     xml:"listeners"`
	Processors   uint   `toml:"processors"    xml:"processors"`
	Debug        bool   `toml:"debug"         xml:"debug"`
	log          *log.Logger
	packets      chan *packet
	sock         *net.UDPConn
	willow       *willow.Willow
	metrics      *metrics.Metrics
	httpSrv      *http.Server
	newBuf       func(path string, data []byte) *buf.FileBuffer
	bytesPool    sync.Pool
}

// LoadConfigFile does what its name implies.
func LoadConfigFile(path string) (*Config, error) {
	config := &Config{
		OutputPath:   DefaultOutputPath,
		ListenAddr:   DefaultListenAddr,
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

	for i := c.Processors; i > 0; i-- {
		go c.packetProcessor(i)
	}

	for i := c.Listeners; i > 0; i-- {
		go c.packetListener(i)
	}

	smx := http.NewServeMux()
	smx.Handle("/metrics", promhttp.Handler())

	c.httpSrv = &http.Server{
		Handler:           smx,
		Addr:              c.ListenAddr,
		ReadTimeout:       time.Second,
		ReadHeaderTimeout: time.Second,
		WriteTimeout:      time.Second,
		IdleTimeout:       20 * time.Second, //nolint:mnd
	}

	err = c.httpSrv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("web server failed: %w", err)
	}

	return nil
}

// setup makes sure configurations are sound and sane.
func (c *Config) setup() {
	if c.Processors < 1 {
		c.Processors = 1
	}

	if c.Listeners < 1 {
		c.Listeners = 1
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
}

func (c *Config) setupSocket() error {
	addr, err := net.ResolveUDPAddr("udp", c.ListenAddr)
	if err != nil {
		return fmt.Errorf("invalid listen_addr provided: %w", err)
	}

	c.sock, err = net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("unable to use provided listen_addr: %w", err)
	}

	err = c.sock.SetReadBuffer(int(c.BufferUDP)) //nolint:gosec
	if err != nil {
		return fmt.Errorf("unable to set socket read buffer %d: %w", c.BufferUDP, err)
	}

	return nil
}

// Shutdown stops the application.
func (c *Config) Shutdown() error {
	// Stop accepting packets.
	c.sock.Close()
	close(c.packets)
	// Flush all file buffers to disk.
	c.willow.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	// This stops the main loop and the program exits.
	return c.httpSrv.Shutdown(ctx) //nolint:wrapcheck
}
