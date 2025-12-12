package relay

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"

	"encoding/json"

	"github.com/gorilla/websocket"
	"github.com/nbd-wtf/go-nostr"
	"github.com/paul/glienicke/pkg/event"
	"github.com/paul/glienicke/pkg/nips/nip02"
	"github.com/paul/glienicke/pkg/nips/nip09"
	"github.com/paul/glienicke/pkg/nips/nip11"
	"github.com/paul/glienicke/pkg/nips/nip40"
	"github.com/paul/glienicke/pkg/nips/nip42"
	"github.com/paul/glienicke/pkg/nips/nip44"
	"github.com/paul/glienicke/pkg/nips/nip50"
	"github.com/paul/glienicke/pkg/nips/nip59"
	"github.com/paul/glienicke/pkg/nips/nip65"
	"github.com/paul/glienicke/pkg/protocol"
	"github.com/paul/glienicke/pkg/storage"
)

// Version of the relay
const Version = "0.10.0"

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

// Relay is the main relay orchestrator
type Relay struct {
	store     storage.Store
	clients   map[*protocol.Client]bool
	clientsMu sync.RWMutex
	version   string
}

// New creates a new relay instance
func New(store storage.Store) *Relay {
	return &Relay{
		store:   store,
		clients: make(map[*protocol.Client]bool),
		version: Version,
	}
}

// ServeHTTP handles WebSocket upgrade requests
func (r *Relay) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Header.Get("Accept") == "application/nostr+json" {
		info := &nip11.RelayInformationDocument{
			Name:          "Glienicke Nostr Relay",
			Description:   "A Nostr relay written in Go",
			Software:      "https://github.com/paul/glienicke",
			Version:       r.version,
			SupportedNIPs: []int{1, 2, 9, 11, 17, 40, 42, 44, 50, 59, 65},
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
	// NIP-42: Handle AUTH events
	if nip42.IsAuthEvent(evt) {
		if err := nip42.ValidateAuthEvent(evt); err != nil {
			c.SendOK(evt.ID, false, fmt.Sprintf("invalid AUTH: %v", err))
			return fmt.Errorf("invalid AUTH event: %w", err)
		}
		c.SendOK(evt.ID, true, "authenticated")
		return nil
	}

	// NIP-02: Validate follow list events
	if nip02.IsFollowListEvent(evt) {
		if err := nip02.ValidateFollowList(evt); err != nil {
			c.SendOK(evt.ID, false, fmt.Sprintf("invalid follow list: %v", err))
			return fmt.Errorf("invalid follow list event: %w", err)
		}
	}

	// NIP-65: Validate relay list events
	if nip65.IsRelayListEvent(evt) {
		if err := nip65.ValidateRelayList(evt); err != nil {
			c.SendOK(evt.ID, false, fmt.Sprintf("invalid relay list: %v", err))
			return fmt.Errorf("invalid relay list event: %w", err)
		}
	}

	// NIP-40: Check for expired events
	if nip40.ShouldRejectEvent(evt) {
		c.SendOK(evt.ID, false, "event has expired")
		return fmt.Errorf("event has expired")
	}

	// NIP-09: Handle event deletion
	if evt.Kind == 5 {
		if err := nip09.HandleDeletion(ctx, r.store, evt); err != nil {
			log.Printf("NIP-09 deletion handling failed: %v", err)
		}
		return nil
	}

	// NIP-59: Handle gift wrap events
	nostrEvt := convertLocalEventToNostrEvent(evt)
	if nip59.IsGiftWrap(nostrEvt) {
		// For gift wrap events, we don't unwrap or validate the inner event.
		// We just store it and broadcast it to the recipient.
		if err := r.store.SaveEvent(ctx, evt); err != nil {
			c.SendOK(evt.ID, false, fmt.Sprintf("error: failed to save event: %v", err))
			return fmt.Errorf("failed to save gift wrap event: %w", err)
		}
		c.SendOK(evt.ID, true, "")
		r.broadcastEvent(evt)
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
	var events []*event.Event
	var err error

	// Check if any filter has search field (NIP-50)
	hasSearch := false
	for _, filter := range filters {
		if filter.Search != "" {
			hasSearch = true
			break
		}
	}

	if hasSearch {
		// Use NIP-50 search
		events, err = nip50.SearchEvents(ctx, r.store, filters)
		if err != nil {
			return fmt.Errorf("failed to search events: %w", err)
		}
	} else {
		// Use regular query
		events, err = r.store.QueryEvents(ctx, filters)
		if err != nil {
			return fmt.Errorf("failed to query events: %w", err)
		}
	}

	// Send stored events to the client, filtering out expired events
	for _, evt := range events {
		// NIP-40: Filter out expired events
		if nip40.ShouldFilterEvent(evt) {
			continue
		}
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
			// NIP-40: Filter out expired events
			if nip40.ShouldFilterEvent(evt) {
				return
			}

			// NIP-44: Encrypted Direct Messages (kind 4)
			if nip44.IsEncryptedDirectMessage(evt) {
				recipientPubKey, found := nip44.GetRecipientPubKey(evt)
				if !found || !c.HasSubscriptionToPubKey(recipientPubKey) {
					return // Don't broadcast if not the recipient or not subscribed to recipient
				}
			}

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

// convertLocalEventToNostrEvent converts a local_event.Event to a nostr.Event
func convertLocalEventToNostrEvent(le *event.Event) *nostr.Event {
	if le == nil {
		return nil
	}

	nostrEvt := &nostr.Event{
		ID:        le.ID,
		PubKey:    le.PubKey,
		CreatedAt: nostr.Timestamp(le.CreatedAt),
		Kind:      le.Kind,
		Tags:      make(nostr.Tags, len(le.Tags)),
		Content:   le.Content,
		Sig:       le.Sig,
	}

	for i, tag := range le.Tags {
		nostrEvt.Tags[i] = nostr.Tag(tag)
	}

	return nostrEvt
}
