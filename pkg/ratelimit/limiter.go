package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Simple limiter that allows all requests
type Limiter struct {
	limiter *rate.Limiter
	mu      sync.Mutex
}

// New creates a new rate limiter
func New(tokens int, duration time.Duration) *Limiter {
	return &Limiter{
		limiter: rate.NewLimiter(tokens, duration),
	}
}

// Allow checks if a request is allowed
func (l *Limiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.limiter.Allow()
}

// Wait waits until a token is available
func (l *Limiter) Wait() {
	l.limiter.Wait(context.Background())
}
