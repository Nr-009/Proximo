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
	Active      bool
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
		http.Error(w, "internal server error", http.StatusInternalServerError)
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
		"server":       s.Port,
		"latency_ms":   latency.Milliseconds(),
		"active_conns": conns,
	})
}

func (s *Server) ActiveConns() int64 {
	return atomic.LoadInt64(&s.activeConns)
}

func Start(s *Server) {
	s.Active = true
	addr := fmt.Sprintf(":%d", s.Port)
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s,
	}

	log.Printf("[backend] server starting on port %d | latency %dms | capacity %d | error rate %d%%",
		s.Port, s.BaseLatency.Milliseconds(), s.Capacity, s.ErrorRate)

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
		s.Active = false
		log.Printf("[backend] shutting down server on port %d", s.Port)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.httpServer.Shutdown(ctx)
	}
}