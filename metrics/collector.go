package metrics

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/Nr-009/Proximo/backends"
)

type ServerSnapshot struct {
	Port         int
	Active       bool
	Requests     int64
	Errors       int64
	Connections  int64
	LatencySum   int64
	LatencyCount int64
}

type Snapshot struct {
	Timestamp time.Time
	Rejected  int64
	Servers   []ServerSnapshot
}

type Collector struct {
	servers  *[]*backends.Server
	mu       *sync.RWMutex
	rejected *int64
	latest   Snapshot
	snapMu   sync.Mutex
}

func NewCollector(servers *[]*backends.Server, mu *sync.RWMutex, rejected *int64) *Collector {
	return &Collector{
		servers:  servers,
		mu:       mu,
		rejected: rejected,
	}
}

func (c *Collector) Start() {
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			c.collect()
		}
	}()
}

func (c *Collector) collect() {
	c.mu.RLock()
	snapshot := make([]*backends.Server, len(*c.servers))
	copy(snapshot, *c.servers)
	c.mu.RUnlock()

	serverSnapshots := make([]ServerSnapshot, len(snapshot))
	for i, s := range snapshot {
		serverSnapshots[i] = ServerSnapshot{
			Port:         s.Port,
			Active:       s.Active,
			Requests:     atomic.LoadInt64(&s.Requests),
			Errors:       atomic.LoadInt64(&s.Errors),
			Connections:  atomic.LoadInt64(&s.Connections),
			LatencySum:   atomic.LoadInt64(&s.LatencySum),
			LatencyCount: atomic.LoadInt64(&s.LatencyCount),
		}
	}

	c.snapMu.Lock()
	c.latest = Snapshot{
		Timestamp: time.Now(),
		Rejected:  atomic.LoadInt64(c.rejected),
		Servers:   serverSnapshots,
	}
	c.snapMu.Unlock()
}

func (c *Collector) Latest() Snapshot {
	c.snapMu.Lock()
	defer c.snapMu.Unlock()
	return c.latest
}