package relay

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"encoding/json"

	"github.com/gorilla/websocket"
	"github.com/nbd-wtf/go-nostr"
	"github.com/paul/glienicke/internal/store/memory"
	"github.com/paul/glienicke/internal/store/sqlite"
	"github.com/paul/glienicke/pkg/event"
	"github.com/paul/glienicke/pkg/nips/nip02"
	"github.com/paul/glienicke/pkg/nips/nip09"
	"github.com/paul/glienicke/pkg/nips/nip11"
	"github.com/paul/glienicke/pkg/nips/nip22"
	"github.com/paul/glienicke/pkg/nips/nip25"
	"github.com/paul/glienicke/pkg/nips/nip28"
	"github.com/paul/glienicke/pkg/nips/nip40"
	"github.com/paul/glienicke/pkg/nips/nip42"
	"github.com/paul/glienicke/pkg/nips/nip44"
	"github.com/paul/glienicke/pkg/nips/nip50"
	"github.com/paul/glienicke/pkg/nips/nip59"
	"github.com/paul/glienicke/pkg/nips/nip62"
	"github.com/paul/glienicke/pkg/nips/nip65"
	"github.com/paul/glienicke/pkg/protocol"
	"github.com/paul/glienicke/pkg/storage"
)

// ChannelStore defines the interface for storing channel events
type ChannelStore interface {
	SaveChannelEvent(ctx context.Context, evt *event.Event) error
	QueryChannelEvents(ctx context.Context, channelID string, since, until *int64, limit *int) ([]*event.Event, error)
}

// Version of the relay
const Version = "0.19.5"

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

// Metrics holds relay monitoring metrics
type Metrics struct {
	mu               sync.RWMutex
	startTime        time.Time
	totalConnections int64
	totalEvents      int64
	totalRequests    int64
	packetsPerSecond float64
	lastPacketTime   time.Time
	packetCount      int64
	lastPacketReset  time.Time
	rateLimitedCount int64
	memoryUsage      uint64
	dbStatus         string
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status            string  `json:"status"`
	UptimeSeconds     float64 `json:"uptime_seconds"`
	Version           string  `json:"version"`
	ActiveConnections int     `json:"active_connections"`
	TotalConnections  int64   `json:"total_connections"`
	TotalEvents       int64   `json:"total_events"`
	TotalRequests     int64   `json:"total_requests"`
	PacketsPerSecond  float64 `json:"packets_per_second"`
	RateLimitedCount  int64   `json:"rate_limited_count"`
	MemoryUsageMB     float64 `json:"memory_usage_mb"`
	DatabaseStatus    string  `json:"database_status"`
	Timestamp         string  `json:"timestamp"`
}

// ipRateLimiter tracks per-IP REQ rate using a token bucket and ban state
type ipRateLimiter struct {
	tokens     float64
	lastRefill time.Time
	violations int
	bannedAt   time.Time
	pubkeys    map[string]bool // authenticated pubkeys seen from this IP
}

// Relay is the main relay orchestrator
type Relay struct {
	store           storage.Store
	clients         map[*protocol.Client]bool
	clientsMu       sync.RWMutex
	version         string
	metrics         *Metrics
	mux             *http.ServeMux
	ipLimiters      map[string]*ipRateLimiter
	ipLimiterMu     sync.Mutex
	maxEventsPerREQ  int
	rateLimitEnabled bool
	requireAuth      bool // NIP-42: require authentication before allowing REQ/EVENT
	closeAfterEOSE   bool // Auto-close subscriptions after sending stored events
}

// New creates a new relay instance
func New(store storage.Store) *Relay {
	// Rate limiting enabled by default; disable with GLIENICKE_RATE_LIMIT_ENABLED=false
	rlEnabled := true
	if v := os.Getenv("GLIENICKE_RATE_LIMIT_ENABLED"); v == "false" || v == "0" {
		rlEnabled = false
		log.Println("Rate limiting DISABLED via GLIENICKE_RATE_LIMIT_ENABLED")
	}

	r := &Relay{
		store:            store,
		clients:          make(map[*protocol.Client]bool),
		version:          Version,
		ipLimiters:       make(map[string]*ipRateLimiter),
		maxEventsPerREQ:  defaultMaxEventsPerREQ,
		rateLimitEnabled: rlEnabled,
		requireAuth:      true,
		metrics: &Metrics{
			startTime:       time.Now(),
			dbStatus:        "unknown",
			lastPacketReset: time.Now(),
		},
		mux: http.NewServeMux(),
	}

	// Setup HTTP routes
	r.setupRoutes()

	return r
}

// SetMaxEventsPerREQ sets the maximum number of events returned per REQ response.
func (r *Relay) SetMaxEventsPerREQ(max int) {
	r.maxEventsPerREQ = max
}

// SetRequireAuth enables NIP-42 authentication requirement.
func (r *Relay) SetRequireAuth(enabled bool) {
	r.requireAuth = enabled
}

// SetCloseAfterEOSE enables auto-closing subscriptions after EOSE is sent.
// This frees subscription slots but prevents live event delivery on those subscriptions.
func (r *Relay) SetCloseAfterEOSE(enabled bool) {
	r.closeAfterEOSE = enabled
}

// setupRoutes configures HTTP routes for the relay
func (r *Relay) setupRoutes() {
	r.mux.HandleFunc("/", r.ServeHTTP)
	r.mux.HandleFunc("/health", r.HealthHandler)
}

// ServeHTTP handles WebSocket upgrade requests
func (r *Relay) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Header.Get("Accept") == "application/nostr+json" {
		info := &nip11.RelayInformationDocument{
			Name:          "Glienicke Nostr Relay",
			Description:   "Glienicke - a Nostr relay written in Go",
			Software:      "https://github.com/paul/glienicke",
			Version:       r.version,
			SupportedNIPs: []int{1, 2, 4, 9, 11, 17, 22, 25, 40, 42, 44, 45, 50, 59, 62, 65},
			Icon:          "https://www.paulstephenborile.com/wp-content/uploads/2026/02/cropped-logo-only.png",
		}

		w.Header().Set("Content-Type", "application/nostr+json")
		json.NewEncoder(w).Encode(info)
		return
	}

	// Extract real client IP from proxy headers (before upgrade, so we can reject banned IPs)
	realIP := req.Header.Get("X-Forwarded-For")
	if realIP != "" {
		// X-Forwarded-For can contain multiple IPs; first one is the original client
		if idx := strings.Index(realIP, ","); idx != -1 {
			realIP = strings.TrimSpace(realIP[:idx])
		}
	} else if realIP = req.Header.Get("X-Real-IP"); realIP == "" {
		// No proxy headers — use the direct connection IP (strip port)
		realIP = req.RemoteAddr
		if host, _, err := net.SplitHostPort(realIP); err == nil {
			realIP = host
		}
	}

	// Reject banned IPs before WebSocket upgrade
	if r.rateLimitEnabled && r.IsIPBanned(realIP) {
		http.Error(w, "banned", http.StatusForbidden)
		return
	}

	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := protocol.NewClient(conn, r, realIP)
	ua := req.Header.Get("User-Agent")
	origin := req.Header.Get("Origin")
	log.Printf("New WebSocket connection from %s (UA: %s, Origin: %s)", client.RemoteAddr(), ua, origin)
	if r.rateLimitEnabled {
		client.SetRateLimit(r.checkRate)
	}
	if r.requireAuth {
		client.SetRequireAuth()
		client.SendAuth()
	}

	// Update metrics
	r.metrics.mu.Lock()
	r.metrics.totalConnections++
	r.metrics.lastPacketTime = time.Now()
	r.metrics.packetCount++
	r.metrics.mu.Unlock()

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

// HealthHandler handles health check requests
func (r *Relay) HealthHandler(w http.ResponseWriter, req *http.Request) {
	r.metrics.mu.RLock()
	defer r.metrics.mu.RUnlock()

	// Update metrics
	r.updateMetrics()

	// Get current memory usage
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	memoryUsageMB := float64(m.Alloc) / 1024 / 1024

	// Get active connections count
	r.clientsMu.RLock()
	activeConnections := len(r.clients)
	r.clientsMu.RUnlock()

	// Determine health status
	status := "healthy"
	if r.metrics.dbStatus != "ok" {
		status = "unhealthy"
	}

	// Create response
	response := HealthResponse{
		Status:            status,
		UptimeSeconds:     time.Since(r.metrics.startTime).Seconds(),
		Version:           r.version,
		ActiveConnections: activeConnections,
		TotalConnections:  r.metrics.totalConnections,
		TotalEvents:       r.metrics.totalEvents,
		TotalRequests:     r.metrics.totalRequests,
		PacketsPerSecond:  r.metrics.packetsPerSecond,
		RateLimitedCount:  r.metrics.rateLimitedCount,
		MemoryUsageMB:     memoryUsageMB,
		DatabaseStatus:    r.metrics.dbStatus,
		Timestamp:         time.Now().UTC().Format(time.RFC3339),
	}

	// Set headers and send response
	w.Header().Set("Content-Type", "application/json")
	if status == "healthy" {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(response)
}

// updateMetrics updates dynamic metrics like packets per second
func (r *Relay) updateMetrics() {
	now := time.Now()

	// Calculate packets per second for the last minute
	if now.Sub(r.metrics.lastPacketReset) >= time.Minute {
		r.metrics.packetsPerSecond = float64(r.metrics.packetCount) / now.Sub(r.metrics.lastPacketReset).Seconds()
		r.metrics.packetCount = 0
		r.metrics.lastPacketReset = now
	} else if r.metrics.packetCount > 0 {
		r.metrics.packetsPerSecond = float64(r.metrics.packetCount) / now.Sub(r.metrics.lastPacketReset).Seconds()
	}

	// Check database status
	if r.store != nil {
		// Simple ping - try to query count of events
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := r.store.CountEvents(ctx, []*event.Filter{})
		if err != nil {
			r.metrics.dbStatus = "error: " + err.Error()
		} else {
			r.metrics.dbStatus = "ok"
		}
	} else {
		r.metrics.dbStatus = "not_initialized"
	}
}

const (
	reqRatePerSec       = 10             // max sustained REQ rate per IP per second
	reqBurstLimit       = 20             // max burst of REQs per IP
	banViolationLimit   = 10             // number of rate limit violations before banning
	banDuration         = 24 * time.Hour // how long an IP stays banned
	defaultMaxEventsPerREQ = 100         // max events returned per REQ response
)

// isIPBanned checks if an IP is currently banned. Must be called with ipLimiterMu held.
func (r *Relay) isIPBanned(ip string) bool {
	lim, ok := r.ipLimiters[ip]
	if !ok {
		return false
	}
	if lim.bannedAt.IsZero() {
		return false
	}
	if time.Since(lim.bannedAt) > banDuration {
		// Ban expired — reset
		lim.bannedAt = time.Time{}
		lim.violations = 0
		return false
	}
	return true
}

// IsIPBanned checks if an IP is currently banned (thread-safe, for use in ServeHTTP).
func (r *Relay) IsIPBanned(ip string) bool {
	r.ipLimiterMu.Lock()
	defer r.ipLimiterMu.Unlock()
	return r.isIPBanned(ip)
}

// checkRate implements per-IP rate limiting using a shared token bucket.
// Returns empty string if allowed, or a reason string if rejected.
func (r *Relay) checkRate(clientIP string, pubkey string) string {
	r.ipLimiterMu.Lock()
	defer r.ipLimiterMu.Unlock()

	// Check ban first
	if r.isIPBanned(clientIP) {
		return "banned: too many rate limit violations"
	}

	lim, ok := r.ipLimiters[clientIP]
	if !ok {
		lim = &ipRateLimiter{
			tokens:     reqBurstLimit,
			lastRefill: time.Now(),
			pubkeys:    make(map[string]bool),
		}
		r.ipLimiters[clientIP] = lim
	}
	if pubkey != "" {
		lim.pubkeys[pubkey] = true
	}

	// Refill tokens
	now := time.Now()
	elapsed := now.Sub(lim.lastRefill).Seconds()
	lim.tokens += elapsed * reqRatePerSec
	if lim.tokens > reqBurstLimit {
		lim.tokens = reqBurstLimit
	}
	lim.lastRefill = now

	if lim.tokens < 1 {
		lim.violations++
		r.metrics.mu.Lock()
		r.metrics.rateLimitedCount++
		r.metrics.mu.Unlock()

		if lim.violations >= banViolationLimit {
			lim.bannedAt = now
			pubkeyList := make([]string, 0, len(lim.pubkeys))
			for pk := range lim.pubkeys {
				pubkeyList = append(pubkeyList, pk)
			}
			if len(pubkeyList) > 0 {
				log.Printf("BANNED IP %s for %v after %d violations (pubkeys: %s)", clientIP, banDuration, lim.violations, strings.Join(pubkeyList, ", "))
			} else {
				log.Printf("BANNED IP %s for %v after %d violations (no authenticated pubkeys)", clientIP, banDuration, lim.violations)
			}
			return "banned: too many rate limit violations"
		}

		log.Printf("Rate limited %s (violations %d/%d)", clientIP, lim.violations, banViolationLimit)
		return "rate-limited: too many messages, slow down"
	}

	lim.tokens--
	return ""
}

// HandleEvent processes an EVENT message from a client
func (r *Relay) HandleEvent(ctx context.Context, c *protocol.Client, evt *event.Event) error {
	// Update metrics
	r.metrics.mu.Lock()
	r.metrics.totalEvents++
	r.metrics.packetCount++
	r.metrics.lastPacketTime = time.Now()
	r.metrics.mu.Unlock()

	// NIP-42: Handle AUTH events
	if nip42.IsAuthEvent(evt) {
		if err := nip42.ValidateAuthEvent(evt); err != nil {
			c.SendOK(evt.ID, false, fmt.Sprintf("invalid AUTH: %v", err))
			return fmt.Errorf("invalid AUTH event: %w", err)
		}
		// Verify challenge matches what we sent
		if c.AuthChallenge() != "" && evt.Content != c.AuthChallenge() {
			c.SendOK(evt.ID, false, "invalid: challenge mismatch")
			return fmt.Errorf("AUTH challenge mismatch")
		}
		c.Authenticate(evt.PubKey)
		c.SendOK(evt.ID, true, "authenticated")
		log.Printf("Client %s authenticated as %s", c.RemoteAddr(), evt.PubKey)
		return nil
	}

	// NIP-42: Reject non-AUTH events from unauthenticated clients when auth is required
	if r.requireAuth && !c.IsAuthenticated() {
		c.SendOK(evt.ID, false, "auth-required: this relay requires NIP-42 authentication")
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

	// NIP-28: Validate channel events
	if nip28.IsNIP28Event(evt) {
		if err := nip28.New().Process(evt, r.store); err != nil {
			c.SendOK(evt.ID, false, fmt.Sprintf("invalid channel event: %v", err))
			return fmt.Errorf("invalid channel event: %w", err)
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

	// NIP-28: Save channel events to channel table
	if nip28.IsNIP28Event(evt) {
		var channelStore ChannelStore
		if sqliteStore, ok := r.store.(*sqlite.Store); ok {
			channelStore = sqliteStore
		} else if memStore, ok := r.store.(*memory.Store); ok {
			channelStore = memStore
		}

		if channelStore != nil {
			if err := channelStore.SaveChannelEvent(ctx, evt); err != nil {
				log.Printf("NIP-28: failed to save channel event: %v", err)
			}
		}
	}

	// Send OK message
	c.SendOK(evt.ID, true, "")

	// Broadcast to subscribed clients
	r.broadcastEvent(evt)

	return nil
}

// HandleReq processes a REQ message from a client
func (r *Relay) HandleReq(ctx context.Context, c *protocol.Client, subID string, filters []*event.Filter) error {
	// Update metrics
	r.metrics.mu.Lock()
	r.metrics.totalRequests++
	r.metrics.packetCount++
	r.metrics.lastPacketTime = time.Now()
	r.metrics.mu.Unlock()

	var events []*event.Event
	var err error

	// Check for channel_id in filters (NIP-28)
	channelID, isChannelQuery := getChannelIDFromFilters(filters)

	if isChannelQuery {
		var channelStore ChannelStore
		if sqliteStore, ok := r.store.(*sqlite.Store); ok {
			channelStore = sqliteStore
		} else if memStore, ok := r.store.(*memory.Store); ok {
			channelStore = memStore
		}

		if channelStore == nil {
			return fmt.Errorf("channel events require storage with channel support")
		}
		// NIP-28: Query channel events
		events, err = r.queryChannelEvents(ctx, channelStore, channelID, filters)
		if err != nil {
			return fmt.Errorf("failed to query channel events: %w", err)
		}
	} else if hasSearchField(filters) {
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
	sent := 0
	for _, evt := range events {
		// NIP-40: Filter out expired events
		if nip40.ShouldFilterEvent(evt) {
			continue
		}
		if sent >= r.maxEventsPerREQ {
			break
		}
		if err := c.SendEvent(subID, evt); err != nil {
			log.Printf("Failed to send stored event to client: %v", err)
		}
		sent++
	}

	// Send EOSE to indicate end of stored events
	if err := c.SendEOSE(subID); err != nil {
		log.Printf("Failed to send EOSE to client: %v", err)
	}

	if sent < len(events) {
		log.Printf("Sent %d stored events (capped from %d) for subscription %s", sent, len(events), subID)
	}

	// Auto-close subscription after EOSE to free the slot
	if r.closeAfterEOSE {
		c.RemoveSubscription(subID)
	}

	return nil
}

// HandleClose processes a CLOSE message from a client
func (r *Relay) HandleClose(ctx context.Context, c *protocol.Client, subID string) error {
	// Subscription close is routine — don't log
	c.RemoveSubscription(subID)
	return nil
}

// HandleCount processes a COUNT message from a client (NIP-45)
func (r *Relay) HandleCount(ctx context.Context, c *protocol.Client, countID string, filters []*event.Filter) error {
	log.Printf("Received COUNT request %s from client %s", countID, c.RemoteAddr())

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

// GetMux returns the HTTP multiplexer for the relay
func (r *Relay) GetMux() *http.ServeMux {
	return r.mux
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
	log.Printf("Relay starting on %s", addr)
	log.Printf("Health endpoint available at http://%s/health", addr)
	return http.ListenAndServe(addr, r.mux)
}

// StartTLS starts the relay HTTPS server with TLS certificates
func (r *Relay) StartTLS(addr, certFile, keyFile string) error {
	log.Printf("Relay starting with TLS on %s", addr)
	log.Printf("Certificate file: %s", certFile)
	log.Printf("Private key file: %s", keyFile)
	log.Printf("Health endpoint available at https://%s/health", addr)

	server := &http.Server{
		Addr:    addr,
		Handler: r.mux,
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

// getChannelIDFromFilters extracts channel_id from filters if present
func getChannelIDFromFilters(filters []*event.Filter) (string, bool) {
	for _, filter := range filters {
		if channelIDs, ok := filter.Tags["channel_id"]; ok {
			if len(channelIDs) > 0 && channelIDs[0] != "" {
				return channelIDs[0], true
			}
		}
	}
	return "", false
}

// hasSearchField checks if any filter has a search field
func hasSearchField(filters []*event.Filter) bool {
	for _, filter := range filters {
		if filter.Search != "" {
			return true
		}
	}
	return false
}

// queryChannelEvents handles NIP-28 channel event queries
func (r *Relay) queryChannelEvents(ctx context.Context, store ChannelStore, channelID string, filters []*event.Filter) ([]*event.Event, error) {
	var since, until *int64
	var limit *int

	for _, filter := range filters {
		if filter.Since != nil {
			since = filter.Since
		}
		if filter.Until != nil {
			until = filter.Until
		}
		if filter.Limit != nil {
			limit = filter.Limit
		}
	}

	// If kinds are specified, filter by them
	if len(filters) > 0 && len(filters[0].Kinds) > 0 {
		var allEvents []*event.Event
		for _, kind := range filters[0].Kinds {
			events, err := store.QueryChannelEvents(ctx, channelID, since, until, limit)
			if err != nil {
				return nil, err
			}
			// Filter by kind in memory
			var kindEvents []*event.Event
			for _, evt := range events {
				if evt.Kind == kind {
					kindEvents = append(kindEvents, evt)
				}
			}
			allEvents = append(allEvents, kindEvents...)
		}
		return allEvents, nil
	}

	return store.QueryChannelEvents(ctx, channelID, since, until, limit)
}
