package balancer

import (
	"errors"

	"github.com/Nr-009/Proximo/backends"
)

type Balancer interface {
	NextServer() (*backends.Server, error)
	OnAddServer(s *backends.Server)
	OnRemoveServer(s *backends.Server)
}

func New(strategy string, servers *[]*backends.Server, cfg ...interface{}) (Balancer, error) {
	switch strategy {
	case "round-robin":
		return NewRoundRobin(servers), nil
	case "weighted":
		return NewWeighted(servers), nil
	case "least-conn":
		return NewLeastConn(servers), nil
	case "p2c":
		return NewPowerOfTwo(servers), nil
	case "canary":
		if len(cfg) < 3 {
			return nil, errors.New("canary requires canary_port, canary_percent, error_threshold")
		}
		port := cfg[0].(int)
		percent := cfg[1].(int)
		threshold := cfg[2].(float64)
		return NewCanary(servers, port, percent, threshold), nil
	default:
		return nil, errors.New("unknown strategy: " + strategy)
	}
}