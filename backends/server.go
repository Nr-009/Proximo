package backends

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync/atomic"
	"time"
)

type Server struct {
	Port        int
	BaseLatency time.Duration
	Capacity    int
	ErrorRate   int
	Weight      int
	Active      bool
	IsCanary    bool
	Connections int64
	Errors      int64
	Requests    int64
	activeConns int64
	httpServer  *http.Server
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/health" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
		return
	}

	conns := atomic.AddInt64(&s.activeConns, 1)
	defer atomic.AddInt64(&s.activeConns, -1)

	if s.ErrorRate > 0 && rand.Intn(100) < s.ErrorRate {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	latency := s.BaseLatency
	if int(conns) > s.Capacity {
		multiplier := float64(conns) / float64(s.Capacity)
		latency = time.Duration(float64(s.BaseLatency) * multiplier)
	}

	time.Sleep(latency)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"server":     s.Port,
		"latency_ms": latency.Milliseconds(),
		"is_canary":  s.IsCanary,
	})
}

func Start(s *Server) {
	addr := fmt.Sprintf(":%d", s.Port)
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s,
	}

	log.Printf("[backend] server starting on port %d | latency %dms | capacity %d | weight %d | canary %v | error rate %d%%",
		s.Port, s.BaseLatency.Milliseconds(), s.Capacity, s.Weight, s.IsCanary, s.ErrorRate)

	if err := s.httpServer.ListenAndServe(); err != nil {
		if err == http.ErrServerClosed {
			log.Printf("[backend] server on port %d closed", s.Port)
		} else {
			log.Fatalf("[backend] server on port %d failed: %v", s.Port, err)
		}
	}
}

func (s *Server) Shutdown() {
	if s.httpServer != nil {
		log.Printf("[backend] shutting down server on port %d", s.Port)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.httpServer.Shutdown(ctx)
	}
}