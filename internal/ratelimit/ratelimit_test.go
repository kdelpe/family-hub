package ratelimit

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAllow(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := New(3, 15*time.Minute)

		for range 3 {
			ok, _ := l.Allow("a")
			assert.True(t, ok)
		}
		ok, wait := l.Allow("a")
		assert.False(t, ok, "fourth in the window")
		assert.Equal(t, 5*time.Minute, wait, "one event refills every window/n")

		ok, _ = l.Allow("b")
		assert.True(t, ok, "keys are independent")

		time.Sleep(5 * time.Minute)
		ok, _ = l.Allow("a")
		assert.True(t, ok, "refilled")
	})
}

func TestExhaustedCountsOnlySpentEvents(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := New(2, time.Hour)

		blocked, _ := l.Exhausted("a")
		assert.False(t, blocked, "an unknown key")
		for range 5 {
			blocked, _ = l.Exhausted("a")
		}
		assert.False(t, blocked, "checking spends nothing")

		l.Spend("a")
		l.Spend("a")
		blocked, wait := l.Exhausted("a")
		assert.True(t, blocked)
		assert.Equal(t, 30*time.Minute, wait)

		time.Sleep(30 * time.Minute)
		blocked, _ = l.Exhausted("a")
		assert.False(t, blocked)
	})
}

func TestIdleBucketsAreDropped(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := New(1, time.Minute)
		l.Spend("old")

		time.Sleep(time.Minute)
		l.Spend("new")

		assert.NotContains(t, l.buckets, "old")
		assert.Contains(t, l.buckets, "new")
	})
}
