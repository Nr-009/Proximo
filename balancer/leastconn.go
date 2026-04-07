package balancer

import (
	"errors"
	"log"
	"sync/atomic"

	"github.com/Nr-009/Proximo/backends"
)

type LeastConn struct {
	servers *[]*backends.Server
}

func NewLeastConn(servers *[]*backends.Server) *LeastConn {
	return &LeastConn{servers: servers}
}

func (lc *LeastConn) NextServer() (*backends.Server, error) {
	var best *backends.Server

	for _, s := range *lc.servers {
		if !s.Active {
			continue
		}
		if best == nil || atomic.LoadInt64(&s.Connections) < atomic.LoadInt64(&best.Connections) {
			best = s
		}
	}

	if best == nil {
		return nil, errors.New("all servers are inactive")
	}

	atomic.AddInt64(&best.Connections, 1)
	return best, nil
}

func (lc *LeastConn) OnAddServer(s *backends.Server) {
	log.Printf("[least-conn] notified of server on port %d", s.Port)
}

func (lc *LeastConn) OnRemoveServer(s *backends.Server) {
	log.Printf("[least-conn] notified of removal on port %d", s.Port)
}