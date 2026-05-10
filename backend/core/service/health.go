package service

import (
	"github.com/neochaotic/powerlab/backend/common/utils/port"
	"github.com/neochaotic/powerlab/backend/common/utils/systemctl"
)

// HealthService surfaces process + port liveness state for the
// admin diagnostic page. Used by the readiness probe + the "Why
// is X not working?" troubleshoot widget.
type HealthService interface {
	// Services returns systemctl-managed casaos* services keyed by
	// running-flag (true → []running, false → []not-running).
	Services() (map[bool]*[]string, error)
	// Ports returns the (tcp, udp) port lists currently in use on
	// the host. Used to flag conflicts before app install.
	Ports() ([]int, []int, error)
}

type service struct{}

func (s *service) Services() (map[bool]*[]string, error) {
	services, err := systemctl.ListServices("casaos*")
	if err != nil {
		return nil, err
	}

	var running, notRunning []string

	for _, service := range services {
		if service.Running {
			running = append(running, service.Name)
		} else {
			notRunning = append(notRunning, service.Name)
		}
	}

	result := map[bool]*[]string{
		true:  &running,
		false: &notRunning,
	}

	return result, nil
}

func (s *service) Ports() ([]int, []int, error) {
	return port.ListPortsInUse()
}

// NewHealthService returns a stateless HealthService.
func NewHealthService() HealthService {
	return &service{}
}
