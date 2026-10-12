package integration

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/paulborile/glienicke/internal/store/memory"
	"github.com/paulborile/glienicke/internal/testutil"
	"github.com/paulborile/glienicke/pkg/event"
	"github.com/paulborile/glienicke/pkg/relay"
	"github.com/stretchr/testify/require"
)

// TestReqBurstDoesNotOverflowSendQueue reproduces a production bug: a REQ
// matching more stored events than fit in the client's send queue caused
// HandleReq to enqueue events faster than the writePump could drain them,
// overflowing the queue and disconnecting the client as a "slow client" even
// though it never stalled — it simply hadn't been given a chance to read
// anything yet. This is what production logs showed as a repeating
// "Send queue full ... disconnecting slow client" for freshly connected
// clients issuing their first subscription.
//
// The fix keeps defaultMaxEventsPerREQ safely below the send queue capacity,
// so the entire capped reply always fits in the queue in one burst. This
// test seeds more matching events than that cap — more than the old 64-slot
// send queue could ever hold in one uncapped burst too — then opens a
// subscription without reading anything until the whole reply, including
// EOSE, has had time to enqueue. Before the fix this overflowed the queue
// and disconnected the client before it read a single event; the fix must
// deliver exactly the capped number without ever dropping the connection.
//
// Events are seeded directly into the store (bypassing the WebSocket) so
// this test exercises only the stored-event replay path in HandleReq, not
// the per-IP publish rate limiter.
func TestReqBurstDoesNotOverflowSendQueue(t *testing.T) {
	// A cap explicitly set here, independent of the production default, so
	// this test keeps proving the invariant (cap fits under the send queue)
	// even if the default value changes later. It must stay below
	// protocol.sendQueueSize (64) with headroom, same as the real default.
	const (
		maxEventsPerREQ = 50
		burstSize       = 80 // > maxEventsPerREQ and > the old 64-slot queue
	)

	store := memory.New()
	kp := testutil.MustGenerateKeyPair()
	marker := kp.PubKeyHex

	for i := 0; i < burstSize; i++ {
		evt := &event.Event{
			Kind:      1,
			Content:   fmt.Sprintf("burst event %d", i),
			Tags:      [][]string{{"t", marker}},
			CreatedAt: time.Now().Unix(),
		}
		require.NoError(t, kp.SignEvent(evt))
		require.NoError(t, store.SaveEvent(context.Background(), evt))
	}

	r := relay.New(store)
	r.SetRequireAuth(false)
	r.SetMaxEventsPerREQ(maxEventsPerREQ)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	listener.Close()

	srv := &http.Server{Addr: addr, Handler: r.GetMux()}
	go func() { _ = srv.ListenAndServe() }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
		r.Close()
	}()
	time.Sleep(100 * time.Millisecond)

	wsURL := fmt.Sprintf("ws://%s/", addr)
	subscriber, err := testutil.NewWSClient(wsURL)
	require.NoError(t, err)
	defer subscriber.Close()

	filter := &event.Filter{Kinds: []int{1}, Tags: map[string][]string{"t": {marker}}}
	require.NoError(t, subscriber.SendReq("burst-sub", filter))

	// Do not read anything until after the server has had time to run the
	// entire synchronous send loop in HandleReq — this is the window in which
	// the original bug overflowed the queue before the writePump could drain
	// it. Only after that do we start reading, same as CollectEvents does.
	time.Sleep(200 * time.Millisecond)

	events, err := subscriber.CollectEvents("burst-sub", 5*time.Second)
	require.NoError(t, err, "subscriber should not be disconnected by its own stored-event replay")
	require.Len(t, events, maxEventsPerREQ,
		"reply must deliver exactly the capped number of events, not fewer from a dropped connection")
}
