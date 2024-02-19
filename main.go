package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Notifiarr/fogwillow/pkg/fog"
)

func main() {
	configFile := flag.String("config", "/config/fog.conf", "config file path")
	flag.Parse()

	// Load configuration file.
	fog, err := fog.LoadConfigFile(*configFile)
	if err != nil {
		log.Fatalf("Config File Error: %s", err)
	}

	go catchSignal(fog)

	if err := fog.Start(); err != nil {
		log.Fatalf("Starting Fog Failed: %v", err)
	}

	fog.Printf("Done. Good bye!")
}

func catchSignal(fog *fog.Config) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	// Wait here for a signal to shut down.
	fog.Printf("Shutting down! Caught signal: %s", <-sigCh)

	if err := fog.Shutdown(); err != nil {
		log.Fatalf("Stopping Fog Failed: %v", err)
	}
}
