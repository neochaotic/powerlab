package v2

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/api/types/volume"
	"github.com/labstack/echo/v4"
)

// stubDockerVisibilityClient is the test fake for the
// dockerVisibilityClient interface. Each call returns the pre-staged
// result + error; nothing reaches a real Docker daemon. Concurrent
// callers are unsupported (each test stages, calls, checks).
type stubDockerVisibilityClient struct {
	containers     []container.Summary
	containersErr  error
	images         []image.Summary
	imagesErr      error
	networks       []network.Summary
	networksErr    error
	volumes        volume.ListResponse
	volumesErr     error
	info           system.Info
	infoErr        error
	diskUsage      types.DiskUsage
	diskUsageErr   error
	closed         bool
}

func (s *stubDockerVisibilityClient) ContainerList(_ context.Context, _ container.ListOptions) ([]container.Summary, error) {
	return s.containers, s.containersErr
}

func (s *stubDockerVisibilityClient) ImageList(_ context.Context, _ image.ListOptions) ([]image.Summary, error) {
	return s.images, s.imagesErr
}

func (s *stubDockerVisibilityClient) NetworkList(_ context.Context, _ network.ListOptions) ([]network.Summary, error) {
	return s.networks, s.networksErr
}

func (s *stubDockerVisibilityClient) VolumeList(_ context.Context, _ volume.ListOptions) (volume.ListResponse, error) {
	return s.volumes, s.volumesErr
}

func (s *stubDockerVisibilityClient) Info(_ context.Context) (system.Info, error) {
	return s.info, s.infoErr
}

func (s *stubDockerVisibilityClient) DiskUsage(_ context.Context, _ types.DiskUsageOptions) (types.DiskUsage, error) {
	return s.diskUsage, s.diskUsageErr
}

func (s *stubDockerVisibilityClient) Close() error {
	s.closed = true
	return nil
}

// withStubDockerClient swaps the package-level constructor so the
// handler under test reaches the stub. Returns a cleanup that restores
// the original constructor — call via defer so the test stays isolated.
func withStubDockerClient(t *testing.T, stub *stubDockerVisibilityClient) {
	t.Helper()
	orig := newDockerVisibilityClient
	newDockerVisibilityClient = func() (dockerVisibilityClient, error) {
		return stub, nil
	}
	t.Cleanup(func() {
		newDockerVisibilityClient = orig
	})
}

// invokeHandler is the route-test boilerplate: build an echo request,
// dispatch to the bound handler, return the response. Reused by every
// per-endpoint test so the shape is one line per call site.
func invokeHandler(t *testing.T, method, path string, h echo.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	e := echo.New()
	ctx := e.NewContext(req, rec)
	if err := h(ctx); err != nil {
		t.Fatalf("handler returned err: %v", err)
	}
	return rec
}

// DockerContainers must return the documented JSON shape — fields
// per #630: id, name, image, state, ports, created_at, labels. Names
// are stripped of the SDK's leading slash; labels are passed through.
func TestDockerContainers_ShapeContract(t *testing.T) {
	stub := &stubDockerVisibilityClient{
		containers: []container.Summary{
			{
				ID:      "abc123",
				Names:   []string{"/plex"},
				Image:   "plex/plex:latest",
				State:   "running",
				Status:  "Up 5 hours",
				Created: 1700000000,
				Ports: []container.Port{
					{PrivatePort: 32400, PublicPort: 32400, Type: "tcp", IP: "0.0.0.0"},
				},
				Labels: map[string]string{"foo": "bar"},
			},
		},
	}
	withStubDockerClient(t, stub)

	app := &AppManagement{}
	rec := invokeHandler(t, http.MethodGet, "/v2/app_management/docker/containers", app.DockerContainers)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s; want 200", rec.Code, rec.Body.String())
	}
	var resp DockerVisibilityContainersResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("body not JSON: %v\n%s", err, rec.Body.String())
	}
	if len(resp.Containers) != 1 {
		t.Fatalf("got %d containers; want 1", len(resp.Containers))
	}
	c := resp.Containers[0]
	if c.ID != "abc123" || c.Name != "plex" || c.Image != "plex/plex:latest" || c.State != "running" {
		t.Fatalf("container row mismatch: %+v", c)
	}
	if c.CreatedAt != 1700000000 {
		t.Fatalf("created_at=%d; want 1700000000", c.CreatedAt)
	}
	if len(c.Ports) != 1 || c.Ports[0].PrivatePort != 32400 || c.Ports[0].Protocol != "tcp" {
		t.Fatalf("ports mismatch: %+v", c.Ports)
	}
	if c.Labels["foo"] != "bar" {
		t.Fatalf("labels mismatch: %+v", c.Labels)
	}
	if !stub.closed {
		t.Fatalf("client.Close() was not called — handler leaks daemon sockets")
	}
}

// DockerContainers must return 503 with a docker_unavailable error when
// the daemon call fails — the upstream proxy (powerlab-mcp) pattern-
// matches on that shape to pivot to audit/journal.
func TestDockerContainers_DaemonFailure(t *testing.T) {
	withStubDockerClient(t, &stubDockerVisibilityClient{
		containersErr: errors.New("dial unix /var/run/docker.sock: no such file"),
	})

	app := &AppManagement{}
	rec := invokeHandler(t, http.MethodGet, "/v2/app_management/docker/containers", app.DockerContainers)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s; want 503", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if resp["code"] != "docker_unavailable" {
		t.Fatalf("code=%q; want docker_unavailable", resp["code"])
	}
}

// DockerImages must surface id, tags[], size, created_at per the
// documented shape — and accept multiple repo tags on one image
// (a single ID can carry many tags).
func TestDockerImages_ShapeContract(t *testing.T) {
	withStubDockerClient(t, &stubDockerVisibilityClient{
		images: []image.Summary{
			{
				ID:       "sha256:deadbeef",
				RepoTags: []string{"nginx:latest", "nginx:1.25"},
				Size:     12345678,
				Created:  1700000000,
			},
		},
	})

	app := &AppManagement{}
	rec := invokeHandler(t, http.MethodGet, "/v2/app_management/docker/images", app.DockerImages)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s; want 200", rec.Code, rec.Body.String())
	}
	var resp DockerVisibilityImagesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if len(resp.Images) != 1 {
		t.Fatalf("got %d images; want 1", len(resp.Images))
	}
	im := resp.Images[0]
	if im.ID != "sha256:deadbeef" || im.Size != 12345678 || im.CreatedAt != 1700000000 {
		t.Fatalf("image row mismatch: %+v", im)
	}
	if len(im.Tags) != 2 || im.Tags[0] != "nginx:latest" {
		t.Fatalf("tags mismatch: %+v", im.Tags)
	}
}

// DockerNetworks must flatten IPAM configs + attached containers
// into the documented shape — the agent reads ipam.configs[].subnet
// to know who owns 10.20.0.0/16, and attached_containers[].id to
// answer "which network is jellyfin on?".
func TestDockerNetworks_ShapeContract(t *testing.T) {
	withStubDockerClient(t, &stubDockerVisibilityClient{
		networks: []network.Summary{
			{
				ID:     "net-id-1",
				Name:   "bridge",
				Driver: "bridge",
				Scope:  "local",
				IPAM: network.IPAM{
					Driver: "default",
					Config: []network.IPAMConfig{
						{Subnet: "172.17.0.0/16", Gateway: "172.17.0.1"},
					},
				},
				Containers: map[string]network.EndpointResource{
					"abc123": {Name: "plex", IPv4Address: "172.17.0.2/16"},
				},
			},
		},
	})

	app := &AppManagement{}
	rec := invokeHandler(t, http.MethodGet, "/v2/app_management/docker/networks", app.DockerNetworks)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s; want 200", rec.Code, rec.Body.String())
	}
	var resp DockerVisibilityNetworksResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if len(resp.Networks) != 1 {
		t.Fatalf("got %d networks; want 1", len(resp.Networks))
	}
	n := resp.Networks[0]
	if n.Name != "bridge" || n.Driver != "bridge" || n.Scope != "local" {
		t.Fatalf("network row mismatch: %+v", n)
	}
	if len(n.IPAM.Configs) != 1 || n.IPAM.Configs[0].Subnet != "172.17.0.0/16" {
		t.Fatalf("ipam mismatch: %+v", n.IPAM)
	}
	if len(n.AttachedContainers) != 1 || n.AttachedContainers[0].Name != "plex" {
		t.Fatalf("attached_containers mismatch: %+v", n.AttachedContainers)
	}
}

// DockerVolumes must surface name/driver/mountpoint/size and an
// in_use_by slice. Size is enriched via the SDK's DiskUsage call
// (VolumeList alone never populates UsageData — bug class fix for
// #645). in_use_by is joined against ContainerList by walking each
// container's Mounts.
func TestDockerVolumes_ShapeContract(t *testing.T) {
	withStubDockerClient(t, &stubDockerVisibilityClient{
		volumes: volume.ListResponse{
			Volumes: []*volume.Volume{
				{
					Name:       "plex_data",
					Driver:     "local",
					Mountpoint: "/var/lib/docker/volumes/plex_data/_data",
					// UsageData intentionally nil — VolumeList alone
					// never populates it; the fix's DiskUsage call does.
				},
			},
		},
		diskUsage: types.DiskUsage{
			Volumes: []*volume.Volume{
				{Name: "plex_data", UsageData: &volume.UsageData{Size: 999000, RefCount: 1}},
			},
		},
		containers: []container.Summary{
			{
				ID:    "abc123def456ffff",
				Names: []string{"/plex"},
				Mounts: []container.MountPoint{
					{Type: mount.TypeVolume, Name: "plex_data", Destination: "/data"},
				},
			},
		},
	})

	app := &AppManagement{}
	rec := invokeHandler(t, http.MethodGet, "/v2/app_management/docker/volumes", app.DockerVolumes)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s; want 200", rec.Code, rec.Body.String())
	}
	var resp DockerVisibilityVolumesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if len(resp.Volumes) != 1 {
		t.Fatalf("got %d volumes; want 1", len(resp.Volumes))
	}
	v := resp.Volumes[0]
	if v.Name != "plex_data" || v.Driver != "local" || v.Mountpoint == "" {
		t.Fatalf("volume row mismatch: %+v", v)
	}
	if v.Size != 999000 {
		t.Fatalf("size=%d; want 999000 (DiskUsage-enriched)", v.Size)
	}
	if v.InUseBy == nil {
		t.Fatalf("in_use_by is nil — must be empty slice (shape stability)")
	}
	if len(v.InUseBy) != 1 {
		t.Fatalf("in_use_by len=%d; want 1 (joined from container Mounts)", len(v.InUseBy))
	}
	if v.InUseBy[0].ID != "abc123def456" {
		t.Fatalf("in_use_by[0].id=%q; want abc123def456 (12-char short id)", v.InUseBy[0].ID)
	}
	if v.InUseBy[0].Name != "plex" {
		t.Fatalf("in_use_by[0].name=%q; want plex", v.InUseBy[0].Name)
	}
}

// DockerVolumes — when DiskUsage fails (older daemon, permission
// quirk) the handler still returns 200 with size=-1 per row. Failing
// the whole call would hide the volume listing — the agent can still
// see the names.
func TestDockerVolumes_SizeMinusOneWhenDiskUsageFails(t *testing.T) {
	withStubDockerClient(t, &stubDockerVisibilityClient{
		volumes: volume.ListResponse{
			Volumes: []*volume.Volume{
				{
					Name:       "ephemeral",
					Driver:     "local",
					Mountpoint: "/var/lib/docker/volumes/ephemeral/_data",
				},
			},
		},
		diskUsageErr: errors.New("permission denied"),
	})

	app := &AppManagement{}
	rec := invokeHandler(t, http.MethodGet, "/v2/app_management/docker/volumes", app.DockerVolumes)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d; want 200 (DiskUsage failure must NOT 503 the whole call)", rec.Code)
	}
	var resp DockerVisibilityVolumesResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Volumes) != 1 || resp.Volumes[0].Size != -1 {
		t.Fatalf("size=%d; want -1 (DiskUsage failed)", resp.Volumes[0].Size)
	}
	if resp.Volumes[0].InUseBy == nil {
		t.Fatalf("in_use_by must be empty slice, not nil (shape stability)")
	}
}

// DockerVolumes — when a volume appears in DiskUsage but UsageData is
// nil for it (some drivers genuinely can't report size — NFS, smb),
// size stays -1 for that row; sibling volumes with UsageData are
// still enriched. No crash from the nil UsageData pointer.
func TestDockerVolumes_DegradesPerVolumeWhenUsageDataNil(t *testing.T) {
	withStubDockerClient(t, &stubDockerVisibilityClient{
		volumes: volume.ListResponse{
			Volumes: []*volume.Volume{
				{Name: "fat_volume", Driver: "local", Mountpoint: "/m1"},
				{Name: "nfs_volume", Driver: "nfs", Mountpoint: "/m2"},
			},
		},
		diskUsage: types.DiskUsage{
			Volumes: []*volume.Volume{
				{Name: "fat_volume", UsageData: &volume.UsageData{Size: 555}},
				{Name: "nfs_volume" /* UsageData nil — driver can't report */},
			},
		},
	})

	app := &AppManagement{}
	rec := invokeHandler(t, http.MethodGet, "/v2/app_management/docker/volumes", app.DockerVolumes)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d; want 200", rec.Code)
	}
	var resp DockerVisibilityVolumesResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Volumes) != 2 {
		t.Fatalf("got %d volumes; want 2", len(resp.Volumes))
	}
	sizeByName := map[string]int64{}
	for _, v := range resp.Volumes {
		sizeByName[v.Name] = v.Size
	}
	if sizeByName["fat_volume"] != 555 {
		t.Fatalf("fat_volume size=%d; want 555", sizeByName["fat_volume"])
	}
	if sizeByName["nfs_volume"] != -1 {
		t.Fatalf("nfs_volume size=%d; want -1 (driver did not populate UsageData)", sizeByName["nfs_volume"])
	}
}

// DockerVolumes — when ContainerList fails the handler still returns
// 200 with the volume listing + size enrichment; in_use_by is empty.
// The agent can still answer "which volume is fat" — only the
// "who's using it" join is lost. Failing the whole call would hide
// the disk-usage answer that motivated the fix (#645).
func TestDockerVolumes_ContainerListFailureLosesInUseByOnly(t *testing.T) {
	withStubDockerClient(t, &stubDockerVisibilityClient{
		volumes: volume.ListResponse{
			Volumes: []*volume.Volume{
				{Name: "data", Driver: "local", Mountpoint: "/m"},
			},
		},
		diskUsage: types.DiskUsage{
			Volumes: []*volume.Volume{
				{Name: "data", UsageData: &volume.UsageData{Size: 7777}},
			},
		},
		containersErr: errors.New("docker daemon hiccup"),
	})

	app := &AppManagement{}
	rec := invokeHandler(t, http.MethodGet, "/v2/app_management/docker/volumes", app.DockerVolumes)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d; want 200 (ContainerList failure must NOT 503)", rec.Code)
	}
	var resp DockerVisibilityVolumesResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Volumes) != 1 || resp.Volumes[0].Size != 7777 {
		t.Fatalf("size lost when ContainerList failed: %+v", resp.Volumes)
	}
	if resp.Volumes[0].InUseBy == nil || len(resp.Volumes[0].InUseBy) != 0 {
		t.Fatalf("in_use_by should be empty (not nil) when ContainerList fails: %+v", resp.Volumes[0].InUseBy)
	}
}

// DockerVolumes — bind mounts must NOT pollute in_use_by. A container
// can mount /host/path:/container/path (Type=bind) and the bind's
// MountPoint.Name is the source path, not a volume name — joining
// loosely on Name would attribute bind sources to fake volumes.
// Only Type=volume mounts count.
func TestDockerVolumes_BindMountsDoNotPolluteInUseBy(t *testing.T) {
	withStubDockerClient(t, &stubDockerVisibilityClient{
		volumes: volume.ListResponse{
			Volumes: []*volume.Volume{
				{Name: "real_volume", Driver: "local", Mountpoint: "/var/lib/docker/volumes/real_volume/_data"},
			},
		},
		containers: []container.Summary{
			{
				ID:    "container1xxx",
				Names: []string{"/binduser"},
				Mounts: []container.MountPoint{
					// A bind mount whose Name happens to equal a volume
					// name MUST NOT be joined — only Type=volume entries
					// are joinable per Docker's mount-point model.
					{Type: mount.TypeBind, Name: "real_volume", Source: "/host/real_volume", Destination: "/data"},
				},
			},
		},
	})

	app := &AppManagement{}
	rec := invokeHandler(t, http.MethodGet, "/v2/app_management/docker/volumes", app.DockerVolumes)

	var resp DockerVisibilityVolumesResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Volumes) != 1 {
		t.Fatalf("got %d volumes; want 1", len(resp.Volumes))
	}
	if len(resp.Volumes[0].InUseBy) != 0 {
		t.Fatalf("bind mount leaked into in_use_by: %+v", resp.Volumes[0].InUseBy)
	}
}

// DockerVolumes — when two containers mount the same volume, both
// rows show up in in_use_by (the common "shared volume" pattern —
// e.g. an app + its sidecar both mounting the data dir).
func TestDockerVolumes_MultipleContainersOnOneVolume(t *testing.T) {
	withStubDockerClient(t, &stubDockerVisibilityClient{
		volumes: volume.ListResponse{
			Volumes: []*volume.Volume{
				{Name: "shared", Driver: "local", Mountpoint: "/m"},
			},
		},
		containers: []container.Summary{
			{
				ID:    "aaaaaaaaaaaaffff",
				Names: []string{"/app"},
				Mounts: []container.MountPoint{
					{Type: mount.TypeVolume, Name: "shared"},
				},
			},
			{
				ID:    "bbbbbbbbbbbbffff",
				Names: []string{"/sidecar"},
				Mounts: []container.MountPoint{
					{Type: mount.TypeVolume, Name: "shared"},
				},
			},
		},
	})

	app := &AppManagement{}
	rec := invokeHandler(t, http.MethodGet, "/v2/app_management/docker/volumes", app.DockerVolumes)

	var resp DockerVisibilityVolumesResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Volumes) != 1 {
		t.Fatalf("got %d volumes; want 1", len(resp.Volumes))
	}
	if len(resp.Volumes[0].InUseBy) != 2 {
		t.Fatalf("in_use_by len=%d; want 2 (app + sidecar): %+v", len(resp.Volumes[0].InUseBy), resp.Volumes[0].InUseBy)
	}
	names := map[string]bool{}
	for _, u := range resp.Volumes[0].InUseBy {
		names[u.Name] = true
	}
	if !names["app"] || !names["sidecar"] {
		t.Fatalf("missing user(s): %+v", names)
	}
}

// DockerSystem must combine /info (version + counts) with /system/df
// (per-category disk usage rollup) into one payload — the agent reads
// docker_version to know what API surface to expect, and
// disk_usage.{containers,images,volumes,build_cache} to know when to
// suggest a prune.
func TestDockerSystem_ShapeContract(t *testing.T) {
	withStubDockerClient(t, &stubDockerVisibilityClient{
		info: system.Info{
			ServerVersion: "28.5.1",
			Containers:    3,
			Images:        12,
		},
		diskUsage: types.DiskUsage{
			LayersSize: 5000000000,
			Containers: []*container.Summary{
				{SizeRw: 1000},
				{SizeRw: 2000},
			},
			Volumes: []*volume.Volume{
				{UsageData: &volume.UsageData{Size: 500}},
			},
		},
	})

	app := &AppManagement{}
	rec := invokeHandler(t, http.MethodGet, "/v2/app_management/docker/system", app.DockerSystem)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s; want 200", rec.Code, rec.Body.String())
	}
	var resp DockerVisibilitySystemResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if resp.DockerVersion != "28.5.1" {
		t.Fatalf("docker_version=%q; want 28.5.1", resp.DockerVersion)
	}
	if resp.ContainersCount != 3 || resp.ImagesCount != 12 {
		t.Fatalf("counts mismatch: containers=%d images=%d", resp.ContainersCount, resp.ImagesCount)
	}
	if resp.DiskUsage.Images != 5000000000 {
		t.Fatalf("disk_usage.images=%d; want 5000000000 (LayersSize)", resp.DiskUsage.Images)
	}
	if resp.DiskUsage.Containers != 3000 {
		t.Fatalf("disk_usage.containers=%d; want 3000 (sum of SizeRw)", resp.DiskUsage.Containers)
	}
	if resp.DiskUsage.Volumes != 500 {
		t.Fatalf("disk_usage.volumes=%d; want 500", resp.DiskUsage.Volumes)
	}
}

// DockerSystem must still return 200 + best-effort body when /system/df
// fails (older daemon, permission quirk) — the agent gets version +
// counts even when df is broken. Failing the whole call would hide
// useful data.
func TestDockerSystem_DfFailureFallsBackToInfoOnly(t *testing.T) {
	withStubDockerClient(t, &stubDockerVisibilityClient{
		info: system.Info{ServerVersion: "28.5.1", Containers: 2, Images: 5},
		diskUsageErr: errors.New("permission denied"),
	})

	app := &AppManagement{}
	rec := invokeHandler(t, http.MethodGet, "/v2/app_management/docker/system", app.DockerSystem)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d; want 200 (df-only failure must NOT 503 the whole call)", rec.Code)
	}
	var resp DockerVisibilitySystemResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.DockerVersion != "28.5.1" || resp.ContainersCount != 2 {
		t.Fatalf("info portion lost: %+v", resp)
	}
	if resp.DiskUsage.Containers != 0 || resp.DiskUsage.Images != 0 {
		t.Fatalf("disk_usage should be zeroed when df fails: %+v", resp.DiskUsage)
	}
}

// DockerSystem — when /info itself fails, return 503 (no data
// recoverable; agent must pivot via the structured error).
func TestDockerSystem_InfoFailureReturns503(t *testing.T) {
	withStubDockerClient(t, &stubDockerVisibilityClient{
		infoErr: errors.New("cannot connect to docker daemon"),
	})

	app := &AppManagement{}
	rec := invokeHandler(t, http.MethodGet, "/v2/app_management/docker/system", app.DockerSystem)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d; want 503 (info failure = daemon unreachable)", rec.Code)
	}
}
