package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/paul/glienicke/internal/store/sqlite"
	"github.com/paul/glienicke/pkg/config"
	"github.com/paul/glienicke/pkg/relay"
)

func main() {
	addr := flag.String("addr", ":8080", "Address to listen on")
	dbPath := flag.String("db", "relay.db", "Path to SQLite database (will be created if it doesn't exist)")
	certFile := flag.String("cert", "", "TLS certificate file for secure WebSocket (WSS)")
	keyFile := flag.String("key", "", "TLS private key file for secure WebSocket (WSS)")
	configPath := flag.String("config", "config/relay.yaml", "Path to rate limit configuration file")
	flag.Parse()

	// Load rate limit configuration
	rateLimitConfig, err := config.LoadRateLimitConfig(*configPath)
	if err != nil {
		log.Printf("Warning: Failed to load rate limit config from %s: %v", *configPath, err)
		log.Printf("Using default rate limit configuration")
		rateLimitConfig = config.DefaultRateLimitConfig()
	}

	// Validate configuration
	if err := config.ValidateRateLimitConfig(rateLimitConfig); err != nil {
		log.Fatalf("Invalid rate limit configuration: %v", err)
	}

	// Autoconfigure SQLite storage
	expandedPath := expandPath(*dbPath)
	log.Printf("Using SQLite database: %s", expandedPath)

	store, err := sqlite.New(expandedPath)
	if err != nil {
		log.Fatalf("Failed to initialize SQLite store: %v", err)
	}
	defer store.Close()

	// Create relay with configuration
	r := relay.New(store, rateLimitConfig)
	defer r.Close()

	// Handle shutdown gracefully
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Start relay in goroutine
	go func() {
		if *certFile != "" && *keyFile != "" {
			log.Printf("Starting Nostr relay v%s with TLS on %s (WSS)", relay.Version, *addr)
			log.Printf("Certificate: %s", *certFile)
			log.Printf("Private key: %s", *keyFile)
			if err := r.StartTLS(*addr, *certFile, *keyFile); err != nil {
				log.Fatalf("Relay error: %v", err)
			}
		} else {
			log.Printf("Starting Nostr relay v%s on %s (unencrypted WS)", relay.Version, *addr)
			log.Println("WARNING: Using unencrypted WebSocket connections. Use -cert and -key flags for production.")
			if err := r.Start(*addr); err != nil {
				log.Fatalf("Relay error: %v", err)
			}
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
