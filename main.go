package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Notifiarr/fogwillow/pkg/fog"
)

const shutdownWait = 100 * time.Millisecond

func main() {
	configFile := flag.String("config", "fog.conf", "config file path")
	flag.Parse()

	// Load configuration file.
	fog, err := fog.LoadConfigFile(*configFile)
	if err != nil {
		log.Fatalf("Config File Error: %s", err)
	}

	fog.SetupLogs()
	fog.PrintConfig()

	if err := fog.Start(); err != nil {
		log.Fatalf("Starting Fog Failed: %v", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	// Wait here for a signal to shut down.
	fog.Printf("Shutting down! Caught signal: %s", <-sigCh)
	fog.Shutdown()
	time.Sleep(shutdownWait)
}
