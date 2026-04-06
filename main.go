package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Nr-009/Proximo/backends"
	"github.com/Nr-009/Proximo/proxy"
)

func main() {
	servers := []*backends.Server{
		{Port: 9001, BaseLatency: 30 * time.Millisecond, Capacity: 50, ErrorRate: 0},
		{Port: 9002, BaseLatency: 30 * time.Millisecond, Capacity: 20, ErrorRate: 0},
		{Port: 9003, BaseLatency: 30 * time.Millisecond, Capacity: 5, ErrorRate: 10},
	}

	for _, s := range servers {
		go backends.Start(s)
	}

	time.Sleep(100 * time.Millisecond)

	proxyServer := proxy.NewServer(8080)
	go proxyServer.Start()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[main] shutdown signal received — stopping all servers")

	proxyServer.Shutdown()
	for _, s := range servers {
		s.Shutdown()
	}

	log.Println("[main] all servers stopped cleanly")
}