package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/paul/glienicke/internal/store/memory"
	"github.com/paul/glienicke/pkg/relay"
)

const Version = "0.2.0"

func main() {
	addr := flag.String("addr", ":8080", "Address to listen on")
	flag.Parse()

	// Create in-memory store (use a persistent store in production)
	store := memory.New()
	defer store.Close()

	// Create relay
	r := relay.New(store)
	defer r.Close()

	// Handle shutdown gracefully
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Start relay in goroutine
	go func() {
		log.Printf("Starting Nostr relay v%s on %s", Version, *addr)
		if err := r.Start(*addr); err != nil {
			log.Fatalf("Relay error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-sigCh
	log.Println("Shutting down relay...")
}
