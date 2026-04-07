package main

import (
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type Sender struct {
	Rate     int
	Duration time.Duration
	PortMode string // random or fixed
	Port     int    // specific port if fixed
	ProxyURL string
}

type Result struct {
	Success int64
	Errors  int64
	Total   int64
}

func (s *Sender) makeRequest(result *Result) {
	client := &http.Client{Timeout: 5 * time.Second}

	url := fmt.Sprintf("http://%s", s.ProxyURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		atomic.AddInt64(&result.Errors, 1)
		atomic.AddInt64(&result.Total, 1)
		return
	}

	if s.PortMode == "fixed" {
		req.Header.Set("X-Source-Port", fmt.Sprintf("%d", s.Port))
	} else {
		req.Header.Set("X-Source-Port", fmt.Sprintf("%d", rand.Intn(60000)+1024))
	}

	resp, err := client.Do(req)
	if err != nil {
		atomic.AddInt64(&result.Errors, 1)
		atomic.AddInt64(&result.Total, 1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		atomic.AddInt64(&result.Success, 1)
	} else {
		atomic.AddInt64(&result.Errors, 1)
	}
	atomic.AddInt64(&result.Total, 1)
}

func (s *Sender) Steady(result *Result) {
	if s.Rate <= 0 {
		return
	}

	interval := time.Duration(float64(time.Second) / float64(s.Rate))
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	deadline := time.After(s.Duration)

	fmt.Printf("[sender] steady — %d req/s for %s\n", s.Rate, s.Duration)

	for {
		select {
		case <-deadline:
			return
		case <-ticker.C:
			go s.makeRequest(result)
		}
	}
}

func (s *Sender) Burst(result *Result) {
	total := s.Rate // treat rate as total requests in burst
	fmt.Printf("[sender] burst — %d requests all at once\n", total)

	var wg sync.WaitGroup
	for i := 0; i < total; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.makeRequest(result)
		}()
	}
	wg.Wait()
}

func (s *Sender) Wave(result *Result) {
	minRate := s.Rate / 4
	maxRate := s.Rate

	fmt.Printf("[sender] wave — %d to %d req/s over %s\n", minRate, maxRate, s.Duration)

	deadline := time.After(s.Duration)
	start := time.Now()

	for {
		select {
		case <-deadline:
			return
		default:
			elapsed := time.Since(start).Seconds()
			total := s.Duration.Seconds()

			// sine curve — ramps up then back down
			sineValue := math.Sin(math.Pi * elapsed / total)
			currentRate := float64(minRate) + float64(maxRate-minRate)*sineValue

			if currentRate > 0 {
				interval := time.Duration(float64(time.Second) / currentRate)
				time.Sleep(interval)
				go s.makeRequest(result)
			}
		}
	}
}