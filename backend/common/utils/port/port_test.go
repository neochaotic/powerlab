package port_test

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/neochaotic/powerlab/backend/common/utils/port"
	"github.com/stretchr/testify/assert"
)

func TestPortAvailable(t *testing.T) {
	//	fmt.Println(PortAvailable())
	// fmt.Println(IsPortAvailable(6881,"tcp"))
	p, _ := port.GetAvailablePort("udp")
	fmt.Println("udp", p)
	fmt.Println(port.IsPortAvailable(p, "udp"))

	t1, _ := port.GetAvailablePort("tcp")
	fmt.Println("tcp", t1)
	fmt.Println(port.IsPortAvailable(t1, "tcp"))
}

func TestPorts(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("ListPortsInUse reads /proc/net/* — Linux only")
	}
	tcpPorts, udpPorts, err := port.ListPortsInUse()
	assert.NoError(t, err)

	assert.NotEmpty(t, tcpPorts)
	assert.NotEmpty(t, udpPorts)
}
