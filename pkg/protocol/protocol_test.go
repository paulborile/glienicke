package protocol

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// newTestClient spins up a real WebSocket connection (server side) and returns a
// Client wrapping it. The server handler simply upgrades and parks the
// connection so the *websocket.Conn is valid for Close()/RemoteAddr().
func newTestClient(t *testing.T) (*Client, func()) {
	t.Helper()

	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	connCh := make(chan *websocket.Conn, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("server upgrade failed: %v", err)
			return
		}
		connCh <- conn
		// Park: hold the connection open until the test tears down.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	dialConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		srv.Close()
		t.Fatalf("dial failed: %v", err)
	}

	serverConn := <-connCh
	c := NewClient(serverConn, nil, "127.0.0.1")

	cleanup := func() {
		dialConn.Close()
		srv.Close()
	}
	return c, cleanup
}

// TestEnqueueDropsSlowClient verifies that once a client's send queue is full,
// the next enqueue disconnects the client and returns an error instead of
// blocking — bounding per-client memory under backpressure.
func TestEnqueueDropsSlowClient(t *testing.T) {
	c, cleanup := newTestClient(t)
	defer cleanup()

	// Fill the queue to capacity. No writePump is running, so nothing drains it;
	// every one of these must be accepted into the buffer without blocking.
	for i := 0; i < sendQueueSize; i++ {
		if err := c.enqueue([]byte("x")); err != nil {
			t.Fatalf("enqueue %d/%d should have succeeded, got: %v", i+1, sendQueueSize, err)
		}
	}

	// The queue is now full. The next enqueue must NOT block: it should drop the
	// client and return the queue-full error.
	done := make(chan error, 1)
	go func() { done <- c.enqueue([]byte("overflow")) }()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("enqueue on a full queue should have returned an error, got nil")
		}
		if !strings.Contains(err.Error(), "queue full") {
			t.Fatalf("expected a 'queue full' error, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("enqueue blocked on a full queue; it must drop the slow client instead")
	}

	// The client must have been closed by the overflow.
	select {
	case <-c.closeCh:
		// closed as expected
	default:
		t.Fatal("client should have been closed after send-queue overflow")
	}
}

// TestEnqueueReturnsClosedAfterClose verifies enqueue reports a closed client
// rather than dropping silently or panicking.
func TestEnqueueReturnsClosedAfterClose(t *testing.T) {
	c, cleanup := newTestClient(t)
	defer cleanup()

	c.Close()

	if err := c.enqueue([]byte("x")); err == nil {
		t.Fatal("enqueue on a closed client should return an error")
	}
}

// TestTruncateForLog verifies short payloads are returned as-is (so a normal
// malformed message is fully visible in logs), and long ones are cut with a
// visible marker rather than silently dropped or left unbounded.
func TestTruncateForLog(t *testing.T) {
	t.Run("under limit is unchanged", func(t *testing.T) {
		got := truncateForLog([]byte("short"), 500)
		if got != "short" {
			t.Fatalf("got %q, want %q", got, "short")
		}
	})

	t.Run("exactly at limit is unchanged", func(t *testing.T) {
		data := strings.Repeat("x", 500)
		got := truncateForLog([]byte(data), 500)
		if got != data {
			t.Fatalf("payload at exactly the limit must not be truncated")
		}
	})

	t.Run("over limit is cut with marker", func(t *testing.T) {
		data := strings.Repeat("x", 600)
		got := truncateForLog([]byte(data), 500)
		if len(got) <= 500 {
			t.Fatalf("truncated output should include the marker suffix, got len %d", len(got))
		}
		if !strings.HasPrefix(got, strings.Repeat("x", 500)) {
			t.Fatalf("truncated output should start with the first 500 bytes")
		}
		if !strings.Contains(got, "truncated, 600 bytes total") {
			t.Fatalf("truncated output should report the original size, got: %s", got)
		}
	})
}
