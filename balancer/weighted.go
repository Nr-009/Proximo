package balancer

import (
	"errors"
	"log"
	"sync"

	"github.com/Nr-009/Proximo/backends"
)

type Weighted struct {
	servers       []*backends.Server
	currentWeight []int
	mu            sync.Mutex
}

func NewWeighted(servers []*backends.Server) *Weighted {
	return &Weighted{
		servers:       servers,
		currentWeight: make([]int, len(servers)),
	}
}

func (w *Weighted) NextServer() (*backends.Server, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	total := 0
	best := -1

	for i, s := range w.servers {
		if !s.Active {
			continue
		}

		// step 1 — add weight to current
		w.currentWeight[i] += s.Weight
		total += s.Weight

		// step 2 — pick highest current weight
		if best == -1 || w.currentWeight[i] > w.currentWeight[best] {
			best = i
		}
	}

	if best == -1 {
		return nil, errors.New("all servers are inactive")
	}

	// step 3 — penalize winner by subtracting total
	w.currentWeight[best] -= total

	return w.servers[best], nil
}

func (w *Weighted) AddServer(s *backends.Server) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.servers = append(w.servers, s)
	w.currentWeight = append(w.currentWeight, 0)
	log.Printf("[weighted] server added on port %d", s.Port)
}

func (w *Weighted) RemoveServer(s *backends.Server) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for i, srv := range w.servers {
		if srv.Port == s.Port {
			w.servers = append(w.servers[:i], w.servers[i+1:]...)
			w.currentWeight = append(w.currentWeight[:i], w.currentWeight[i+1:]...)
			log.Printf("[weighted] server removed on port %d", s.Port)
			return
		}
	}
}