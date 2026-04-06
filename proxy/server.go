package proxy

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"
)

type Server struct {
	httpServer *http.Server
}

func NewServer(port int) *Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "load balancer ready — forwarding coming in step 1.3")
	})

	return &Server{
		httpServer: &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: mux,
		},
	}
}

func (s *Server) Start() {
	log.Printf("[proxy] load balancer starting on %s", s.httpServer.Addr)
	if err := s.httpServer.ListenAndServe(); err != nil {
		if err == http.ErrServerClosed {
			log.Printf("[proxy] load balancer closed")
		} else {
			log.Fatalf("[proxy] failed to start: %v", err)
		}
	}
}

func (s *Server) Shutdown() {
	log.Printf("[proxy] shutting down load balancer")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.httpServer.Shutdown(ctx)
}