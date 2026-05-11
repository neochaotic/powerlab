package service

import (
	"time"

	"github.com/neochaotic/powerlab/backend/common/utils/port"
	"github.com/neochaotic/powerlab/backend/common/utils/systemctl"
)

// HealthService surfaces process + port liveness state for the
// admin diagnostic page. Used by the readiness probe + the "Why
// is X not working?" troubleshoot widget.
type HealthService interface {
	// Services returns systemctl-managed PowerLab + legacy CasaOS
	// services keyed by running-flag (true → []running, false → []not-running).
	Services() (map[bool]*[]string, error)
	// Ports returns the (tcp, udp) port lists currently in use on
	// the host. Used to flag conflicts before app install.
	Ports() ([]int, []int, error)
}

// ServiceLister is the systemctl boundary the health service depends
// on. Injectable so tests can verify which patterns are queried
// without standing up a real systemd connection.
type ServiceLister func(pattern string, wait ...time.Duration) ([]systemctl.Service, error)

type service struct {
	lister ServiceLister
}

// healthServicePatterns names the glob patterns the operator's
// systemd is queried with. `powerlab-*` covers fresh PowerLab
// installs; `casaos*` is retained for co-resident installs where
// the user is migrating from CasaOS and still has legacy units
// (#101 / ADR-0021 ecosystem-compat). The kill of `casaos*` was
// #245 — dropping it stranded the health dashboard on a clean
// PowerLab box.
var healthServicePatterns = []string{"casaos*", "powerlab-*"}

func (s *service) Services() (map[bool]*[]string, error) {
	seen := make(map[string]struct{})
	var running, notRunning []string

	for _, pattern := range healthServicePatterns {
		services, err := s.lister(pattern)
		if err != nil {
			return nil, err
		}
		for _, svc := range services {
			if _, dup := seen[svc.Name]; dup {
				continue
			}
			seen[svc.Name] = struct{}{}
			if svc.Running {
				running = append(running, svc.Name)
			} else {
				notRunning = append(notRunning, svc.Name)
			}
		}
	}

	return map[bool]*[]string{
		true:  &running,
		false: &notRunning,
	}, nil
}

func (s *service) Ports() ([]int, []int, error) {
	return port.ListPortsInUse()
}

// NewHealthService returns a stateless HealthService backed by the
// real systemctl.ListServices.
func NewHealthService() HealthService {
	return &service{lister: systemctl.ListServices}
}

// NewHealthServiceWithLister returns a HealthService that calls the
// injected ServiceLister instead of systemctl.ListServices. Test-
// only seam — production code uses NewHealthService.
func NewHealthServiceWithLister(lister ServiceLister) HealthService {
	return &service{lister: lister}
}
