package proxy

import (
	"fmt"
	"io"
	"log"
	"net/http"
)

func (p *Proxy) forward(w http.ResponseWriter, r *http.Request) {
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

	url := fmt.Sprintf("http://localhost:%d%s", backend.Port, r.URL.Path)
	if r.URL.RawQuery != "" {
		url += "?" + r.URL.RawQuery
	}

	req, err := http.NewRequest(r.Method, url, r.Body)
	if err != nil {
		log.Printf("[proxy] failed to create request: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	for key, values := range r.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[proxy] backend %d failed: %v", backend.Port, err)
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return
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