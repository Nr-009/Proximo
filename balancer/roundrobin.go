package balancer

import (
	"errors"
	"log"
	"sync"
	"sync/atomic"

	"github.com/Nr-009/Proximo/backends"
)

type RoundRobin struct {
	servers []*backends.Server
	counter uint64
	mu      sync.RWMutex
}

func NewRoundRobin(servers []*backends.Server) *RoundRobin {
	return &RoundRobin{servers: servers}
}

func (rr *RoundRobin) NextServer() (*backends.Server, error) {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	total := uint64(len(rr.servers))
	if total == 0 {
		return nil, errors.New("no servers available")
	}

	for range rr.servers {
		idx := atomic.AddUint64(&rr.counter, 1) % total
		s := rr.servers[idx]
		if s.Active {
			return s, nil
		}
	}

	return nil, errors.New("all servers are inactive")
}

func (rr *RoundRobin) AddServer(s *backends.Server) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	rr.servers = append(rr.servers, s)
	log.Printf("[round-robin] server added on port %d", s.Port)
}

func (rr *RoundRobin) RemoveServer(s *backends.Server) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	for i, srv := range rr.servers {
		if srv.Port == s.Port {
			rr.servers = append(rr.servers[:i], rr.servers[i+1:]...)
			log.Printf("[round-robin] server removed on port %d", s.Port)
			return
		}
	}
}