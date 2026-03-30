package httpserver

import "time"

// HTTP server defaults.
const (
	DefaultListenAddr        = ":9000"
	DefaultReadTimeout       = 10 * time.Second
	DefaultReadHeaderTimeout = 2 * time.Second
	DefaultWriteTimeout      = 10 * time.Second
	DefaultIdleTimeout       = 120 * time.Second
	DefaultMaxHeaderBytes    = 1 << 16 // 64KB
)

// Config is the configuration for the HTTP server.
type Config struct {
	ListenAddr        string        `toml:"listen_addr"         xml:"listen_addr"`
	ReadTimeout       time.Duration `toml:"read_timeout"        xml:"read_timeout"`
	ReadHeaderTimeout time.Duration `toml:"read_header_timeout" xml:"read_header_timeout"`
	WriteTimeout      time.Duration `toml:"write_timeout"       xml:"write_timeout"`
	IdleTimeout       time.Duration `toml:"idle_timeout"        xml:"idle_timeout"`
	MaxHeaderBytes    int           `toml:"max_header_bytes"    xml:"max_header_bytes"`
	TLSCertPath       string        `toml:"tls_cert_path"       xml:"tls_cert_path"`
	TLSKeyPath        string        `toml:"tls_key_path"        xml:"tls_key_path"`
}

// DefaultConfig returns the default configuration for the HTTP server.
func DefaultConfig() *Config {
	return &Config{
		ListenAddr:        DefaultListenAddr,
		ReadTimeout:       DefaultReadTimeout,
		ReadHeaderTimeout: DefaultReadHeaderTimeout,
		WriteTimeout:      DefaultWriteTimeout,
		IdleTimeout:       DefaultIdleTimeout,
		MaxHeaderBytes:    DefaultMaxHeaderBytes,
	}
}

// Setup fills empty listen address and any zero timeouts or limits so logs and runtime match.
func (c *Config) Setup() {
	if c.ListenAddr == "" {
		c.ListenAddr = DefaultListenAddr
	}

	if c.ReadTimeout <= 0 {
		c.ReadTimeout = DefaultReadTimeout
	}

	if c.ReadHeaderTimeout <= 0 {
		c.ReadHeaderTimeout = DefaultReadHeaderTimeout
	}

	if c.WriteTimeout <= 0 {
		c.WriteTimeout = DefaultWriteTimeout
	}

	if c.IdleTimeout <= 0 {
		c.IdleTimeout = DefaultIdleTimeout
	}

	if c.MaxHeaderBytes <= 0 {
		c.MaxHeaderBytes = DefaultMaxHeaderBytes
	}
}
