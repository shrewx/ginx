package service_discovery

import (
	"fmt"
	"github.com/hashicorp/go-uuid"
)

type ServiceDiscovery interface {
	Watch(service ServiceInfo) error
}

type ServiceInfo struct {
	ID      string
	Tags    []string
	Name    string
	Address string
	Port    int

	Weight int

	HealthPath     string
	Timeout        int
	Interval       int
	DeregisterTime int
}

func (s *ServiceInfo) Default() {
	if s.Timeout == 0 {
		s.Timeout = 5
	}
	if s.Interval == 0 {
		s.Interval = 5
	}
	if s.DeregisterTime == 0 {
		s.DeregisterTime = 30
	}
	if s.HealthPath == "" {
		s.HealthPath = "/health"
	}
	if s.ID == "" {
		id, _ := uuid.GenerateUUID()
		s.ID = fmt.Sprintf("%s-%s", s.Name, id)
	}
}
