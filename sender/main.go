package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

func readLine(prompt string) string {
	fmt.Print(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	return strings.TrimSpace(scanner.Text())
}

func readInt(prompt string, defaultVal int) int {
	input := readLine(prompt)
	if input == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(input)
	if err != nil {
		fmt.Printf("invalid input — using default %d\n", defaultVal)
		return defaultVal
	}
	return val
}

func main() {
	fmt.Println("=== Proximo Sender ===")
	fmt.Println()
	proxyURL := readLine("Proxy address [localhost:8080]: ")
	if proxyURL == "" {
		proxyURL = "localhost:8080"
	}

	adminURL := readLine("Admin address [localhost:9000]: ")
	if adminURL == "" {
		adminURL = "localhost:9000"
	}

	numSenders := readInt("How many senders? [1]: ", 1)

	type senderConfig struct {
		sender  *Sender
		pattern string
	}

	configs := make([]senderConfig, numSenders)
	for i := 0; i < numSenders; i++ {
		fmt.Printf("\n--- Sender %d ---\n", i+1)
		pattern := readLine("Pattern (steady/burst/wave) [steady]: ")
		if pattern == "" {
			pattern = "steady"
		}
		rate := readInt("Rate (req/s or total for burst) [10]: ", 10)
		durationSec := readInt("Duration in seconds [30]: ", 30)
		portMode := readLine("Port mode (random/fixed) [random]: ")
		if portMode == "" {
			portMode = "random"
		}
		port := 0
		if portMode == "fixed" {
			port = readInt("Fixed port [7001]: ", 7001)
		}
		configs[i] = senderConfig{
			sender: &Sender{
				Rate:     rate,
				Duration: time.Duration(durationSec) * time.Second,
				PortMode: portMode,
				Port:     port,
				ProxyURL: proxyURL,
			},
			pattern: pattern,
		}
	}

	fmt.Println("\n=== Starting senders ===")
	http.Post("http://"+adminURL+"/metrics/start", "application/json", nil)

	var wg sync.WaitGroup
	results := make([]*Result, numSenders)
	for i, cfg := range configs {
		results[i] = &Result{}
		wg.Add(1)
		go func(idx int, c senderConfig, result *Result) {
			defer wg.Done()
			switch c.pattern {
			case "steady":
				c.sender.Steady(result)
			case "burst":
				c.sender.Burst(result)
			case "wave":
				c.sender.Wave(result)
			default:
				fmt.Printf("[sender %d] unknown pattern %s\n", idx+1, c.pattern)
			}
		}(i+1, cfg, results[i])
	}

	wg.Wait()
	http.Post("http://"+adminURL+"/metrics/stop", "application/json", nil)

	fmt.Println("\n=== Results ===")
	totalSuccess := int64(0)
	totalErrors := int64(0)
	totalRequests := int64(0)
	for i, r := range results {
		fmt.Printf("Sender %d — total: %d | success: %d | errors: %d\n",
			i+1, r.Total, r.Success, r.Errors)
		totalSuccess += r.Success
		totalErrors += r.Errors
		totalRequests += r.Total
	}
	fmt.Println("---")
	fmt.Printf("Overall — total: %d | success: %d | errors: %d\n",
		totalRequests, totalSuccess, totalErrors)
}