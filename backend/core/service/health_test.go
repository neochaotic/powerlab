package service_test

import (
	"runtime"
	"testing"
	"time"

	"github.com/neochaotic/powerlab/backend/common/utils/systemctl"
	"github.com/neochaotic/powerlab/backend/core/service"
	"github.com/stretchr/testify/assert"
)

func TestPorts(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("skipping test on non-linux system")
	}
	service := service.NewHealthService()

	tcpPorts, udpPorts, err := service.Ports()
	assert.NoError(t, err)

	assert.NotEmpty(t, tcpPorts)
	assert.NotEmpty(t, udpPorts)
}

// Closes #245 — Services() called ListServices("casaos*") only, so
// PowerLab fresh installs (where units are powerlab-*) saw an empty
// health dashboard. Must now query both glob namespaces and dedupe.
func TestServices_QueriesBothCasaosAndPowerLabGlobs(t *testing.T) {
	var capturedPatterns []string
	fake := func(pattern string, _ ...time.Duration) ([]systemctl.Service, error) {
		capturedPatterns = append(capturedPatterns, pattern)
		switch pattern {
		case "casaos*":
			return []systemctl.Service{{Name: "casaos-legacy.service", Running: true}}, nil
		case "powerlab-*":
			return []systemctl.Service{
				{Name: "powerlab-core.service", Running: true},
				{Name: "powerlab-gateway.service", Running: false},
			}, nil
		}
		return nil, nil
	}

	svc := service.NewHealthServiceWithLister(fake)
	result, err := svc.Services()
	assert.NoError(t, err)

	assert.ElementsMatch(t, []string{"casaos*", "powerlab-*"}, capturedPatterns,
		"both globs must be queried — the CasaOS-only glob was the #245 bug")

	running := *result[true]
	notRunning := *result[false]
	assert.ElementsMatch(t,
		[]string{"casaos-legacy.service", "powerlab-core.service"},
		running)
	assert.ElementsMatch(t,
		[]string{"powerlab-gateway.service"},
		notRunning)
}

// A unit that matches both globs (would be a pathological case but
// still worth locking) must NOT appear twice in the result.
func TestServices_DedupesAcrossGlobs(t *testing.T) {
	fake := func(pattern string, _ ...time.Duration) ([]systemctl.Service, error) {
		// Same unit name returned from both glob queries.
		return []systemctl.Service{{Name: "shared.service", Running: true}}, nil
	}

	svc := service.NewHealthServiceWithLister(fake)
	result, err := svc.Services()
	assert.NoError(t, err)

	all := append(*result[true], *result[false]...)
	assert.Len(t, all, 1, "duplicates across glob queries must be deduped")
	assert.Equal(t, "shared.service", all[0])
}
