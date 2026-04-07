package balancer

import (
	"errors"
	"log"
	"sync"

	"github.com/Nr-009/Proximo/backends"
)

type Weighted struct {
	servers       *[]*backends.Server
	currentWeight map[int]int
	mu            sync.Mutex
}

func NewWeighted(servers *[]*backends.Server) *Weighted {
	cw := make(map[int]int)
	for _, s := range *servers {
		cw[s.Port] = 0
	}
	return &Weighted{
		servers:       servers,
		currentWeight: cw,
	}
}

func (w *Weighted) NextServer() (*backends.Server, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	total := 0
	var best *backends.Server

	for _, s := range *w.servers {
		if !s.Active {
			continue
		}

		w.currentWeight[s.Port] += s.Weight
		total += s.Weight

		if best == nil || w.currentWeight[s.Port] > w.currentWeight[best.Port] {
			best = s
		}
	}

	if best == nil {
		return nil, errors.New("all servers are inactive")
	}

	w.currentWeight[best.Port] -= total
	return best, nil
}

func (w *Weighted) OnAddServer(s *backends.Server) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.currentWeight[s.Port] = 0
	log.Printf("[weighted] internal state updated for new server on port %d", s.Port)
}

func (w *Weighted) OnRemoveServer(s *backends.Server) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.currentWeight, s.Port)
	log.Printf("[weighted] internal state cleaned up for server on port %d", s.Port)
}