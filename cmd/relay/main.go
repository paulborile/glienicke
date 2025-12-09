package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/paul/glienicke/internal/store/sqlite"
	"github.com/paul/glienicke/pkg/relay"
)

func main() {
	addr := flag.String("addr", ":8080", "Address to listen on")
	dbPath := flag.String("db", "relay.db", "Path to SQLite database (will be created if it doesn't exist)")
	flag.Parse()

	// Autoconfigure SQLite storage
	expandedPath := expandPath(*dbPath)
	log.Printf("Using SQLite database: %s", expandedPath)

	store, err := sqlite.New(expandedPath)
	if err != nil {
		log.Fatalf("Failed to initialize SQLite store: %v", err)
	}
	defer store.Close()

	// Create relay
	r := relay.New(store)
	defer r.Close()

	// Handle shutdown gracefully
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Start relay in goroutine
	go func() {
		log.Printf("Starting Nostr relay v%s on %s", relay.Version, *addr)
		if err := r.Start(*addr); err != nil {
			log.Fatalf("Relay error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-sigCh
	log.Println("Shutting down relay...")
}

// expandPath expands ~ to home directory and makes path absolute
func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[1:])
		}
	}

	abs, err := filepath.Abs(path)
	if err == nil {
		return abs
	}
	return path
}
