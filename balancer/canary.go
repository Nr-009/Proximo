package balancer

import (
	"errors"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"

	"github.com/Nr-009/Proximo/backends"
)

type Canary struct {
	servers        *[]*backends.Server
	canaryPort     int
	canaryPercent  int
	errorThreshold float64
	counter        uint64
	mu             sync.Mutex
}

func NewCanary(servers *[]*backends.Server, canaryPort, canaryPercent int, errorThreshold float64) *Canary {
	if errorThreshold == 0 {
		errorThreshold = 0.10
	}

	return &Canary{
		servers:        servers,
		canaryPort:     canaryPort,
		canaryPercent:  canaryPercent,
		errorThreshold: errorThreshold,
	}
}

func (c *Canary) getCanary() *backends.Server {
	for _, s := range *c.servers {
		if s.Port == c.canaryPort {
			return s
		}
	}
	return nil
}

func (c *Canary) canaryHealthy(s *backends.Server) bool {
	if !s.Active {
		return false
	}

	requests := atomic.LoadInt64(&s.Requests)
	if requests == 0 {
		return true
	}

	errs := atomic.LoadInt64(&s.Errors)
	errorRate := float64(errs) / float64(requests)

	if errorRate > c.errorThreshold {
		log.Printf("[canary] server on port %d error rate %.2f%% exceeds threshold %.2f%% — routing to stable",
			s.Port, errorRate*100, c.errorThreshold*100)
		return false
	}

	return true
}

func (c *Canary) nextStable() (*backends.Server, error) {
	total := uint64(0)
	for _, s := range *c.servers {
		if s.Port != c.canaryPort && s.Active {
			total++
		}
	}

	if total == 0 {
		return nil, errors.New("no stable servers available")
	}

	idx := atomic.AddUint64(&c.counter, 1) % total

	current := uint64(0)
	for _, s := range *c.servers {
		if s.Port != c.canaryPort && s.Active {
			if current == idx {
				return s, nil
			}
			current++
		}
	}

	return nil, errors.New("no stable servers available")
}

func (c *Canary) NextServer() (*backends.Server, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	canary := c.getCanary()
	if canary != nil && c.canaryPercent > 0 {
		if rand.Intn(100) < c.canaryPercent && c.canaryHealthy(canary) {
			atomic.AddInt64(&canary.Connections, 1)
			log.Printf("[canary] routing to canary on port %d", canary.Port)
			return canary, nil
		}
	}

	stable, err := c.nextStable()
	if err != nil {
		return nil, err
	}

	atomic.AddInt64(&stable.Connections, 1)
	return stable, nil
}

func (c *Canary) NextStable() (*backends.Server, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	stable, err := c.nextStable()
	if err != nil {
		return nil, err
	}

	atomic.AddInt64(&stable.Connections, 1)
	return stable, nil
}

func (c *Canary) OnAddServer(s *backends.Server) {
	log.Printf("[canary] notified of server on port %d", s.Port)
}

func (c *Canary) OnRemoveServer(s *backends.Server) {
	log.Printf("[canary] notified of removal on port %d", s.Port)
}