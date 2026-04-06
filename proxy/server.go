package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/Nr-009/Proximo/backends"
	"github.com/Nr-009/Proximo/balancer"
	"github.com/Nr-009/Proximo/config"
)

type Proxy struct {
	backends      []*backends.Server
	balancer      balancer.Balancer
	mu            sync.RWMutex
	trafficServer *http.Server
	adminServer   *http.Server
}

func New() *Proxy {
	return &Proxy{}
}

func (p *Proxy) AddServers(configs []config.ServerConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, c := range configs {
		s := &backends.Server{
			Port:        c.Port,
			BaseLatency: time.Duration(c.LatencyMS) * time.Millisecond,
			Capacity:    c.Capacity,
			ErrorRate:   c.ErrorRate,
			Weight:      c.Weight,
			Active:      true,
		}
		p.backends = append(p.backends, s)
		go backends.Start(s)

		// notify balancer to update internal state for new server
		if p.balancer != nil {
			p.balancer.OnAddServer(s)
		}

		log.Printf("[proxy] added backend on port %d", c.Port)
	}
}

func (p *Proxy) SetStrategy(strategy string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	b, err := balancer.New(strategy, &p.backends)
	if err != nil {
		return err
	}

	p.balancer = b
	log.Printf("[proxy] strategy set to %s", strategy)
	return nil
}

func (p *Proxy) Start(trafficPort, adminPort int) {
	go p.startAdmin(adminPort)
	time.Sleep(100 * time.Millisecond)
	p.startTraffic(trafficPort)
}

func (p *Proxy) startTraffic(port int) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", p.forward)

	p.trafficServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	log.Printf("[proxy] traffic listener starting on port %d", port)
	if err := p.trafficServer.ListenAndServe(); err != nil {
		if err == http.ErrServerClosed {
			log.Printf("[proxy] traffic listener closed")
		} else {
			log.Fatalf("[proxy] traffic listener failed: %v", err)
		}
	}
}

func (p *Proxy) startAdmin(port int) {
	mux := http.NewServeMux()
	mux.HandleFunc("/servers", p.handleServers)
	mux.HandleFunc("/strategy", p.handleStrategy)
	mux.HandleFunc("/shutdown", p.handleShutdown)

	p.adminServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	log.Printf("[proxy] admin listener starting on port %d", port)
	if err := p.adminServer.ListenAndServe(); err != nil {
		if err == http.ErrServerClosed {
			log.Printf("[proxy] admin listener closed")
		} else {
			log.Fatalf("[proxy] admin listener failed: %v", err)
		}
	}
}

func (p *Proxy) handleServers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var configs []config.ServerConfig
	if err := json.NewDecoder(r.Body).Decode(&configs); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	p.AddServers(configs)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "servers added"})
}

func (p *Proxy) handleStrategy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var cfg config.StrategyConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := p.SetStrategy(cfg.Strategy); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "strategy updated"})
}

func (p *Proxy) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Println("[proxy] shutdown command received")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "shutting down"})

	go p.Shutdown()
}

func (p *Proxy) Shutdown() {
	p.mu.Lock()
	defer p.mu.Unlock()

	log.Println("[proxy] shutting down everything")

	if p.trafficServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		p.trafficServer.Shutdown(ctx)
	}

	for _, b := range p.backends {
		b.Shutdown()
	}

	if p.adminServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		p.adminServer.Shutdown(ctx)
	}

	log.Println("[proxy] all servers stopped cleanly")
}