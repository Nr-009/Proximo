package ratelimit

import (
	"sync"
	"time"
)

type LeakyBucket struct {
	queues  map[string]int
	mu      sync.Mutex
	bucketSize        int
	requestsPerSecond int
}

func NewLeakyBucket(requestsPerSecond int, bucketSize int) *LeakyBucket {
	lb := &LeakyBucket{
		queues:            make(map[string]int),
		bucketSize:        bucketSize,
		requestsPerSecond: requestsPerSecond,
	}
	go lb.drain()
	return lb
}

func (lb *LeakyBucket) drain() {
	interval := time.Duration(1000/lb.requestsPerSecond) * time.Millisecond
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		lb.mu.Lock()
		for client, depth := range lb.queues {
			if depth > 0 {
				lb.queues[client] = depth - 1
			}
		}
		lb.mu.Unlock()
	}
}

func (lb *LeakyBucket) Allow(clientPort string) bool {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	depth, exists := lb.queues[clientPort]
	if !exists {
		lb.queues[clientPort] = 1
		return true
	}

	if depth >= lb.bucketSize {
		return false
	}

	lb.queues[clientPort] = depth + 1
	return true
}

func (lb *LeakyBucket) Done(clientPort string) {}