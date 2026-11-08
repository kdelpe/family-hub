package ratelimit

import (
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTake(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := New(3, 15*time.Minute)

		for range 3 {
			ok, _ := l.Take("a")
			assert.True(t, ok)
		}
		ok, wait := l.Take("a")
		assert.False(t, ok, "fourth in the window")
		assert.Equal(t, 5*time.Minute, wait, "one event refills every window/n")

		ok, _ = l.Take("b")
		assert.True(t, ok, "keys are independent")

		time.Sleep(5 * time.Minute)
		ok, _ = l.Take("a")
		assert.True(t, ok, "refilled")
	})
}

func TestRefund(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := New(1, time.Hour)

		ok, _ := l.Take("a")
		assert.True(t, ok)
		l.Refund("a")
		ok, _ = l.Take("a")
		assert.True(t, ok, "the refunded event is back")

		l.Refund("a")
		l.Refund("a")
		ok, _ = l.Take("a")
		assert.True(t, ok)
		ok, _ = l.Take("a")
		assert.False(t, ok, "refunds never exceed n")
	})
}

func TestConcurrentTakesNeverExceedN(t *testing.T) {
	l := New(5, time.Hour)
	var mu sync.Mutex
	granted := 0
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			if ok, _ := l.Take("a"); ok {
				mu.Lock()
				granted++
				mu.Unlock()
			}
		})
	}
	wg.Wait()

	assert.Equal(t, 5, granted)
}

func TestSweepDropsOnlyFullBuckets(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := New(2, time.Minute)
		l.Take("used once")

		time.Sleep(30 * time.Second)
		l.Take("drained")
		l.Take("drained")

		time.Sleep(30 * time.Second)
		l.Take("trigger")

		assert.NotContains(t, l.buckets, "used once", "refilled, so dropped")
		assert.Contains(t, l.buckets, "drained", "still owes time")
	})
}
