package service

// Tests for the auto-port-remap logic in compose_service.go.
//
// These cover the key behaviors:
//   - Free ports stay as-is (no remap on a vacant host).
//   - Conflicting ports get pushed to the next available number.
//   - TCP+UDP that share the same published number always migrate together
//     (DNS-style services like AdGuard's 53/tcp + 53/udp).
//   - The returned remap map is what callers use to update x-casaos.port_map.
//
// We construct ComposeApp directly (not via NewComposeAppFromYAML) so the test
// is hermetic — no temp dirs, no compose-go loader, no Docker.

import (
	"net"
	"strconv"
	"testing"

	"github.com/IceWhaleTech/CasaOS-AppManagement/codegen"
	"github.com/compose-spec/compose-go/types"
	"gotest.tools/v3/assert"
)

// occupyTCP/UDP bind on 0.0.0.0 because that's what portutil.IsPortAvailable
// uses internally. Binding on 127.0.0.1 only would NOT register as "in use"
// from IsPortAvailable's perspective on most kernels.
func occupyTCP(t *testing.T, port int) func() {
	t.Helper()
	l, err := net.Listen("tcp", net.JoinHostPort("0.0.0.0", strconv.Itoa(port)))
	if err != nil {
		t.Skipf("could not occupy port %d for test: %v", port, err)
	}
	return func() { _ = l.Close() }
}

func occupyUDP(t *testing.T, port int) func() {
	t.Helper()
	addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort("0.0.0.0", strconv.Itoa(port)))
	if err != nil {
		t.Skipf("could not resolve udp %d: %v", port, err)
	}
	c, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Skipf("could not occupy udp port %d: %v", port, err)
	}
	return func() { _ = c.Close() }
}

// makeApp builds a minimal ComposeApp from a list of (published, proto) tuples.
// Each tuple becomes a single ServicePortConfig in a single service.
func makeApp(name string, ports ...struct{ pub, proto string }) *ComposeApp {
	svcPorts := make([]types.ServicePortConfig, 0, len(ports))
	for _, p := range ports {
		svcPorts = append(svcPorts, types.ServicePortConfig{
			Published: p.pub,
			Protocol:  p.proto,
			Mode:      "ingress",
		})
	}
	app := &ComposeApp{
		Name: name,
		Services: types.Services{
			types.ServiceConfig{
				Name:  name,
				Image: "test/image:latest",
				Ports: svcPorts,
			},
		},
		Extensions: map[string]interface{}{},
	}
	return app
}

// findFreePort asks the OS for an unused port (so tests don't fight each other).
func findFreePort(t *testing.T, proto string) int {
	t.Helper()
	if proto == "udp" {
		c, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
		if err != nil {
			t.Fatalf("could not allocate udp port: %v", err)
		}
		defer c.Close()
		return c.LocalAddr().(*net.UDPAddr).Port
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not allocate tcp port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// ─── Tests ──────────────────────────────────────────────────────────────

func TestAutoRemapPorts_AllFree(t *testing.T) {
	freeA := findFreePort(t, "tcp")
	freeB := findFreePort(t, "tcp")
	app := makeApp("free-app",
		struct{ pub, proto string }{strconv.Itoa(freeA), "tcp"},
		struct{ pub, proto string }{strconv.Itoa(freeB), "tcp"},
	)

	remap := autoRemapPorts(app)

	assert.Equal(t, len(remap), 0, "no remaps should happen when ports are free")
	assert.Equal(t, app.Services[0].Ports[0].Published, strconv.Itoa(freeA))
	assert.Equal(t, app.Services[0].Ports[1].Published, strconv.Itoa(freeB))
}

func TestAutoRemapPorts_TCPConflictRemaps(t *testing.T) {
	taken := findFreePort(t, "tcp")
	close := occupyTCP(t, taken)
	defer close()

	app := makeApp("tcp-conflict",
		struct{ pub, proto string }{strconv.Itoa(taken), "tcp"},
	)

	remap := autoRemapPorts(app)

	assert.Equal(t, len(remap), 1, "one port should have been remapped")
	newStr := remap[strconv.Itoa(taken)]
	assert.Assert(t, newStr != "", "old → new mapping must be present")

	newInt, _ := strconv.Atoi(newStr)
	assert.Assert(t, newInt > taken, "new port must be above the original")
	assert.Equal(t, app.Services[0].Ports[0].Published, newStr,
		"the in-place mutation must apply to the compose project")
}

// Critical regression: previously the remap incorrectly treated TCP/531 as
// blocking UDP/531 (different protocols). After the fix they are tracked
// independently AND when both protocols share the same Published port they
// must always end up on the SAME new port (DNS pairing).
func TestAutoRemapPorts_TCPandUDPPairedRemapTogether(t *testing.T) {
	// Pick a port and occupy it for BOTH tcp and udp so the pair must remap together.
	port := findFreePort(t, "tcp")
	closeT := occupyTCP(t, port)
	defer closeT()
	closeU := occupyUDP(t, port)
	defer closeU()

	app := makeApp("dns-app",
		struct{ pub, proto string }{strconv.Itoa(port), "tcp"},
		struct{ pub, proto string }{strconv.Itoa(port), "udp"},
	)

	remap := autoRemapPorts(app)

	assert.Equal(t, len(remap), 1, "tcp+udp share the same Published, so one entry in remap")
	tcpNew := app.Services[0].Ports[0].Published
	udpNew := app.Services[0].Ports[1].Published
	assert.Equal(t, tcpNew, udpNew,
		"paired tcp+udp must remap to the same port to preserve protocol convention")
	assert.Assert(t, tcpNew != strconv.Itoa(port), "should have moved off the original")
}

// When TCP is taken but UDP at the same number is free, the pair STILL
// migrates together — keeping both protocols on whichever number ends up free.
func TestAutoRemapPorts_PartialConflictMigratesPair(t *testing.T) {
	port := findFreePort(t, "tcp")
	closeT := occupyTCP(t, port) // only TCP is taken
	defer closeT()

	app := makeApp("dns-half-blocked",
		struct{ pub, proto string }{strconv.Itoa(port), "tcp"},
		struct{ pub, proto string }{strconv.Itoa(port), "udp"},
	)

	remap := autoRemapPorts(app)

	assert.Equal(t, len(remap), 1)
	tcpNew := app.Services[0].Ports[0].Published
	udpNew := app.Services[0].Ports[1].Published
	assert.Equal(t, tcpNew, udpNew, "even when only TCP conflicts, the pair migrates together")
}

// updateStorePortMap should rewrite x-casaos.port_map / web / port to match remaps.
func TestUpdateStorePortMap_RewritesPortMap(t *testing.T) {
	app := &ComposeApp{
		Name:     "rewrite-test",
		Services: types.Services{},
		Extensions: map[string]interface{}{
			"x-casaos": map[string]interface{}{
				"port_map": "8080",
			},
		},
	}

	remap := map[string]string{"8080": "8081"}
	updateStorePortMap(app, remap)

	xc := app.Extensions["x-casaos"].(map[string]interface{})
	assert.Equal(t, xc["port_map"].(string), "8081",
		"port_map must be rewritten to match the remap")
}

func TestUpdateStorePortMap_NoOpWhenNoRemap(t *testing.T) {
	app := &ComposeApp{
		Extensions: map[string]interface{}{
			"x-casaos": map[string]interface{}{"port_map": "8080"},
		},
	}
	updateStorePortMap(app, nil)
	xc := app.Extensions["x-casaos"].(map[string]interface{})
	assert.Equal(t, xc["port_map"].(string), "8080")
}

// ─── remapVolumePaths tests ─────────────────────────────────────────────

func TestRemapVolumePaths_NoOpWhenStoragePathIsDataDefault(t *testing.T) {
	app := &ComposeApp{
		Services: types.Services{
			types.ServiceConfig{
				Name: "x",
				Volumes: []types.ServiceVolumeConfig{
					{Type: "bind", Source: "/DATA/AppData/$AppID/config", Target: "/config"},
				},
			},
		},
	}
	remapVolumePaths(app, "/DATA")
	assert.Equal(t, app.Services[0].Volumes[0].Source, "/DATA/AppData/$AppID/config",
		"no rewrite when storage path equals /DATA")

	// Empty string is treated the same as no-op.
	remapVolumePaths(app, "")
	assert.Equal(t, app.Services[0].Volumes[0].Source, "/DATA/AppData/$AppID/config")
}

func TestRemapVolumePaths_RewritesDATAPrefix(t *testing.T) {
	app := &ComposeApp{
		Services: types.Services{
			types.ServiceConfig{
				Name: "x",
				Volumes: []types.ServiceVolumeConfig{
					{Type: "bind", Source: "/DATA/AppData/syncthing/config", Target: "/config"},
					{Type: "bind", Source: "/DATA", Target: "/DATA"},
					{Type: "bind", Source: "/etc/timezone", Target: "/etc/timezone"}, // unrelated, should NOT be touched
				},
			},
		},
	}
	remapVolumePaths(app, "/tmp/powerlab-data")

	vols := app.Services[0].Volumes
	assert.Equal(t, vols[0].Source, "/tmp/powerlab-data/AppData/syncthing/config")
	assert.Equal(t, vols[1].Source, "/tmp/powerlab-data")
	assert.Equal(t, vols[2].Source, "/etc/timezone", "non-/DATA paths are untouched")
}

func TestRemapVolumePaths_IgnoresNonBindVolumes(t *testing.T) {
	app := &ComposeApp{
		Services: types.Services{
			types.ServiceConfig{
				Name: "x",
				Volumes: []types.ServiceVolumeConfig{
					{Type: "volume", Source: "/DATA/named", Target: "/data"}, // named volume, not bind
				},
			},
		},
	}
	remapVolumePaths(app, "/tmp/powerlab-data")
	assert.Equal(t, app.Services[0].Volumes[0].Source, "/DATA/named",
		"named volumes (non-bind) must NOT be rewritten")
}

// Sanity check: codegen alias ComposeApp == types.Project so we can construct one.
var _ = codegen.ComposeApp{}
