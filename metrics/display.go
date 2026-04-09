package metrics

import (
	"fmt"
	"sync/atomic"
	"time"
)

func StartLiveDisplay(c *Collector, stop <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		start := time.Now()

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				snap := c.Latest()
				if len(snap.Servers) == 0 {
					continue
				}

				elapsed := int(time.Since(start).Seconds())
				var totalRequests int64
				var totalErrors int64
				for _, s := range snap.Servers {
					totalRequests += s.Requests
					totalErrors += s.Errors
				}
				totalRejected := atomic.LoadInt64(c.rejected)

				fmt.Printf("\n[t=%ds] Total: %d | Forwarded: %d | Rejected: %d | Errors: %d\n",
					elapsed, totalRequests+totalRejected, totalRequests, totalRejected, totalErrors)

				for _, s := range snap.Servers {
					status := "up"
					if !s.Active {
						status = "DOWN"
					}
					avgLatency := int64(0)
					if s.LatencyCount > 0 {
						avgLatency = s.LatencySum / s.LatencyCount
					}
					fmt.Printf("  %d [%s]: %d req | %d err | %d conns | avg %dms\n",
						s.Port, status, s.Requests, s.Errors, s.Connections, avgLatency)
				}
			}
		}
	}()
}

func PrintFinalSummary(c *Collector) {
	snap := c.Latest()

	fmt.Println("\n================================================================================")
	fmt.Println("FINAL SUMMARY")
	fmt.Println("================================================================================")
	fmt.Printf("%-8s %-10s %-10s %-10s %-12s %-10s\n",
		"Server", "Requests", "Errors", "Error%", "Connections", "Avg ms")
	fmt.Println("------------------------------------------------------------------------")

	var totalRequests int64
	var totalErrors int64
	var totalLatencySum int64
	var totalLatencyCount int64

	for _, s := range snap.Servers {
		errPct := 0.0
		if s.Requests > 0 {
			errPct = float64(s.Errors) / float64(s.Requests) * 100
		}
		avgLatency := int64(0)
		if s.LatencyCount > 0 {
			avgLatency = s.LatencySum / s.LatencyCount
		}
		status := ""
		if !s.Active {
			status = " [DOWN]"
		}
		fmt.Printf("%-8d %-10d %-10d %-10.1f %-12d %-10d%s\n",
			s.Port, s.Requests, s.Errors, errPct, s.Connections, avgLatency, status)
		totalRequests += s.Requests
		totalErrors += s.Errors
		totalLatencySum += s.LatencySum
		totalLatencyCount += s.LatencyCount
	}

	fmt.Println("------------------------------------------------------------------------")
	totalErrPct := 0.0
	if totalRequests > 0 {
		totalErrPct = float64(totalErrors) / float64(totalRequests) * 100
	}
	totalAvgLatency := int64(0)
	if totalLatencyCount > 0 {
		totalAvgLatency = totalLatencySum / totalLatencyCount
	}
	fmt.Printf("%-8s %-10d %-10d %-10.1f %-12s %-10d\n",
		"Total", totalRequests, totalErrors, totalErrPct, "-", totalAvgLatency)
	fmt.Printf("\nRejected (rate limit / blacklist): %d\n", snap.Rejected)
	fmt.Println("================================================================================")
}