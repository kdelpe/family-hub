// Package ratelimit keeps an in-memory token bucket per key, for a single app instance.
package ratelimit

import (
	"math"
	"sync"
	"time"
)

// Limiter allows n events per window for each key, refilling evenly across the window.
// Hand-rolled because x/time/rate can't give back a token that was granted at once.
type Limiter struct {
	mu        sync.Mutex
	n         float64
	perToken  time.Duration
	window    time.Duration
	buckets   map[string]*bucket
	lastSweep time.Time
}

type bucket struct {
	tokens float64
	at     time.Time
}

func New(n int, window time.Duration) *Limiter {
	return &Limiter{
		n:        float64(n),
		perToken: window / time.Duration(n),
		window:   window,
		buckets:  make(map[string]*bucket),
	}
}

// Take spends one event for key, or reports how long until one is available.
func (l *Limiter) Take(key string) (bool, time.Duration) {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweep(now)
	b := l.refilled(key, now)
	if b.tokens < 1 {
		return false, time.Duration(math.Ceil((1 - b.tokens) * float64(l.perToken)))
	}
	b.tokens--
	return true, 0
}

// Refund gives back an event that turned out not to count.
func (l *Limiter) Refund(key string) {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	b := l.refilled(key, now)
	b.tokens = min(l.n, b.tokens+1)
}

func (l *Limiter) refilled(key string, now time.Time) *bucket {
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.n, at: now}
		l.buckets[key] = b
	}
	b.tokens = min(l.n, b.tokens+float64(now.Sub(b.at))/float64(l.perToken))
	b.at = now
	return b
}

// A full bucket is the same as no bucket, so dropping one never forgives anything.
func (l *Limiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < l.window {
		return
	}
	for key, b := range l.buckets {
		if b.tokens+float64(now.Sub(b.at))/float64(l.perToken) >= l.n {
			delete(l.buckets, key)
		}
	}
	l.lastSweep = now
}
