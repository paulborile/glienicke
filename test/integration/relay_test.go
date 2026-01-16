package integration

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/paul/glienicke/internal/store/memory"
	"github.com/paul/glienicke/internal/testutil"
	"github.com/paul/glienicke/pkg/event"
	"github.com/paul/glienicke/pkg/relay"
)

// setupRelay creates a test relay and returns the WebSocket URL and HTTP URL
func setupRelay(t *testing.T) (string, *relay.Relay, func(), string) {
	t.Helper()

	store := memory.New()
	r := relay.New(store)

	// Find available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to get available port: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	// Start relay
	srv := &http.Server{
		Addr:    addr,
		Handler: r.GetMux(),
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("Relay server error: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	wsURL := fmt.Sprintf("ws://%s/", addr)
	httpURL := fmt.Sprintf("http://%s", addr)
	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
		r.Close()
	}

	return wsURL, r, cleanup, httpURL
}

func TestEventMessage(t *testing.T) {
	url, _, cleanup, _ := setupRelay(t)
	defer cleanup()

	client, err := testutil.NewWSClient(url)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Create a test event
	evt, _ := testutil.MustNewTestEvent(1, "Hello, Nostr!", nil)

	// Send EVENT message
	if err := client.SendEvent(evt); err != nil {
		t.Fatalf("Failed to send event: %v", err)
	}

	// Expect OK response
	accepted, msg, err := client.ExpectOK(evt.ID, 2*time.Second)
	if err != nil {
		t.Fatalf("Failed to receive OK: %v", err)
	}

	if !accepted {
		t.Errorf("Event was not accepted: %s", msg)
	}
}

func TestEventValidation(t *testing.T) {
	url, _, cleanup, _ := setupRelay(t)
	defer cleanup()

	client, err := testutil.NewWSClient(url)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Create an invalid event (missing signature)
	evt := &event.Event{
		ID:        "invalid",
		PubKey:    "0000000000000000000000000000000000000000000000000000000000000000",
		CreatedAt: 1234567890,
		Kind:      1,
		Tags:      [][]string{},
		Content:   "Invalid event",
		Sig:       "",
	}

	// Send EVENT message
	if err := client.SendEvent(evt); err != nil {
		t.Fatalf("Failed to send event: %v", err)
	}

	// Expect OK response with rejection
	accepted, msg, err := client.ExpectOK(evt.ID, 2*time.Second)
	if err != nil {
		t.Fatalf("Failed to receive OK: %v", err)
	}

	if accepted {
		t.Error("Invalid event was accepted")
	}

	if msg == "" {
		t.Error("Expected error message for invalid event")
	}

	t.Logf("Rejection message: %s", msg)
}

func TestReqMessage(t *testing.T) {
	url, _, cleanup, _ := setupRelay(t)
	defer cleanup()

	client, err := testutil.NewWSClient(url)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// First, send some events
	evt1, kp := testutil.MustNewTestEvent(1, "First event", nil)
	evt2, _ := testutil.NewTestEventWithKey(kp, 1, "Second event", nil)

	if err := client.SendEvent(evt1); err != nil {
		t.Fatalf("Failed to send first event: %v", err)
	}
	if _, _, err := client.ExpectOK(evt1.ID, 2*time.Second); err != nil {
		t.Fatalf("Failed to receive OK for first event: %v", err)
	}

	if err := client.SendEvent(evt2); err != nil {
		t.Fatalf("Failed to send second event: %v", err)
	}
	if _, _, err := client.ExpectOK(evt2.ID, 2*time.Second); err != nil {
		t.Fatalf("Failed to receive OK for second event: %v", err)
	}

	// Now subscribe to events from this author
	filter := &event.Filter{
		Authors: []string{kp.PubKeyHex},
	}

	if err := client.SendReq("test-sub", filter); err != nil {
		t.Fatalf("Failed to send REQ: %v", err)
	}

	// Expect the two stored events
	receivedEvents := make(map[string]bool)
	for i := 0; i < 2; i++ {
		evt, err := client.ExpectEvent("test-sub", 2*time.Second)
		if err != nil {
			t.Fatalf("Failed to receive stored event: %v", err)
		}
		receivedEvents[evt.ID] = true
	}

	if !receivedEvents[evt1.ID] || !receivedEvents[evt2.ID] {
		t.Errorf("Did not receive the correct stored events")
	}

	// Expect EOSE
	if err := client.ExpectEOSE("test-sub", 2*time.Second); err != nil {
		t.Fatalf("Failed to receive EOSE: %v", err)
	}

	t.Log("REQ message and event retrieval working correctly")
}

func TestCloseMessage(t *testing.T) {
	url, _, cleanup, _ := setupRelay(t)
	defer cleanup()

	// Client A subscribes
	clientA, err := testutil.NewWSClient(url)
	if err != nil {
		t.Fatalf("Failed to connect clientA: %v", err)
	}
	defer clientA.Close()

	// Client B publishes
	clientB, err := testutil.NewWSClient(url)
	if err != nil {
		t.Fatalf("Failed to connect clientB: %v", err)
	}
	defer clientB.Close()

	// Client A subscribes to kind 1 events
	if err := clientA.SendReq("sub1", &event.Filter{Kinds: []int{1}}); err != nil {
		t.Fatalf("Failed to send REQ: %v", err)
	}

	// Give time for subscription to register
	time.Sleep(50 * time.Millisecond)

	// Client B sends a matching event
	evt1, _ := testutil.MustNewTestEvent(1, "First message", nil)
	if err := clientB.SendEvent(evt1); err != nil {
		t.Fatalf("Failed to send first event: %v", err)
	}

	// Client A should receive it
	if _, err := clientA.ExpectEvent("sub1", 2*time.Second); err != nil {
		t.Fatalf("Client A did not receive the first event: %v", err)
	}

	// Client A closes the subscription
	if err := clientA.SendClose("sub1"); err != nil {
		t.Fatalf("Failed to send CLOSE: %v", err)
	}

	// Give time for close to register
	time.Sleep(50 * time.Millisecond)

	// Client B sends another matching event
	evt2, _ := testutil.MustNewTestEvent(1, "Second message", nil)
	if err := clientB.SendEvent(evt2); err != nil {
		t.Fatalf("Failed to send second event: %v", err)
	}

	// Client A should NOT receive the second event
	_, err = clientA.ExpectEvent("sub1", 200*time.Millisecond)
	if err == nil {
		t.Fatal("Client A received an event after closing subscription")
	}

	t.Log("CLOSE message working correctly")
}

func TestEventBroadcast(t *testing.T) {
	url, _, cleanup, _ := setupRelay(t)
	defer cleanup()

	// Connect two clients
	client1, err := testutil.NewWSClient(url)
	if err != nil {
		t.Fatalf("Failed to connect client1: %v", err)
	}
	defer client1.Close()

	client2, err := testutil.NewWSClient(url)
	if err != nil {
		t.Fatalf("Failed to connect client2: %v", err)
	}
	defer client2.Close()

	// Client1 subscribes to kind 1 events
	filter := &event.Filter{
		Kinds: []int{1},
	}

	if err := client1.SendReq("sub1", filter); err != nil {
		t.Fatalf("Failed to send REQ: %v", err)
	}

	// Give subscription time to register
	time.Sleep(100 * time.Millisecond)

	// Client2 publishes an event
	evt, _ := testutil.MustNewTestEvent(1, "Broadcast test", nil)

	if err := client2.SendEvent(evt); err != nil {
		t.Fatalf("Failed to send event: %v", err)
	}

	// Client2 should receive OK
	accepted, msg, err := client2.ExpectOK(evt.ID, 2*time.Second)
	if err != nil {
		t.Fatalf("Failed to receive OK: %v", err)
	}
	if !accepted {
		t.Fatalf("Event was not accepted: %s", msg)
	}

	// Client1 should receive the event on its subscription
	receivedEvt, err := client1.ExpectEvent("sub1", 2*time.Second)
	if err != nil {
		t.Fatalf("Failed to receive broadcast event: %v", err)
	}

	if receivedEvt.ID != evt.ID {
		t.Errorf("Received wrong event: got %s, want %s", receivedEvt.ID, evt.ID)
	}

	t.Log("Event broadcast working correctly")
}

func TestMultipleFilters(t *testing.T) {
	url, _, cleanup, _ := setupRelay(t)
	defer cleanup()

	client, err := testutil.NewWSClient(url)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Send events of different kinds
	evt1, _ := testutil.MustNewTestEvent(1, "Kind 1 event", nil)
	evt2, _ := testutil.MustNewTestEvent(3, "Kind 3 event", nil)

	if err := client.SendEvent(evt1); err != nil {
		t.Fatalf("Failed to send evt1: %v", err)
	}
	if _, _, err := client.ExpectOK(evt1.ID, 2*time.Second); err != nil {
		t.Fatalf("Failed to receive OK for evt1: %v", err)
	}

	if err := client.SendEvent(evt2); err != nil {
		t.Fatalf("Failed to send evt2: %v", err)
	}
	if _, _, err := client.ExpectOK(evt2.ID, 2*time.Second); err != nil {
		t.Fatalf("Failed to receive OK for evt2: %v", err)
	}

	// Subscribe with multiple filters (OR'd together)
	filter1 := &event.Filter{Kinds: []int{1}}
	filter2 := &event.Filter{Kinds: []int{3}}

	if err := client.SendReq("multi-sub", filter1, filter2); err != nil {
		t.Fatalf("Failed to send REQ: %v", err)
	}

	t.Log("Multiple filters sent successfully")
}

func TestEventWithTags(t *testing.T) {
	url, _, cleanup, _ := setupRelay(t)
	defer cleanup()

	client, err := testutil.NewWSClient(url)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Create event with tags
	tags := [][]string{
		{"e", "referenced-event-id"},
		{"p", "referenced-pubkey"},
		{"t", "nostr"},
	}

	evt, _ := testutil.MustNewTestEvent(1, "Event with tags", tags)

	// Send event
	if err := client.SendEvent(evt); err != nil {
		t.Fatalf("Failed to send event: %v", err)
	}

	accepted, msg, err := client.ExpectOK(evt.ID, 2*time.Second)
	if err != nil {
		t.Fatalf("Failed to receive OK: %v", err)
	}

	if !accepted {
		t.Errorf("Event with tags was not accepted: %s", msg)
	}

	t.Log("Event with tags accepted successfully")
}

// TestFilterMatching moved to pkg/event/filter_unit_test.go as a proper unit test
// The integration test was unreliable due to WebSocket connection sharing and timing issues
// Unit tests provide better isolation and more precise testing of filter matching logic
