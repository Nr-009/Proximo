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

func New(strategy string, servers *[]*backends.Server) (Balancer, error) {
	switch strategy {
	case "round-robin":
		return NewRoundRobin(servers), nil
	case "weighted":
		return NewWeighted(servers), nil
	case "least-conn":
		return NewLeastConn(servers), nil
	case "p2c":
		return NewPowerOfTwo(servers), nil
	default:
		return nil, errors.New("unknown strategy: " + strategy)
	}
}