package balancer

import (
	"errors"
	"log"
	"sync/atomic"

	"github.com/Nr-009/Proximo/backends"
)

type RoundRobin struct {
	servers *[]*backends.Server
	counter uint64
}

func NewRoundRobin(servers *[]*backends.Server) *RoundRobin {
	return &RoundRobin{servers: servers}
}

func (rr *RoundRobin) NextServer() (*backends.Server, error) {
	total := uint64(len(*rr.servers))
	if total == 0 {
		return nil, errors.New("no servers available")
	}

	for range *rr.servers {
		idx := atomic.AddUint64(&rr.counter, 1) % total
		s := (*rr.servers)[idx]
		if s.Active {
			return s, nil
		}
	}

	return nil, errors.New("all servers are inactive")
}

func (rr *RoundRobin) OnAddServer(s *backends.Server) {
	log.Printf("[round-robin] notified of server on port %d", s.Port)
}

func (rr *RoundRobin) OnRemoveServer(s *backends.Server) {
	log.Printf("[round-robin] notified of removal on port %d", s.Port)
}