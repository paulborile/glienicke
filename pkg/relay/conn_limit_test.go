package relay

import (
	"sync"
	"sync/atomic"
	"testing"
)

// TestAcquireGlobalConnSlot verifies the global connection cap admits exactly
// maxConnections slots and rejects beyond that, and that releasing frees slots.
func TestAcquireGlobalConnSlot(t *testing.T) {
	r := &Relay{}

	// Fill to capacity.
	for i := 0; i < maxConnections; i++ {
		if !r.acquireGlobalConnSlot() {
			t.Fatalf("acquire %d/%d should have succeeded", i+1, maxConnections)
		}
	}

	// One past capacity must be rejected, and must not leak a reservation.
	if r.acquireGlobalConnSlot() {
		t.Fatal("acquire past capacity should have failed")
	}
	if got := atomic.LoadInt64(&r.activeConns); got != maxConnections {
		t.Fatalf("rejected acquire must not change the count: got %d, want %d", got, maxConnections)
	}

	// Releasing one slot lets exactly one more in.
	r.releaseGlobalConnSlot()
	if !r.acquireGlobalConnSlot() {
		t.Fatal("acquire after a release should have succeeded")
	}
}

// TestAcquireGlobalConnSlotConcurrent proves the check-and-reserve is race-free:
// under many concurrent acquires, the number granted never exceeds the cap.
// Run with -race to also catch data races on the counter.
func TestAcquireGlobalConnSlotConcurrent(t *testing.T) {
	r := &Relay{}

	const goroutines = 4000 // well above maxConnections
	var granted int64
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			if r.acquireGlobalConnSlot() {
				atomic.AddInt64(&granted, 1)
			}
		}()
	}
	wg.Wait()

	if granted != maxConnections {
		t.Fatalf("granted %d slots, want exactly %d (cap must not be exceeded or undershot)", granted, maxConnections)
	}
	if got := atomic.LoadInt64(&r.activeConns); got != maxConnections {
		t.Fatalf("activeConns = %d, want %d", got, maxConnections)
	}
}
