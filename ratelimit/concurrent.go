package ratelimit

import (
	"sync"
)

type ConcurrentLimiter struct {
	inflight      map[string]int
	mu            sync.Mutex
	maxConcurrent int
}

func NewConcurrentLimiter(maxConcurrent int) *ConcurrentLimiter {
	return &ConcurrentLimiter{
		inflight:      make(map[string]int),
		maxConcurrent: maxConcurrent,
	}
}

func (cl *ConcurrentLimiter) Allow(clientPort string) bool {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	if cl.inflight[clientPort] >= cl.maxConcurrent {
		return false
	}

	cl.inflight[clientPort]++
	return true
}

func (cl *ConcurrentLimiter) Done(clientPort string) {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	if cl.inflight[clientPort] > 0 {
		cl.inflight[clientPort]--
	}
}