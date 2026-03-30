package protocol

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/paul/glienicke/pkg/event"
)

const (
	// MaxSubscriptionsPerClient is the maximum number of concurrent subscriptions per client
	MaxSubscriptionsPerClient = 20
)

// MessageType represents the type of Nostr protocol message
type MessageType string

const (
	MessageTypeEvent  MessageType = "EVENT"
	MessageTypeReq    MessageType = "REQ"
	MessageTypeClose  MessageType = "CLOSE"
	MessageTypeEOSE   MessageType = "EOSE"   // End of stored events
	MessageTypeOK     MessageType = "OK"     // Command result
	MessageTypeNotice MessageType = "NOTICE" // Human-readable message
	MessageTypeAuth   MessageType = "AUTH"   // NIP-42 authentication
	MessageTypeCount  MessageType = "COUNT"  // NIP-45 event counting
	MessageTypeClosed MessageType = "CLOSED" // NIP-45 count rejection
)

// Handler processes Nostr protocol messages
type Handler interface {
	HandleEvent(ctx context.Context, c *Client, evt *event.Event) error
	HandleReq(ctx context.Context, c *Client, subID string, filters []*event.Filter) error
	HandleClose(ctx context.Context, c *Client, subID string) error
	HandleCount(ctx context.Context, c *Client, countID string, filters []*event.Filter) error
}

// RateLimitFunc is called before processing a message; returns an error message if rejected, empty string if allowed
type RateLimitFunc func(clientIP string) string

// Client represents a WebSocket client connection
type Client struct {
	conn          *websocket.Conn
	handler       Handler
	subscriptions map[string][]*event.Filter // subID -> filters
	subMu         sync.RWMutex
	sendCh        chan []byte
	closeCh       chan struct{}
	closeOnce     sync.Once
	realIP        string        // Real client IP from X-Forwarded-For
	rateLimit     RateLimitFunc // External rate limit check

	// NIP-42 auth
	requireAuth   bool
	authenticated bool
	authChallenge string
	authPubKey    string // pubkey of authenticated client
}

// NewClient creates a new WebSocket client
func NewClient(conn *websocket.Conn, handler Handler, realIP string) *Client {
	return &Client{
		conn:          conn,
		handler:       handler,
		subscriptions: make(map[string][]*event.Filter),
		sendCh:        make(chan []byte, 256),
		closeCh:       make(chan struct{}),
		realIP:        realIP,
	}
}

// SetRateLimit sets an external rate limit function for all incoming messages
func (c *Client) SetRateLimit(fn RateLimitFunc) {
	c.rateLimit = fn
}

// SetRequireAuth enables NIP-42 authentication requirement for this client
func (c *Client) SetRequireAuth() {
	c.requireAuth = true
	// Generate a random challenge
	b := make([]byte, 16)
	rand.Read(b)
	c.authChallenge = hex.EncodeToString(b)
}

// SendAuth sends an AUTH challenge to the client
func (c *Client) SendAuth() error {
	msg := []interface{}{MessageTypeAuth, c.authChallenge}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	select {
	case c.sendCh <- data:
		return nil
	case <-c.closeCh:
		return fmt.Errorf("client closed")
	}
}

// Authenticate marks the client as authenticated with the given pubkey
func (c *Client) Authenticate(pubkey string) {
	c.authenticated = true
	c.authPubKey = pubkey
}

// IsAuthenticated returns whether the client has completed NIP-42 auth
func (c *Client) IsAuthenticated() bool {
	return c.authenticated
}

// AuthChallenge returns the challenge string sent to this client
func (c *Client) AuthChallenge() string {
	return c.authChallenge
}

// AuthPubKey returns the authenticated client's pubkey
func (c *Client) AuthPubKey() string {
	return c.authPubKey
}

// Start begins processing messages from the client
// This method blocks until the connection is closed
func (c *Client) Start(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		c.readPump(ctx)
	}()

	go func() {
		defer wg.Done()
		c.writePump(ctx)
	}()

	wg.Wait()
}

// readPump reads messages from the WebSocket connection
func (c *Client) readPump(ctx context.Context) {
	defer c.Close()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.closeCh:
			return
		default:
		}

		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				// Don't log close 1005 (no status) as an error - it's a normal condition
				if !strings.Contains(err.Error(), "close 1005") {
					log.Printf("WebSocket read error: %v", err)
				}
			}
			return
		}

		if err := c.handleMessage(ctx, message); err != nil {
			log.Printf("Error handling message: %v", err)
			c.SendNotice(fmt.Sprintf("error: %v", err))
		}
	}
}

// writePump sends messages to the WebSocket connection
func (c *Client) writePump(ctx context.Context) {
	defer c.Close()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.closeCh:
			return
		case message := <-c.sendCh:
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("WebSocket write error: %v", err)
				return
			}
		}
	}
}

// handleMessage processes a single protocol message
func (c *Client) handleMessage(ctx context.Context, message []byte) error {
	// Parse as JSON array
	var raw []json.RawMessage
	if err := json.Unmarshal(message, &raw); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	if len(raw) == 0 {
		return fmt.Errorf("empty message")
	}

	// Parse message type
	var msgType string
	if err := json.Unmarshal(raw[0], &msgType); err != nil {
		return fmt.Errorf("invalid message type: %w", err)
	}

	// NIP-42: Require authentication for REQ and COUNT (EVENT allowed through for AUTH handshake)
	if c.requireAuth && !c.authenticated {
		if MessageType(msgType) == MessageTypeReq || MessageType(msgType) == MessageTypeCount {
			if len(raw) >= 2 {
				var subID string
				if json.Unmarshal(raw[1], &subID) == nil {
					c.SendClosed(subID, "auth-required: this relay requires NIP-42 authentication")
					return nil
				}
			}
			c.SendNotice("auth-required: this relay requires NIP-42 authentication")
			return nil
		}
	}

	// Rate limit all messages except CLOSE (always allow clients to clean up subscriptions)
	if MessageType(msgType) != MessageTypeClose && c.rateLimit != nil {
		if reason := c.rateLimit(c.realIP); reason != "" {
			// For REQ/COUNT, send CLOSED with the subscription/count ID per Nostr protocol
			if (MessageType(msgType) == MessageTypeReq || MessageType(msgType) == MessageTypeCount) && len(raw) >= 2 {
				var subID string
				if json.Unmarshal(raw[1], &subID) == nil {
					c.SendClosed(subID, reason)
					return nil
				}
			}
			c.SendNotice(reason)
			return nil
		}
	}

	switch MessageType(msgType) {
	case MessageTypeEvent:
		return c.handleEventMessage(ctx, raw)
	case MessageTypeReq:
		return c.handleReqMessage(ctx, raw)
	case MessageTypeClose:
		return c.handleCloseMessage(ctx, raw)
	case MessageTypeCount:
		return c.handleCountMessage(ctx, raw)
	default:
		return fmt.Errorf("unknown message type: %s", msgType)
	}
}

// handleEventMessage processes an EVENT message
func (c *Client) handleEventMessage(ctx context.Context, raw []json.RawMessage) error {
	if len(raw) != 2 {
		return fmt.Errorf("EVENT message must have 2 elements")
	}

	var evt event.Event
	if err := json.Unmarshal(raw[1], &evt); err != nil {
		return fmt.Errorf("invalid event: %w", err)
	}

	// Validate event
	if err := evt.Validate(); err != nil {
		c.SendOK(evt.ID, false, fmt.Sprintf("invalid: %v", err))
		return nil
	}

	// Handle event
	if err := c.handler.HandleEvent(ctx, c, &evt); err != nil {
		c.SendOK(evt.ID, false, fmt.Sprintf("error: %v", err))
		return nil
	}

	c.SendOK(evt.ID, true, "")
	return nil
}

// handleReqMessage processes a REQ message
func (c *Client) handleReqMessage(ctx context.Context, raw []json.RawMessage) error {
	if len(raw) < 2 {
		return fmt.Errorf("REQ message must have at least 2 elements")
	}

	var subID string
	if err := json.Unmarshal(raw[1], &subID); err != nil {
		return fmt.Errorf("invalid subscription ID: %w", err)
	}

	// Max concurrent subscriptions check (replacing existing sub doesn't count as new)
	c.subMu.RLock()
	_, isReplacement := c.subscriptions[subID]
	subCount := len(c.subscriptions)
	c.subMu.RUnlock()

	if !isReplacement && subCount >= MaxSubscriptionsPerClient {
		log.Printf("Max subscriptions reached for client %s (subscription %s, count %d)", c.RemoteAddr(), subID, subCount)
		c.SendClosed(subID, "rate-limited: too many concurrent subscriptions")
		return nil
	}

	// Parse filters
	var filters []*event.Filter
	for i := 2; i < len(raw); i++ {
		var filter event.Filter
		if err := json.Unmarshal(raw[i], &filter); err != nil {
			return fmt.Errorf("invalid filter: %w", err)
		}
		filters = append(filters, &filter)
	}

	// Store subscription
	c.subMu.Lock()
	c.subscriptions[subID] = filters
	c.subMu.Unlock()

	// Handle subscription
	return c.handler.HandleReq(ctx, c, subID, filters)
}

// handleCloseMessage processes a CLOSE message
func (c *Client) handleCloseMessage(ctx context.Context, raw []json.RawMessage) error {
	if len(raw) != 2 {
		return fmt.Errorf("CLOSE message must have 2 elements")
	}

	var subID string
	if err := json.Unmarshal(raw[1], &subID); err != nil {
		return fmt.Errorf("invalid subscription ID: %w", err)
	}

	// Handle close
	return c.handler.HandleClose(ctx, c, subID)
}

// handleCountMessage processes a COUNT message (NIP-45)
func (c *Client) handleCountMessage(ctx context.Context, raw []json.RawMessage) error {
	if len(raw) < 3 {
		return fmt.Errorf("COUNT message must have at least 3 elements")
	}

	var countID string
	if err := json.Unmarshal(raw[1], &countID); err != nil {
		return fmt.Errorf("invalid count ID: %w", err)
	}

	// Parse filters
	var filters []*event.Filter
	for i := 2; i < len(raw); i++ {
		var filter event.Filter
		if err := json.Unmarshal(raw[i], &filter); err != nil {
			return fmt.Errorf("invalid filter: %w", err)
		}
		filters = append(filters, &filter)
	}

	// Handle count
	return c.handler.HandleCount(ctx, c, countID, filters)
}

// RemoveSubscription removes a subscription from the client
func (c *Client) RemoveSubscription(subID string) {
	c.subMu.Lock()
	defer c.subMu.Unlock()
	delete(c.subscriptions, subID)
}

// SendEvent sends an event to the client for a subscription
func (c *Client) SendEvent(subID string, evt *event.Event) error {
	msg := []interface{}{MessageTypeEvent, subID, evt}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	select {
	case c.sendCh <- data:
		return nil
	case <-c.closeCh:
		return fmt.Errorf("client closed")
	}
}

// SendEOSE sends an end-of-stored-events message
func (c *Client) SendEOSE(subID string) error {
	msg := []interface{}{MessageTypeEOSE, subID}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	select {
	case c.sendCh <- data:
		return nil
	case <-c.closeCh:
		return fmt.Errorf("client closed")
	}
}

// SendOK sends an OK message in response to an EVENT
func (c *Client) SendOK(eventID string, accepted bool, message string) error {
	msg := []interface{}{MessageTypeOK, eventID, accepted, message}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	select {
	case c.sendCh <- data:
		return nil
	case <-c.closeCh:
		return fmt.Errorf("client closed")
	}
}

// SendNotice sends a human-readable notice message
func (c *Client) SendNotice(message string) error {
	msg := []interface{}{MessageTypeNotice, message}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	select {
	case c.sendCh <- data:
		return nil
	case <-c.closeCh:
		return fmt.Errorf("client closed")
	}
}

// Close closes the client connection
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		close(c.closeCh)
		c.conn.Close()
	})
}

// GetSubscriptions returns the current subscriptions
func (c *Client) GetSubscriptions() map[string][]*event.Filter {
	c.subMu.RLock()
	defer c.subMu.RUnlock()

	// Create copy
	subs := make(map[string][]*event.Filter)
	for k, v := range c.subscriptions {
		subs[k] = v
	}
	return subs
}

// HasSubscriptionToPubKey checks if the client has any active subscription that includes the given public key in a 'p' tag.
func (c *Client) HasSubscriptionToPubKey(pubKey string) bool {
	c.subMu.RLock()
	defer c.subMu.RUnlock()

	for _, filters := range c.subscriptions {
		for _, filter := range filters {
			if pTags, ok := filter.Tags["p"]; ok {
				for _, p := range pTags {
					if p == pubKey {
						return true
					}
				}
			}
		}
	}
	return false
}

// RemoteAddr returns the real client IP if available, otherwise the connection's remote address
func (c *Client) RemoteAddr() string {
	if c.realIP != "" {
		return c.realIP
	}
	return c.conn.RemoteAddr().String()
}

// SendCount sends a COUNT response to the client
func (c *Client) SendCount(countID string, count int, approximate bool) error {
	response := map[string]interface{}{
		"count": count,
	}
	if approximate {
		response["approximate"] = true
	}

	msg := []interface{}{MessageTypeCount, countID, response}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	select {
	case c.sendCh <- data:
		return nil
	case <-c.closeCh:
		return fmt.Errorf("client closed")
	}
}

// SendClosed sends a CLOSED message to the client (NIP-45)
func (c *Client) SendClosed(countID string, reason string) error {
	msg := []interface{}{MessageTypeClosed, countID, reason}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	select {
	case c.sendCh <- data:
		return nil
	case <-c.closeCh:
		return fmt.Errorf("client closed")
	}
}
