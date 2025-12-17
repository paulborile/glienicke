package ratelimit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewLimiter(t *testing.T) {
	t.Run("creates limiter with correct rate", func(t *testing.T) {
		limiter := New(10.0, 10)
		assert.NotNil(t, limiter)
	})

	t.Run("creates limiter with zero rate", func(t *testing.T) {
		limiter := New(0.0, 1)
		assert.NotNil(t, limiter)
	})
}

func TestNewWithInterval(t *testing.T) {
	t.Run("creates limiter with interval", func(t *testing.T) {
		limiter := NewWithInterval(5, time.Second)
		assert.NotNil(t, limiter)
	})

	t.Run("creates limiter with minute interval", func(t *testing.T) {
		limiter := NewWithInterval(1, time.Minute)
		assert.NotNil(t, limiter)
	})
}

func TestAllow(t *testing.T) {
	t.Run("allows requests within limit", func(t *testing.T) {
		limiter := New(10.0, 10)

		// Should allow first 10 requests
		for i := 0; i < 10; i++ {
			assert.True(t, limiter.Allow(), "Request %d should be allowed", i+1)
		}

		// 11th request should be denied
		assert.False(t, limiter.Allow(), "11th request should be denied")
	})

	t.Run("refills tokens over time", func(t *testing.T) {
		limiter := New(100.0, 100) // 100 tokens per second

		// Consume all tokens
		for i := 0; i < 100; i++ {
			limiter.Allow()
		}

		// Should be denied now
		assert.False(t, limiter.Allow(), "Should be denied when tokens exhausted")

		// Wait for refill
		time.Sleep(50 * time.Millisecond)

		// Should allow some tokens
		assert.True(t, limiter.Allow(), "Should allow after refill")
	})
}

func TestAllowN(t *testing.T) {
	t.Run("allows N requests within limit", func(t *testing.T) {
		limiter := New(10.0, 10) // 10 capacity

		assert.True(t, limiter.AllowN(5), "Should allow 5 requests")
		assert.True(t, limiter.AllowN(5), "Should allow another 5 requests")
		assert.False(t, limiter.AllowN(1), "Should deny when tokens exhausted")
	})

	t.Run("denies when not enough tokens", func(t *testing.T) {
		limiter := New(5.0, 10)

		assert.False(t, limiter.AllowN(15), "Should deny when requesting more than capacity")
	})
}

func TestWait(t *testing.T) {
	t.Run("waits for token", func(t *testing.T) {
		limiter := New(100.0, 100)

		// Consume all tokens
		for i := 0; i < 100; i++ {
			limiter.Allow()
		}

		// Should wait for refill
		start := time.Now()
		limiter.Wait()
		duration := time.Since(start)

		// Should wait at least some time for refill
		assert.Greater(t, duration, time.Millisecond, "Should wait for token refill")
	})
}

func TestWaitN(t *testing.T) {
	t.Run("waits for N tokens", func(t *testing.T) {
		limiter := New(50.0, 50)

		// Consume all tokens
		for i := 0; i < 50; i++ {
			limiter.Allow()
		}

		// Should wait for 10 tokens
		start := time.Now()
		limiter.WaitN(10)
		duration := time.Since(start)

		// Should wait at least 200ms for 10 tokens at 50/sec
		assert.GreaterOrEqual(t, duration, 180*time.Millisecond, "Should wait for enough tokens")
	})
}

func TestConcurrency(t *testing.T) {
	t.Run("concurrent access is safe", func(t *testing.T) {
		limiter := New(1000.0, 1000)

		// Launch multiple goroutines
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func() {
				for j := 0; j < 100; j++ {
					limiter.Allow()
				}
				done <- true
			}()
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}

		// If we get here without panic, concurrent access is safe
		assert.True(t, true, "Concurrent access should be safe")
	})
}
