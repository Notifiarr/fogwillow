// Package httpserver owns the HTTP listener used for Prometheus metrics and future API routes.
// It is separate from the UDP ingest path and from pkg/willow.
package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server wraps http.Server with a shared ServeMux for /metrics and optional handlers.
type Server struct {
	Mux         *http.ServeMux
	srv         *http.Server
	TLSCertPath string
	TLSKeyPath  string
}

// New builds an HTTP server on addr. register is optional; use it to attach API or other routes
// to the same mux (e.g. mux.Handle("/api/", apiHandler)). /metrics is always registered.
func New(config *Config, register func(mux *http.ServeMux)) *Server {
	if config == nil {
		config = DefaultConfig()
	}

	config.Setup()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	if register != nil {
		register(mux)
	}

	return &Server{
		Mux:         mux,
		TLSCertPath: config.TLSCertPath,
		TLSKeyPath:  config.TLSKeyPath,
		srv: &http.Server{
			Handler:           mux,
			Addr:              config.ListenAddr,
			ReadTimeout:       config.ReadTimeout,
			ReadHeaderTimeout: config.ReadHeaderTimeout,
			WriteTimeout:      config.WriteTimeout,
			IdleTimeout:       config.IdleTimeout,
			MaxHeaderBytes:    config.MaxHeaderBytes,
		},
	}
}

// ListenAndServe binds and serves until Shutdown is called or the listener errors.
func (s *Server) ListenAndServe() error {
	var err error

	if s.TLSCertPath != "" || s.TLSKeyPath != "" {
		err = s.srv.ListenAndServeTLS(s.TLSCertPath, s.TLSKeyPath)
	} else {
		err = s.srv.ListenAndServe()
	}

	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http listen: %w", err)
	}

	return nil
}

// Shutdown stops the server gracefully.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx) //nolint:wrapcheck
}
