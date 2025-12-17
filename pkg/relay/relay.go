package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nbd-wtf/go-nostr"
	"github.com/paul/glienicke/pkg/config"
	"github.com/paul/glienicke/pkg/event"
	"github.com/paul/glienicke/pkg/nips/nip02"
	"github.com/paul/glienicke/pkg/nips/nip09"
	"github.com/paul/glienicke/pkg/nips/nip11"
	"github.com/paul/glienicke/pkg/nips/nip22"
	"github.com/paul/glienicke/pkg/nips/nip25"
	"github.com/paul/glienicke/pkg/nips/nip40"
	"github.com/paul/glienicke/pkg/nips/nip42"
	"github.com/paul/glienicke/pkg/nips/nip44"
	"github.com/paul/glienicke/pkg/nips/nip50"
	"github.com/paul/glienicke/pkg/nips/nip59"
	"github.com/paul/glienicke/pkg/nips/nip62"
	"github.com/paul/glienicke/pkg/nips/nip65"
	"github.com/paul/glienicke/pkg/protocol"
	"github.com/paul/glienicke/pkg/ratelimit"
	"github.com/paul/glienicke/pkg/storage"
)

// Version of the relay
const Version = "0.15.1"

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

// Relay is the main relay orchestrator
type Relay struct {
	store             storage.Store
	clients           map[*protocol.Client]bool
	clientsMu         sync.RWMutex
	version           string
	config            *config.RateLimitConfig
	rateLimiters      map[string]*ratelimit.Limiter
	ipLimitersMu      sync.RWMutex
	ipConnections     map[string]int
	globalConnections int
	connMu            sync.RWMutex
}

// New creates a new relay instance
func New(store storage.Store, config *config.RateLimitConfig) *Relay {
	return &Relay{
		store:         store,
		clients:       make(map[*protocol.Client]bool),
		version:       Version,
		config:        config,
		rateLimiters:  make(map[string]*ratelimit.Limiter),
		ipConnections: make(map[string]int),
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
			SupportedNIPs: []int{1, 2, 4, 9, 11, 17, 22, 25, 40, 42, 44, 45, 50, 59, 62, 65},
		}

		w.Header().Set("Content-Type", "application/nostr+json")
		json.NewEncoder(w).Encode(info)
		return
	}

	// Get client IP for connection limiting
	clientIP := r.getClientIP(req)

	// Check connection limits before upgrading
	if !r.canAcceptConnection(clientIP) {
		log.Printf("Connection rejected for %s: connection limit exceeded", clientIP)
		http.Error(w, "Connection limit exceeded", http.StatusTooManyRequests)
		return
	}

	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		http.Error(w, "WebSocket upgrade failed", http.StatusInternalServerError)
		return
	}

	client := protocol.NewClient(conn, r)

	// Register connection
	r.addConnection(clientIP)
	r.clientsMu.Lock()
	r.clients[client] = true
	r.clientsMu.Unlock()

	defer func() {
		r.clientsMu.Lock()
		delete(r.clients, client)
		r.clientsMu.Unlock()
		r.removeConnection(clientIP)
		client.Close()
	}()

	client.Start(req.Context())
}

// HandleEvent processes an EVENT message from a client
func (r *Relay) HandleEvent(ctx context.Context, c *protocol.Client, evt *event.Event) error {
	// Check rate limits
	if !r.checkRateLimit(c.RemoteAddr(), "event") {
		c.SendOK(evt.ID, false, "rate-limited: too many events")
		return fmt.Errorf("rate limit exceeded for events from %s", c.RemoteAddr())
	}

	// Validate event size
	if !r.validateEventSize(evt) {
		c.SendOK(evt.ID, false, "invalid: event too large")
		return fmt.Errorf("event size exceeds limits")
	}

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

	// NIP-22: Validate comment events
	if nip22.IsCommentEvent(evt) {
		if err := nip22.ValidateComment(evt); err != nil {
			c.SendOK(evt.ID, false, fmt.Sprintf("invalid comment: %v", err))
			return fmt.Errorf("invalid comment event: %w", err)
		}
	}

	// NIP-25: Validate reaction events
	if nip25.IsReactionEvent(evt) {
		if err := nip25.ValidateReaction(evt); err != nil {
			c.SendOK(evt.ID, false, fmt.Sprintf("invalid reaction: %v", err))
			return fmt.Errorf("invalid reaction event: %w", err)
		}
	}

	// NIP-65: Validate relay list events
	if nip65.IsRelayListEvent(evt) {
		if err := nip65.ValidateRelayList(evt); err != nil {
			c.SendOK(evt.ID, false, fmt.Sprintf("invalid relay list: %v", err))
			return fmt.Errorf("invalid relay list event: %w", err)
		}
	}

	// NIP-62: Validate Request to Vanish events
	if nip62.IsRequestToVanishEvent(evt) {
		if err := nip62.ValidateRequestToVanish(evt); err != nil {
			c.SendOK(evt.ID, false, fmt.Sprintf("invalid Request to Vanish: %v", err))
			return fmt.Errorf("invalid Request to Vanish event: %w", err)
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

	// NIP-62: Handle Request to Vanish events
	if nip62.IsRequestToVanishEvent(evt) {
		// Get the relay URL from the request or use default
		relayURL := "ws://localhost:8080" // This should be configurable in production
		if err := nip62.HandleRequestToVanish(ctx, r.store, evt, relayURL); err != nil {
			log.Printf("NIP-62 Request to Vanish handling failed: %v", err)
			c.SendOK(evt.ID, false, fmt.Sprintf("error: failed to process Request to Vanish: %v", err))
			return fmt.Errorf("failed to process Request to Vanish: %w", err)
		}
		c.SendOK(evt.ID, true, "Request to Vanish processed")
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
	// Check rate limits
	if !r.checkRateLimit(c.RemoteAddr(), "req") {
		c.SendClosed(subID, "rate-limited: too many requests")
		return fmt.Errorf("rate limit exceeded for requests from %s", c.RemoteAddr())
	}

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

// HandleCount processes a COUNT message from a client (NIP-45)
func (r *Relay) HandleCount(ctx context.Context, c *protocol.Client, countID string, filters []*event.Filter) error {
	log.Printf("Received COUNT request %s from client %s", countID, c.RemoteAddr())

	// Check rate limits
	if !r.checkRateLimit(c.RemoteAddr(), "count") {
		c.SendClosed(countID, "rate-limited: too many count requests")
		return fmt.Errorf("rate limit exceeded for count requests from %s", c.RemoteAddr())
	}

	// Validate filters
	if len(filters) == 0 {
		c.SendClosed(countID, "error: no filters provided")
		return fmt.Errorf("COUNT request requires at least one filter")
	}

	// Get count from storage
	count, err := r.store.CountEvents(ctx, filters)
	if err != nil {
		c.SendClosed(countID, fmt.Sprintf("error: failed to count events: %v", err))
		return fmt.Errorf("failed to count events: %w", err)
	}

	// Send count response
	// For now, we don't implement approximate counting, but could be added later for performance
	err = c.SendCount(countID, count, false)
	if err != nil {
		return fmt.Errorf("failed to send COUNT response: %w", err)
	}

	log.Printf("COUNT request %s returned %d events", countID, count)
	return nil
}

// parseRateLimit parses rate limit string like "1000/s" or "1/minute"
func (r *Relay) parseRateLimit(rateStr string) (tokensPerSecond float64, err error) {
	if rateStr == "" {
		return 0, fmt.Errorf("empty rate limit string")
	}

	parts := strings.Split(rateStr, "/")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid rate limit format: %s", rateStr)
	}

	tokens, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid token count: %s", parts[0])
	}

	unit := strings.ToLower(parts[1])
	var interval time.Duration
	switch unit {
	case "s", "sec", "second":
		interval = time.Second
	case "m", "min", "minute":
		interval = time.Minute
	case "h", "hr", "hour":
		interval = time.Hour
	default:
		return 0, fmt.Errorf("unknown time unit: %s", unit)
	}

	tokensPerSecond = tokens / interval.Seconds()
	return tokensPerSecond, nil
}

// getLimiter gets or creates a rate limiter for a specific key
func (r *Relay) getLimiter(key, rateLimit string) *ratelimit.Limiter {
	r.ipLimitersMu.Lock()
	defer r.ipLimitersMu.Unlock()

	limiter, exists := r.rateLimiters[key]
	if !exists {
		tokensPerSecond, err := r.parseRateLimit(rateLimit)
		if err != nil {
			// Default to 10 tokens per second on error
			limiter = ratelimit.New(10, 10)
		} else {
			capacity := int64(tokensPerSecond)
			if capacity == 0 {
				capacity = 1
			}
			limiter = ratelimit.New(tokensPerSecond, capacity)
		}
		r.rateLimiters[key] = limiter
	}

	return limiter
}

// checkRateLimit checks if a request should be rate limited
func (r *Relay) checkRateLimit(clientAddr, requestType string) bool {
	var limitStr string

	switch requestType {
	case "event":
		// Check per-IP event limit
		limitStr = r.config.IPEventLimit
		limiter := r.getLimiter(clientAddr+":event", limitStr)
		if !limiter.Allow() {
			return false
		}

		// Check global event limit
		globalLimiter := r.getLimiter("global:event", r.config.GlobalEventLimit)
		return globalLimiter.Allow()

	case "req":
		// Check per-IP REQ limit
		limitStr = r.config.IPReqLimit
		limiter := r.getLimiter(clientAddr+":req", limitStr)
		if !limiter.Allow() {
			return false
		}

		// Check global REQ limit
		globalLimiter := r.getLimiter("global:req", r.config.GlobalReqLimit)
		return globalLimiter.Allow()

	case "count":
		// Check per-IP COUNT limit
		limitStr = r.config.IPCountLimit
		limiter := r.getLimiter(clientAddr+":count", limitStr)
		if !limiter.Allow() {
			return false
		}

		// Check global COUNT limit
		globalLimiter := r.getLimiter("global:count", r.config.GlobalCountLimit)
		return globalLimiter.Allow()

	default:
		return true // Unknown request type, allow
	}
}

// validateEventSize checks if event size is within limits
func (r *Relay) validateEventSize(evt *event.Event) bool {
	// Check event size in bytes
	if r.config.MaxEventSize > 0 && len(evt.Content) > r.config.MaxEventSize {
		return false
	}

	// Check content length in characters
	if r.config.MaxContentLength > 0 && len([]rune(evt.Content)) > r.config.MaxContentLength {
		return false
	}

	return true
}

// canAcceptConnection checks if a new connection from the given IP should be allowed
func (r *Relay) canAcceptConnection(clientAddr string) bool {
	r.connMu.Lock()
	defer r.connMu.Unlock()

	// Check global connection limit
	if r.globalConnections >= r.config.MaxGlobal {
		return false
	}

	// Check per-IP connection limit
	currentConnections := r.ipConnections[clientAddr]
	if currentConnections >= r.config.MaxPerIP {
		return false
	}

	return true
}

// addConnection registers a new connection
func (r *Relay) addConnection(clientAddr string) {
	r.connMu.Lock()
	defer r.connMu.Unlock()

	r.globalConnections++
	r.ipConnections[clientAddr]++
}

// removeConnection removes a connection
func (r *Relay) removeConnection(clientAddr string) {
	r.connMu.Lock()
	defer r.connMu.Unlock()

	r.globalConnections--
	if r.globalConnections < 0 {
		r.globalConnections = 0
	}

	r.ipConnections[clientAddr]--
	if r.ipConnections[clientAddr] <= 0 {
		delete(r.ipConnections, clientAddr)
	}
}

// getClientIP extracts the real client IP from request
func (r *Relay) getClientIP(req *http.Request) string {
	// Check for X-Forwarded-For header (proxy)
	if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
		// Take first IP if multiple
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// Check for X-Real-IP header
	if xri := req.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	ip := req.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return strings.TrimSpace(ip)
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

// StartTLS starts the relay HTTPS server with TLS certificates
func (r *Relay) StartTLS(addr, certFile, keyFile string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", r.ServeHTTP)

	log.Printf("Relay starting with TLS on %s", addr)
	log.Printf("Certificate file: %s", certFile)
	log.Printf("Private key file: %s", keyFile)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return server.ListenAndServeTLS(certFile, keyFile)
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
