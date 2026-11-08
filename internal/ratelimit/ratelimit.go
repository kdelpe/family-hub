// Package ratelimit keeps an in-memory token bucket per key, for a single app instance.
package ratelimit

import (
	"math"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Limiter allows n events per window for each key, refilling evenly across the window.
type Limiter struct {
	mu        sync.Mutex
	every     rate.Limit
	n         int
	window    time.Duration
	buckets   map[string]*bucket
	lastSweep time.Time
}

type bucket struct {
	limiter  *rate.Limiter
	lastUsed time.Time
}

func New(n int, window time.Duration) *Limiter {
	return &Limiter{
		every:   rate.Limit(float64(n) / window.Seconds()),
		n:       n,
		window:  window,
		buckets: make(map[string]*bucket),
	}
}

// Allow spends one event for key, or reports how long until one is available.
func (l *Limiter) Allow(key string) (bool, time.Duration) {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	b := l.bucket(key, now)
	if b.limiter.AllowN(now, 1) {
		return true, 0
	}
	return false, l.wait(b, now)
}

// Exhausted reports whether key has no event left, without spending one.
func (l *Limiter) Exhausted(key string) (bool, time.Duration) {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[key]
	if !ok || b.limiter.TokensAt(now) >= 1 {
		return false, 0
	}
	return true, l.wait(b, now)
}

// Spend uses one event for key even when none is left, so failures keep counting.
func (l *Limiter) Spend(key string) {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	b := l.bucket(key, now)
	b.limiter.ReserveN(now, 1)
}

func (l *Limiter) bucket(key string, now time.Time) *bucket {
	l.sweep(now)
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{limiter: rate.NewLimiter(l.every, l.n)}
		l.buckets[key] = b
	}
	b.lastUsed = now
	return b
}

// A bucket untouched for a whole window is full again, the same as no bucket at all.
func (l *Limiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < l.window {
		return
	}
	for key, b := range l.buckets {
		if now.Sub(b.lastUsed) >= l.window {
			delete(l.buckets, key)
		}
	}
	l.lastSweep = now
}

func (l *Limiter) wait(b *bucket, now time.Time) time.Duration {
	missing := 1 - b.limiter.TokensAt(now)
	return time.Duration(math.Ceil(missing / float64(l.every) * float64(time.Second)))
}
