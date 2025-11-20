package relay

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"

	"encoding/json"
	"github.com/paul/glienicke/pkg/nips/nip11"
	"github.com/gorilla/websocket"
	"github.com/paul/glienicke/pkg/event"
	"github.com/paul/glienicke/pkg/protocol"
	"github.com/paul/glienicke/pkg/storage"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

// Relay is the main relay orchestrator
type Relay struct {
	store   storage.Store
	clients map[*protocol.Client]bool
	clientsMu sync.RWMutex
}

// New creates a new relay instance
func New(store storage.Store) *Relay {
	return &Relay{
		store:   store,
		clients: make(map[*protocol.Client]bool),
	}
}

// ServeHTTP handles WebSocket upgrade requests
func (r *Relay) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Header.Get("Accept") == "application/nostr+json" {
		info := &nip11.RelayInformationDocument{
			Name:          "Glienicke Nostr Relay",
			Description:   "A Nostr relay written in Go",
			Software:      "https://github.com/paul/glienicke",
			Version:       "v0.3.0",
			SupportedNIPs: []int{1, 9, 11},
		}

		w.Header().Set("Content-Type", "application/nostr+json")
		json.NewEncoder(w).Encode(info)
		return
	}

	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		http.Error(w, "WebSocket upgrade failed", http.StatusInternalServerError)
		return
	}

	client := protocol.NewClient(conn, r)

	r.clientsMu.Lock()
	r.clients[client] = true
	r.clientsMu.Unlock()

	defer func() {
		r.clientsMu.Lock()
		delete(r.clients, client)
		r.clientsMu.Unlock()
		client.Close()
	}()

	client.Start(req.Context())
}

// HandleEvent processes an EVENT message from a client
func (r *Relay) HandleEvent(ctx context.Context, c *protocol.Client, evt *event.Event) error {
	// NIP-09: Handle event deletion
	if evt.Kind == 5 {
		for _, tag := range evt.Tags {
			if len(tag) >= 2 && tag[0] == "e" {
				eventID := tag[1]
				if err := r.store.DeleteEvent(ctx, eventID, evt.PubKey); err != nil {
					// Log the error but don't stop processing other deletions
					log.Printf("Failed to delete event %s: %v", eventID, err)
				}
			}
		}
		// Do not send OK for deletion events, just process them.
		return nil
	}

	// Check for duplicate event
	existingEvent, err := r.store.GetEvent(ctx, evt.ID)
	if err != nil && err != storage.ErrNotFound {
		return fmt.Errorf("failed to check for existing event: %w", err)
	}
	if existingEvent != nil {
		// Event already exists, send OK with duplicate status
		c.SendOK(evt.ID, true, "duplicate: event already exists")
		return nil
	}

	// Save to storage
	if err := r.store.SaveEvent(ctx, evt); err != nil {
		c.SendOK(evt.ID, false, fmt.Sprintf("error: failed to save event: %v", err))
		return fmt.Errorf("failed to save event: %w", err)
	}

	// Send OK message
	c.SendOK(evt.ID, true, "")

	// Broadcast to subscribed clients
	r.broadcastEvent(evt)

	return nil
}

// HandleReq processes a REQ message from a client
func (r *Relay) HandleReq(ctx context.Context, c *protocol.Client, subID string, filters []*event.Filter) error {
	// Query stored events matching filters
	events, err := r.store.QueryEvents(ctx, filters)
	if err != nil {
		return fmt.Errorf("failed to query events: %w", err)
	}

	// Send stored events to the client
	for _, evt := range events {
		if err := c.SendEvent(subID, evt); err != nil {
			log.Printf("Failed to send stored event to client: %v", err)
			// May want to terminate connection here depending on error
		}
	}

	// Send EOSE to indicate end of stored events
	if err := c.SendEOSE(subID); err != nil {
		log.Printf("Failed to send EOSE to client: %v", err)
	}

	log.Printf("Sent %d stored events for subscription %s", len(events), subID)
	return nil
}

// HandleClose processes a CLOSE message from a client
func (r *Relay) HandleClose(ctx context.Context, c *protocol.Client, subID string) error {
	log.Printf("Closing subscription %s for client %s", subID, c.RemoteAddr())
	c.RemoveSubscription(subID)
	return nil
}

// broadcastEvent sends an event to all clients with matching subscriptions
func (r *Relay) broadcastEvent(evt *event.Event) {
	r.clientsMu.RLock()
	defer r.clientsMu.RUnlock()

	for client := range r.clients {
		go func(c *protocol.Client) {
			subs := c.GetSubscriptions()
			for subID, filters := range subs {
				// Check if event matches any filter
				for _, filter := range filters {
					if evt.Matches(filter) {
						if err := c.SendEvent(subID, evt); err != nil {
							log.Printf("Failed to send event to client: %v", err)
						}
						return // Only send once per subscription
					}
				}
			}
		}(client)
	}
}

// Close shuts down the relay
func (r *Relay) Close() error {
	r.clientsMu.Lock()
	defer r.clientsMu.Unlock()

	// Close all clients
	for client := range r.clients {
		client.Close()
	}

	return r.store.Close()
}

// Start starts the relay HTTP server
func (r *Relay) Start(addr string) error {
	http.Handle("/", r)
	log.Printf("Relay starting on %s", addr)
	return http.ListenAndServe(addr, nil)
}
