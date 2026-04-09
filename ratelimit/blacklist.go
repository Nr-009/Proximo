package ratelimit

import (
	"log"
	"sync"
)

type Blacklist struct {
	blocked    map[string]bool
	violations map[string]int
	threshold  int
	mu         sync.RWMutex
}

func NewBlacklist(threshold int) *Blacklist {
	return &Blacklist{
		blocked:    make(map[string]bool),
		violations: make(map[string]int),
		threshold:  threshold,
	}
}

func (bl *Blacklist) IsBlocked(clientPort string) bool {
	bl.mu.RLock()
	defer bl.mu.RUnlock()
	return bl.blocked[clientPort]
}

func (bl *Blacklist) RecordViolation(clientPort string) {
	bl.mu.Lock()
	defer bl.mu.Unlock()

	bl.violations[clientPort]++
	if bl.violations[clientPort] >= bl.threshold {
		bl.blocked[clientPort] = true
		log.Printf("[blacklist] client %s blacklisted after %d violations", clientPort, bl.violations[clientPort])
	}
}

func (bl *Blacklist) Block(clientPort string) {
	bl.mu.Lock()
	defer bl.mu.Unlock()
	bl.blocked[clientPort] = true
	log.Printf("[blacklist] client %s manually blacklisted", clientPort)
}

func (bl *Blacklist) Status() map[string]interface{} {
	bl.mu.RLock()
	defer bl.mu.RUnlock()

	blocked := make(map[string]bool)
	for k, v := range bl.blocked {
		blocked[k] = v
	}
	violations := make(map[string]int)
	for k, v := range bl.violations {
		violations[k] = v
	}

	return map[string]interface{}{
		"blocked":    blocked,
		"violations": violations,
		"threshold":  bl.threshold,
	}
}