package balancer

import (
	"errors"
	"log"
	"math/rand"
	"sync/atomic"

	"github.com/Nr-009/Proximo/backends"
)

type PowerOfTwo struct {
	servers *[]*backends.Server
}

func NewPowerOfTwo(servers *[]*backends.Server) *PowerOfTwo {
	return &PowerOfTwo{servers: servers}
}

func (p *PowerOfTwo) NextServer() (*backends.Server, error) {
	active := make([]*backends.Server, 0)
	for _, s := range *p.servers {
		if s.Active {
			active = append(active, s)
		}
	}
	if len(active) == 0 {
		return nil, errors.New("all servers are inactive")
	}
	if len(active) == 1 {
		return active[0], nil
	}

	i := rand.Intn(len(active))
	j := rand.Intn(len(active) - 1)
	if j >= i {
		j++
	}

	a := active[i]
	b := active[j]

	if atomic.LoadInt64(&a.Connections) <= atomic.LoadInt64(&b.Connections) {
		return a, nil
	}
	return b, nil
}

func (p *PowerOfTwo) OnAddServer(s *backends.Server) {
	log.Printf("[p2c] notified of server on port %d", s.Port)
}

func (p *PowerOfTwo) OnRemoveServer(s *backends.Server) {
	log.Printf("[p2c] notified of removal on port %d", s.Port)
}