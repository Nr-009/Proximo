package ratelimit

import (
	"sync"
	"time"
)

type SlidingWindow struct {
	windows           map[string][]time.Time
	mu                sync.Mutex
	requestsPerSecond int
	windowSeconds     int
}

func NewSlidingWindow(requestsPerSecond int, windowSeconds int) *SlidingWindow {
	return &SlidingWindow{
		windows:           make(map[string][]time.Time),
		requestsPerSecond: requestsPerSecond,
		windowSeconds:     windowSeconds,
	}
}

func (sw *SlidingWindow) Allow(clientPort string) bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-time.Duration(sw.windowSeconds) * time.Second)
	limit := sw.requestsPerSecond * sw.windowSeconds

	timestamps, exists := sw.windows[clientPort]
	if !exists {
		sw.windows[clientPort] = []time.Time{now}
		return true
	}

	valid := timestamps[:0]
	for _, t := range timestamps {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= limit {
		sw.windows[clientPort] = valid
		return false
	}

	sw.windows[clientPort] = append(valid, now)
	return true
}

func (sw *SlidingWindow) Done(clientPort string) {}