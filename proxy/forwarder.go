package proxy

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"sync/atomic"

	"github.com/Nr-009/Proximo/backends"
	"github.com/Nr-009/Proximo/balancer"
)

func (p *Proxy) callBackend(r *http.Request, backend *backends.Server) (*http.Response, error) {
	url := fmt.Sprintf("http://localhost:%d%s", backend.Port, r.URL.Path)
	if r.URL.RawQuery != "" {
		url += "?" + r.URL.RawQuery
	}

	req, err := http.NewRequest(r.Method, url, r.Body)
	if err != nil {
		return nil, err
	}

	for key, values := range r.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	client := &http.Client{}
	return client.Do(req)
}

func (p *Proxy) forward(w http.ResponseWriter, r *http.Request) {
	clientPort := r.Header.Get("X-Source-Port")
	if clientPort == "" {
		clientPort = "unknown"
	}

	p.mu.RLock()
	blacklist := p.blacklist
	limiter := p.limiter
	p.mu.RUnlock()

	if blacklist.IsBlocked(clientPort) {
		log.Printf("[blacklist] client %s blocked — 403", clientPort)
		atomic.AddInt64(&p.Rejected, 1)
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if limiter != nil {
		if !limiter.Allow(clientPort) {
			log.Printf("[ratelimit] client %s rejected — too many requests", clientPort)
			blacklist.RecordViolation(clientPort)
			atomic.AddInt64(&p.Rejected, 1)
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		defer limiter.Done(clientPort)
	}

	p.mu.RLock()
	b := p.balancer
	if b == nil {
		p.mu.RUnlock()
		http.Error(w, "no strategy configured", http.StatusServiceUnavailable)
		return
	}
	backend, err := b.NextServer()
	p.mu.RUnlock()

	if err != nil {
		log.Printf("[proxy] no backend available: %v", err)
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}

	atomic.AddInt64(&backend.Requests, 1)
	atomic.AddInt64(&backend.Connections, 1)
	defer atomic.AddInt64(&backend.Connections, -1)

	resp, err := p.callBackend(r, backend)
	if err != nil {
		log.Printf("[proxy] backend %d unreachable: %v", backend.Port, err)
		atomic.AddInt64(&backend.Errors, 1)
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return
	}

	if resp.StatusCode >= 500 && backend.IsCanary {
		log.Printf("[proxy] canary %d failed with %d — retrying on stable", backend.Port, resp.StatusCode)
		atomic.AddInt64(&backend.Errors, 1)
		resp.Body.Close()

		canaryBalancer, ok := b.(*balancer.Canary)
		if !ok {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}

		stable, err := canaryBalancer.NextStable()
		if err != nil {
			log.Printf("[proxy] no stable server after canary failure: %v", err)
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}

		atomic.AddInt64(&stable.Requests, 1)
		atomic.AddInt64(&stable.Connections, 1)
		defer atomic.AddInt64(&stable.Connections, -1)

		resp, err = p.callBackend(r, stable)
		if err != nil {
			log.Printf("[proxy] stable backend %d unreachable: %v", stable.Port, err)
			atomic.AddInt64(&stable.Errors, 1)
			http.Error(w, "bad gateway", http.StatusBadGateway)
			return
		}
	} else if resp.StatusCode >= 500 {
		atomic.AddInt64(&backend.Errors, 1)
		log.Printf("[proxy] backend %d returned %d", backend.Port, resp.StatusCode)
	}

	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}