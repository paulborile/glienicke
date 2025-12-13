package testutil

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/paul/glienicke/pkg/event"
)

// WSClient is a test WebSocket client for integration tests
type WSClient struct {
	conn   *websocket.Conn
	mu     sync.Mutex
	events map[string][]*event.Event // subID -> events
}

// NewWSClient creates a new test WebSocket client
func NewWSClient(url string) (*WSClient, error) {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}

	return &WSClient{
		conn:   conn,
		events: make(map[string][]*event.Event),
	}, nil
}

// NewWSClientWithDialer creates a new test WebSocket client with a custom dialer
func NewWSClientWithDialer(url string, dialer *websocket.Dialer) (*WSClient, error) {
	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}

	return &WSClient{
		conn:   conn,
		events: make(map[string][]*event.Event),
	}, nil
}

// Close closes the WebSocket connection
func (c *WSClient) Close() error {
	return c.conn.Close()
}

// SendEvent sends an EVENT message
func (c *WSClient) SendEvent(evt *event.Event) error {
	msg := []interface{}{"EVENT", evt}
	return c.conn.WriteJSON(msg)
}

// SendReq sends a REQ message
func (c *WSClient) SendReq(subID string, filters ...*event.Filter) error {
	msg := []interface{}{"REQ", subID}
	for _, f := range filters {
		msg = append(msg, f)
	}
	return c.conn.WriteJSON(msg)
}

// SendClose sends a CLOSE message
func (c *WSClient) SendClose(subID string) error {
	msg := []interface{}{"CLOSE", subID}
	return c.conn.WriteJSON(msg)
}

// SendCountMessage sends a COUNT message
func (c *WSClient) SendCountMessage(countID string, filter *event.Filter) error {
	msg := []interface{}{"COUNT", countID, filter}
	return c.conn.WriteJSON(msg)
}

// ReadMessage reads and parses a single message
func (c *WSClient) ReadMessage() ([]interface{}, error) {
	var msg []json.RawMessage
	if err := c.conn.ReadJSON(&msg); err != nil {
		return nil, err
	}

	if len(msg) == 0 {
		return nil, fmt.Errorf("empty message")
	}

	var msgType string
	if err := json.Unmarshal(msg[0], &msgType); err != nil {
		return nil, err
	}

	result := []interface{}{msgType}
	for i := 1; i < len(msg); i++ {
		var item interface{}
		if err := json.Unmarshal(msg[i], &item); err != nil {
			return nil, err
		}
		result = append(result, item)
	}

	return result, nil
}

// ExpectOK waits for an OK message with the given event ID
func (c *WSClient) ExpectOK(eventID string, timeout time.Duration) (bool, string, error) {
	deadline := time.Now().Add(timeout)
	c.conn.SetReadDeadline(deadline)
	defer c.conn.SetReadDeadline(time.Time{})

	for {
		msg, err := c.ReadMessage()
		if err != nil {
			return false, "", err
		}

		if len(msg) < 3 {
			continue
		}

		msgType, ok := msg[0].(string)
		if !ok || msgType != "OK" {
			continue
		}

		receivedID, ok := msg[1].(string)
		if !ok || receivedID != eventID {
			continue
		}

		accepted, ok := msg[2].(bool)
		if !ok {
			return false, "", fmt.Errorf("invalid OK message format")
		}

		var message string
		if len(msg) > 3 {
			if m, ok := msg[3].(string); ok {
				message = m
			}
		}

		return accepted, message, nil
	}
}

// ExpectEvent waits for an EVENT message for the given subscription
func (c *WSClient) ExpectEvent(subID string, timeout time.Duration) (*event.Event, error) {
	deadline := time.Now().Add(timeout)
	c.conn.SetReadDeadline(deadline)
	defer c.conn.SetReadDeadline(time.Time{})

	for {
		var msg []json.RawMessage
		if err := c.conn.ReadJSON(&msg); err != nil {
			return nil, err
		}

		if len(msg) < 3 {
			continue
		}

		var msgType string
		if err := json.Unmarshal(msg[0], &msgType); err != nil {
			return nil, err
		}

		if msgType != "EVENT" {
			continue
		}

		var receivedSubID string
		if err := json.Unmarshal(msg[1], &receivedSubID); err != nil {
			return nil, err
		}

		if receivedSubID != subID {
			continue
		}

		var evt event.Event
		if err := json.Unmarshal(msg[2], &evt); err != nil {
			return nil, err
		}

		return &evt, nil
	}
}

// ExpectEOSE waits for an EOSE message for the given subscription

// ExpectEOSE waits for an EOSE message for the given subscription
func (c *WSClient) ExpectEOSE(subID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	c.conn.SetReadDeadline(deadline)
	defer c.conn.SetReadDeadline(time.Time{})

	for {
		msg, err := c.ReadMessage()
		if err != nil {
			return err
		}

		if len(msg) < 2 {
			continue
		}

		msgType, ok := msg[0].(string)
		if !ok || msgType != "EOSE" {
			continue
		}

		receivedSubID, ok := msg[1].(string)
		if !ok || receivedSubID != subID {
			continue
		}

		return nil
	}
}

// ExpectNotice waits for a NOTICE message
func (c *WSClient) ExpectNotice(timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	c.conn.SetReadDeadline(deadline)
	defer c.conn.SetReadDeadline(time.Time{})

	for {
		msg, err := c.ReadMessage()
		if err != nil {
			return "", err
		}

		if len(msg) < 2 {
			continue
		}

		msgType, ok := msg[0].(string)
		if !ok || msgType != "NOTICE" {
			continue
		}

		notice, ok := msg[1].(string)
		if !ok {
			return "", fmt.Errorf("invalid NOTICE format")
		}

		return notice, nil
	}
}

// CollectEvents collects all events for a subscription until EOSE
func (c *WSClient) CollectEvents(subID string, timeout time.Duration) ([]*event.Event, error) {
	deadline := time.Now().Add(timeout)
	c.conn.SetReadDeadline(deadline)
	defer c.conn.SetReadDeadline(time.Time{})

	var events []*event.Event

	for {
		var msg []json.RawMessage
		if err := c.conn.ReadJSON(&msg); err != nil {
			return nil, err
		}

		if len(msg) < 2 {
			continue
		}

		var msgType string
		if err := json.Unmarshal(msg[0], &msgType); err != nil {
			return nil, err
		}

		var receivedSubID string
		if err := json.Unmarshal(msg[1], &receivedSubID); err != nil {
			return nil, err
		}

		if receivedSubID != subID {
			continue
		}

		switch msgType {
		case "EVENT":
			if len(msg) < 3 {
				continue
			}
			var evt event.Event
			if err := json.Unmarshal(msg[2], &evt); err != nil {
				return nil, err
			}
			events = append(events, &evt)
		case "EOSE":
			return events, nil
		}
	}
}
