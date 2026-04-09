package ratelimit

import (
	"sync"
	"time"
)

type bucket struct {
	tokens     float64
	lastRefill time.Time
}

type TokenBucket struct {
	buckets           map[string]*bucket
	mu                sync.Mutex
	requestsPerSecond float64
	bucketSize        float64
}

func NewTokenBucket(requestsPerSecond int, bucketSize int) *TokenBucket {
	return &TokenBucket{
		buckets:           make(map[string]*bucket),
		requestsPerSecond: float64(requestsPerSecond),
		bucketSize:        float64(bucketSize),
	}
}

func (tb *TokenBucket) Allow(clientPort string) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	b, exists := tb.buckets[clientPort]
	if !exists {
		tb.buckets[clientPort] = &bucket{
			tokens:     tb.bucketSize - 1,
			lastRefill: time.Now(),
		}
		return true
	}

	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	earned := elapsed * tb.requestsPerSecond

	b.tokens = min(tb.bucketSize, b.tokens+earned)
	b.lastRefill = now

	if b.tokens >= 1 {
		b.tokens--
		return true
	}

	return false
}

func (tb *TokenBucket) Done(clientPort string) {}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}