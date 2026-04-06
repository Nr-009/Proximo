package balancer

import (
	"errors"

	"github.com/Nr-009/Proximo/backends"
)

type Balancer interface {
	NextServer() (*backends.Server, error)
	AddServer(s *backends.Server)
	RemoveServer(s *backends.Server)
}

func New(strategy string, servers []*backends.Server) (Balancer, error) {
	switch strategy {
	case "round-robin":
		return NewRoundRobin(servers), nil
	case "weighted":
		return NewWeighted(servers), nil
	default:
		return nil, errors.New("unknown strategy: " + strategy)
	}
}