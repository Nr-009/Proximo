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
	"github.com/Nr-009/Proximo/health"
	"github.com/Nr-009/Proximo/ratelimit"
)

type Proxy struct {
	backends      *[]*backends.Server
	balancer      balancer.Balancer
	limiter       ratelimit.Limiter
	blacklist     *ratelimit.Blacklist
	mu            sync.RWMutex
	trafficServer *http.Server
	adminServer   *http.Server
}

func New() *Proxy {
	backends := make([]*backends.Server, 0)
	return &Proxy{
		backends:  &backends,
		blacklist: ratelimit.NewBlacklist(10),
	}
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
			IsCanary:    c.IsCanary,
			Active:      true,
		}
		*p.backends = append(*p.backends, s)
		go backends.Start(s)

		if p.balancer != nil {
			p.balancer.OnAddServer(s)
		}

		log.Printf("[proxy] added backend on port %d | canary %v", c.Port, c.IsCanary)
	}
}

func (p *Proxy) SetStrategy(strategy string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	b, err := balancer.New(strategy, p.backends)
	if err != nil {
		return err
	}

	p.balancer = b
	log.Printf("[proxy] strategy set to %s", strategy)
	return nil
}

func (p *Proxy) SetCanaryStrategy(cfg config.StrategyConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	b, err := balancer.New("canary", p.backends, cfg.CanaryPort, cfg.CanaryPercent, cfg.ErrorThreshold)
	if err != nil {
		return err
	}

	p.balancer = b
	log.Printf("[proxy] canary strategy set — port %d at %d%% threshold %.0f%%",
		cfg.CanaryPort, cfg.CanaryPercent, cfg.ErrorThreshold*100)
	return nil
}

func (p *Proxy) SetLimiter(limiter ratelimit.Limiter) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.limiter = limiter
	log.Printf("[proxy] rate limiter updated")
}

func (p *Proxy) Start(trafficPort, adminPort int) {
	health.StartHealthChecker(p.backends, &p.mu)
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
	mux.HandleFunc("/ratelimit", p.handleRateLimit)
	mux.HandleFunc("/blacklist", p.handleBlacklist)
	mux.HandleFunc("/kill", p.handleKill)
	mux.HandleFunc("/resurrect", p.handleResurrect)
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

	var err error
	if cfg.Strategy == "canary" {
		err = p.SetCanaryStrategy(cfg)
	} else {
		err = p.SetStrategy(cfg.Strategy)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "strategy updated"})
}

func (p *Proxy) handleRateLimit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var cfg config.RateLimitConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var limiter ratelimit.Limiter
	switch cfg.Algorithm {
	case "token-bucket":
		limiter = ratelimit.NewTokenBucket(cfg.RequestsPerSecond, cfg.BucketSize)
	case "sliding-window":
		limiter = ratelimit.NewSlidingWindow(cfg.RequestsPerSecond, cfg.WindowSeconds)
	case "leaky-bucket":
		limiter = ratelimit.NewLeakyBucket(cfg.RequestsPerSecond, cfg.BucketSize)
	case "concurrent":
		limiter = ratelimit.NewConcurrentLimiter(cfg.MaxConcurrent)
	default:
		http.Error(w, fmt.Sprintf("unknown algorithm: %s", cfg.Algorithm), http.StatusBadRequest)
		return
	}

	p.SetLimiter(limiter)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "rate limiter updated"})
}

func (p *Proxy) handleBlacklist(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var body struct {
			Port string `json:"port"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		p.blacklist.Block(body.Port)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "port blacklisted"})

	case http.MethodGet:
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(p.blacklist.Status())

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (p *Proxy) handleKill(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Port int `json:"port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	p.mu.RLock()
	var target *backends.Server
	for _, s := range *p.backends {
		if s.Port == body.Port {
			target = s
			break
		}
	}
	p.mu.RUnlock()

	if target == nil {
		http.Error(w, "server not found", http.StatusNotFound)
		return
	}

	target.Shutdown()
	log.Printf("[proxy] killed server on port %d", body.Port)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "server killed"})
}

func (p *Proxy) handleResurrect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Port int `json:"port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	p.mu.RLock()
	var target *backends.Server
	for _, s := range *p.backends {
		if s.Port == body.Port {
			target = s
			break
		}
	}
	p.mu.RUnlock()

	if target == nil {
		http.Error(w, "server not found", http.StatusNotFound)
		return
	}

	go backends.Start(target)
	log.Printf("[proxy] resurrecting server on port %d", body.Port)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "server resurrecting"})
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

	for _, b := range *p.backends {
		b.Shutdown()
	}

	if p.adminServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		p.adminServer.Shutdown(ctx)
	}

	log.Println("[proxy] all servers stopped cleanly")
}