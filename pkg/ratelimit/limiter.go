package ratelimit

import (
	"sync"
	"time"
)

// Limiter implements a simple token bucket rate limiter
type Limiter struct {
	tokens     int64
	capacity   int64
	refillRate int64 // tokens per second
	lastTime   time.Time
	mu         sync.Mutex
}

// New creates a new rate limiter
// tokensPerSecond: how many tokens to add per second
// capacity: maximum number of tokens the bucket can hold
func New(tokensPerSecond float64, capacity int64) *Limiter {
	return &Limiter{
		tokens:     capacity,
		capacity:   capacity,
		refillRate: int64(tokensPerSecond),
		lastTime:   time.Now(),
	}
}

// NewWithInterval creates a rate limiter with refill interval
// tokens: how many tokens to add per interval
// interval: time between token additions
func NewWithInterval(tokens int64, interval time.Duration) *Limiter {
	tokensPerSecond := float64(tokens) / interval.Seconds()
	return New(tokensPerSecond, tokens)
}

// Allow checks if a request is allowed (non-blocking)
func (l *Limiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.refill()

	if l.tokens > 0 {
		l.tokens--
		return true
	}

	return false
}

// AllowN checks if n requests are allowed
func (l *Limiter) AllowN(n int64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.refill()

	if l.tokens >= n {
		l.tokens -= n
		return true
	}

	return false
}

// Wait blocks until a token is available
func (l *Limiter) Wait() {
	for !l.Allow() {
		time.Sleep(time.Millisecond * 10)
	}
}

// WaitN blocks until n tokens are available
func (l *Limiter) WaitN(n int64) {
	for !l.AllowN(n) {
		time.Sleep(time.Millisecond * 10)
	}
}

// refill adds tokens based on elapsed time
func (l *Limiter) refill() {
	now := time.Now()
	elapsed := now.Sub(l.lastTime)

	if elapsed > 0 {
		tokensToAdd := int64(elapsed.Seconds() * float64(l.refillRate))
		if tokensToAdd > 0 {
			l.tokens = min(l.capacity, l.tokens+tokensToAdd)
			l.lastTime = now
		}
	}
}

// min returns the minimum of two int64 values
func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
