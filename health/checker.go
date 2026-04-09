package health

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/Nr-009/Proximo/backends"
)

func StartHealthChecker(servers *[]*backends.Server, mu *sync.RWMutex) {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		failures := make(map[int]int)

		for range ticker.C {
			mu.RLock()
			snapshot := make([]*backends.Server, len(*servers))
			copy(snapshot, *servers)
			mu.RUnlock()

			for _, s := range snapshot {
				resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", s.Port))

				if err != nil || resp.StatusCode != 200 {
					failures[s.Port]++
					if failures[s.Port] >= 2 {
						mu.Lock()
						if s.Active {
							s.Active = false
							log.Printf("[health] server %d marked inactive after %d failures", s.Port, failures[s.Port])
						}
						mu.Unlock()
					}
				} else {
					mu.Lock()
					if !s.Active {
						s.Active = true
						log.Printf("[health] server %d resurrected", s.Port)
					}
					mu.Unlock()
					failures[s.Port] = 0
				}

				if resp != nil {
					resp.Body.Close()
				}
			}
		}
	}()
}