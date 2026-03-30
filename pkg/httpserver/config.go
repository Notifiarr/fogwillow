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

func defaultConfig() *Config {
	return &Config{
		ListenAddr:        DefaultListenAddr,
		ReadTimeout:       DefaultReadTimeout,
		ReadHeaderTimeout: DefaultReadHeaderTimeout,
		WriteTimeout:      DefaultWriteTimeout,
		IdleTimeout:       DefaultIdleTimeout,
		MaxHeaderBytes:    DefaultMaxHeaderBytes,
	}
}

func validateConfig(config *Config) {
	if config.ReadTimeout <= 0 {
		config.ReadTimeout = DefaultReadTimeout
	}

	if config.ReadHeaderTimeout <= 0 {
		config.ReadHeaderTimeout = DefaultReadHeaderTimeout
	}

	if config.WriteTimeout <= 0 {
		config.WriteTimeout = DefaultWriteTimeout
	}

	if config.IdleTimeout <= 0 {
		config.IdleTimeout = DefaultIdleTimeout
	}

	if config.MaxHeaderBytes <= 0 {
		config.MaxHeaderBytes = DefaultMaxHeaderBytes
	}
}
