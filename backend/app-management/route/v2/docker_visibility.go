// Package v2 — raw Docker visibility (#630).
//
// Five read-only HTTP handlers expose daemon-wide state that goes beyond
// PowerLab's compose-app ownership: containers started by `docker run`
// outside PowerLab, third-party images, orphan volumes and networks,
// daemon version + disk-usage snapshot. The endpoints exist so MCP
// (powerlab-mcp) — and any future operator-facing tool — can answer
// "what's actually running on this host?" without growing its own
// Docker socket access. Per ADR-0045, app-management is the single
// PowerLab service that talks to the Docker daemon; MCP proxies its
// HTTP API.
//
// All five are READ ONLY by design. No exec, no shell-in-container, no
// container rm/prune — those need a separate threat-model ADR. The
// handlers wrap the existing Docker SDK client construction pattern
// already used by service/container.go (NewClientWithOpts + FromEnv +
// APIVersionNegotiation) so no new dependency is introduced.
package v2

import (
	"context"
	"net/http"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/api/types/volume"
	dockerclient "github.com/docker/docker/client"
	"github.com/labstack/echo/v4"
)

// dockerVisibilityTimeout caps each Docker daemon call. The daemon is
// local; the SDK negotiates the API version on first call (one extra
// roundtrip). 5 seconds is loose headroom — a healthy box answers in
// milliseconds; a hung daemon must not tie up MCP's request slot.
const dockerVisibilityTimeout = 5 * time.Second

// dockerVisibilityClient is the narrow interface the raw-visibility
// handlers depend on. Hand-typed so handler tests can pass a stub
// (no third-party mock library, no real Docker socket). The production
// implementation is a thin wrapper around the docker/docker/client.Client
// constructed via FromEnv (matching service/container.go).
type dockerVisibilityClient interface {
	ContainerList(ctx context.Context, opts container.ListOptions) ([]container.Summary, error)
	ImageList(ctx context.Context, opts image.ListOptions) ([]image.Summary, error)
	NetworkList(ctx context.Context, opts network.ListOptions) ([]network.Summary, error)
	VolumeList(ctx context.Context, opts volume.ListOptions) (volume.ListResponse, error)
	Info(ctx context.Context) (system.Info, error)
	DiskUsage(ctx context.Context, opts types.DiskUsageOptions) (types.DiskUsage, error)
	Close() error
}

// newDockerVisibilityClient is the indirection that lets tests swap
// in a stub without touching the production handler bodies. Production
// returns a real docker/docker client; tests assign a closure that
// returns a stub. Mirrors the pattern PowerLab uses for journal.Runner
// + similar runtime-overridable seams elsewhere in the codebase.
var newDockerVisibilityClient = func() (dockerVisibilityClient, error) {
	// FromEnv + APIVersionNegotiation matches every other Docker call
	// in the service package — DOCKER_HOST honoured, version handshake
	// avoids API-version pinning rot.
	return dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
}

// DockerVisibilityContainer is the JSON wire shape of one row in
// docker://containers. Hand-typed (not the raw SDK Summary) so the
// MCP contract is stable across Docker SDK upgrades — issue #630 names
// these fields explicitly. Names are flattened (no leading slash),
// labels passed through verbatim.
type DockerVisibilityContainer struct {
	ID        string                                `json:"id"`
	Name      string                                `json:"name"`
	Image     string                                `json:"image"`
	State     string                                `json:"state"`
	Status    string                                `json:"status"`
	Ports     []DockerVisibilityPort                `json:"ports"`
	CreatedAt int64                                 `json:"created_at"`
	Labels    map[string]string                     `json:"labels"`
}

// DockerVisibilityPort is the published-port view — host:container with
// protocol. Stable shape independent of the SDK's container.Port.
type DockerVisibilityPort struct {
	PrivatePort uint16 `json:"private_port"`
	PublicPort  uint16 `json:"public_port,omitempty"`
	Protocol    string `json:"protocol"`
	IP          string `json:"ip,omitempty"`
}

// DockerVisibilityContainersResponse is the docker://containers payload.
type DockerVisibilityContainersResponse struct {
	Containers []DockerVisibilityContainer `json:"containers"`
}

// DockerVisibilityImage is one row in docker://images.
type DockerVisibilityImage struct {
	ID        string   `json:"id"`
	Tags      []string `json:"tags"`
	Size      int64    `json:"size"`
	CreatedAt int64    `json:"created_at"`
}

// DockerVisibilityImagesResponse is the docker://images payload.
type DockerVisibilityImagesResponse struct {
	Images []DockerVisibilityImage `json:"images"`
}

// DockerVisibilityNetwork is one row in docker://networks. IPAM and
// attached_containers are exposed so the agent can answer "which
// network is jellyfin on?" without a second roundtrip.
type DockerVisibilityNetwork struct {
	ID                 string                          `json:"id"`
	Name               string                          `json:"name"`
	Driver             string                          `json:"driver"`
	Scope              string                          `json:"scope"`
	IPAM               DockerVisibilityIPAM            `json:"ipam"`
	AttachedContainers []DockerVisibilityNetworkMember `json:"attached_containers"`
}

// DockerVisibilityIPAM mirrors network.IPAM with a flat wire shape.
type DockerVisibilityIPAM struct {
	Driver  string                       `json:"driver"`
	Configs []DockerVisibilityIPAMConfig `json:"configs"`
}

// DockerVisibilityIPAMConfig is one subnet entry.
type DockerVisibilityIPAMConfig struct {
	Subnet  string `json:"subnet,omitempty"`
	Gateway string `json:"gateway,omitempty"`
}

// DockerVisibilityNetworkMember is one attached-container reference.
type DockerVisibilityNetworkMember struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	IPv4 string `json:"ipv4,omitempty"`
}

// DockerVisibilityNetworksResponse is the docker://networks payload.
type DockerVisibilityNetworksResponse struct {
	Networks []DockerVisibilityNetwork `json:"networks"`
}

// DockerVisibilityVolume is one row in docker://volumes. UsageData
// is opt-in on the daemon side; when absent size = -1 (SDK convention).
// InUseBy is best-effort: the daemon doesn't always report it on
// `volume ls`; the field stays present (empty slice) so the wire shape
// is stable.
type DockerVisibilityVolume struct {
	Name       string   `json:"name"`
	Driver     string   `json:"driver"`
	Mountpoint string   `json:"mountpoint"`
	Size       int64    `json:"size"`
	InUseBy    []string `json:"in_use_by"`
}

// DockerVisibilityVolumesResponse is the docker://volumes payload.
type DockerVisibilityVolumesResponse struct {
	Volumes []DockerVisibilityVolume `json:"volumes"`
}

// DockerVisibilityDiskUsage is the `docker system df` rollup —
// per-category total bytes. Matches the daemon's /system/df output but
// flat so a future change (e.g. SDK renaming a field) doesn't ripple
// to the MCP wire contract.
type DockerVisibilityDiskUsage struct {
	Containers int64 `json:"containers"`
	Images     int64 `json:"images"`
	Volumes    int64 `json:"volumes"`
	BuildCache int64 `json:"build_cache"`
}

// DockerVisibilitySystemResponse is the docker://system payload —
// daemon version + container/image count + disk usage rollup.
type DockerVisibilitySystemResponse struct {
	DockerVersion   string                    `json:"docker_version"`
	ContainersCount int                       `json:"containers_count"`
	ImagesCount     int                       `json:"images_count"`
	DiskUsage       DockerVisibilityDiskUsage `json:"disk_usage"`
}

// DockerContainers handles GET /v2/app_management/docker/containers.
// Returns every container on the daemon (PowerLab-managed AND
// non-PowerLab), equivalent to `docker ps -a`. Used by powerlab-mcp's
// docker://containers resource (#630).
func (a *AppManagement) DockerContainers(ctx echo.Context) error {
	cli, err := newDockerVisibilityClient()
	if err != nil {
		return ctx.JSON(http.StatusServiceUnavailable, dockerVisibilityError(err))
	}
	defer func() { _ = cli.Close() }()

	callCtx, cancel := context.WithTimeout(ctx.Request().Context(), dockerVisibilityTimeout)
	defer cancel()

	// All=true so stopped containers are included — the agent has to
	// see what's down, not just what's up.
	raw, err := cli.ContainerList(callCtx, container.ListOptions{All: true})
	if err != nil {
		return ctx.JSON(http.StatusServiceUnavailable, dockerVisibilityError(err))
	}

	out := DockerVisibilityContainersResponse{Containers: make([]DockerVisibilityContainer, 0, len(raw))}
	for _, c := range raw {
		out.Containers = append(out.Containers, toDockerVisibilityContainer(c))
	}
	return ctx.JSON(http.StatusOK, out)
}

// DockerImages handles GET /v2/app_management/docker/images.
// Returns every local image (one row per image ID; tags are listed
// as a slice because one image can carry multiple repo:tag refs).
// Used by powerlab-mcp's docker://images resource (#630).
func (a *AppManagement) DockerImages(ctx echo.Context) error {
	cli, err := newDockerVisibilityClient()
	if err != nil {
		return ctx.JSON(http.StatusServiceUnavailable, dockerVisibilityError(err))
	}
	defer func() { _ = cli.Close() }()

	callCtx, cancel := context.WithTimeout(ctx.Request().Context(), dockerVisibilityTimeout)
	defer cancel()

	raw, err := cli.ImageList(callCtx, image.ListOptions{All: false})
	if err != nil {
		return ctx.JSON(http.StatusServiceUnavailable, dockerVisibilityError(err))
	}

	out := DockerVisibilityImagesResponse{Images: make([]DockerVisibilityImage, 0, len(raw))}
	for _, im := range raw {
		out.Images = append(out.Images, DockerVisibilityImage{
			ID:        im.ID,
			Tags:      append([]string{}, im.RepoTags...),
			Size:      im.Size,
			CreatedAt: im.Created,
		})
	}
	return ctx.JSON(http.StatusOK, out)
}

// DockerNetworks handles GET /v2/app_management/docker/networks.
// Returns every network on the daemon. IPAM + attached_containers
// flattened so the agent gets the full picture in one read.
// Used by powerlab-mcp's docker://networks resource (#630).
func (a *AppManagement) DockerNetworks(ctx echo.Context) error {
	cli, err := newDockerVisibilityClient()
	if err != nil {
		return ctx.JSON(http.StatusServiceUnavailable, dockerVisibilityError(err))
	}
	defer func() { _ = cli.Close() }()

	callCtx, cancel := context.WithTimeout(ctx.Request().Context(), dockerVisibilityTimeout)
	defer cancel()

	raw, err := cli.NetworkList(callCtx, network.ListOptions{})
	if err != nil {
		return ctx.JSON(http.StatusServiceUnavailable, dockerVisibilityError(err))
	}

	out := DockerVisibilityNetworksResponse{Networks: make([]DockerVisibilityNetwork, 0, len(raw))}
	for _, n := range raw {
		out.Networks = append(out.Networks, toDockerVisibilityNetwork(n))
	}
	return ctx.JSON(http.StatusOK, out)
}

// DockerVolumes handles GET /v2/app_management/docker/volumes.
// Returns every volume; size + in_use_by are best-effort (the daemon's
// `volume ls` skips them by default for speed).
// Used by powerlab-mcp's docker://volumes resource (#630).
func (a *AppManagement) DockerVolumes(ctx echo.Context) error {
	cli, err := newDockerVisibilityClient()
	if err != nil {
		return ctx.JSON(http.StatusServiceUnavailable, dockerVisibilityError(err))
	}
	defer func() { _ = cli.Close() }()

	callCtx, cancel := context.WithTimeout(ctx.Request().Context(), dockerVisibilityTimeout)
	defer cancel()

	raw, err := cli.VolumeList(callCtx, volume.ListOptions{})
	if err != nil {
		return ctx.JSON(http.StatusServiceUnavailable, dockerVisibilityError(err))
	}

	out := DockerVisibilityVolumesResponse{Volumes: make([]DockerVisibilityVolume, 0, len(raw.Volumes))}
	for _, v := range raw.Volumes {
		if v == nil {
			continue
		}
		size := int64(-1)
		if v.UsageData != nil {
			size = v.UsageData.Size
		}
		out.Volumes = append(out.Volumes, DockerVisibilityVolume{
			Name:       v.Name,
			Driver:     v.Driver,
			Mountpoint: v.Mountpoint,
			Size:       size,
			InUseBy:    []string{}, // daemon doesn't expose this on list — placeholder for shape stability
		})
	}
	return ctx.JSON(http.StatusOK, out)
}

// DockerSystem handles GET /v2/app_management/docker/system. Returns
// daemon version + container/image count + a `docker system df` rollup
// (per-category disk usage). Used by powerlab-mcp's docker://system
// resource (#630).
func (a *AppManagement) DockerSystem(ctx echo.Context) error {
	cli, err := newDockerVisibilityClient()
	if err != nil {
		return ctx.JSON(http.StatusServiceUnavailable, dockerVisibilityError(err))
	}
	defer func() { _ = cli.Close() }()

	callCtx, cancel := context.WithTimeout(ctx.Request().Context(), dockerVisibilityTimeout)
	defer cancel()

	info, infoErr := cli.Info(callCtx)
	if infoErr != nil {
		return ctx.JSON(http.StatusServiceUnavailable, dockerVisibilityError(infoErr))
	}

	df, dfErr := cli.DiskUsage(callCtx, types.DiskUsageOptions{})
	if dfErr != nil {
		// Daemon up + Info fine, but df failed (older daemon, permission
		// quirk). Report what we have rather than 503-ing the whole call;
		// the agent can still see the version + counts.
		return ctx.JSON(http.StatusOK, DockerVisibilitySystemResponse{
			DockerVersion:   info.ServerVersion,
			ContainersCount: info.Containers,
			ImagesCount:     info.Images,
			DiskUsage:       DockerVisibilityDiskUsage{},
		})
	}

	return ctx.JSON(http.StatusOK, DockerVisibilitySystemResponse{
		DockerVersion:   info.ServerVersion,
		ContainersCount: info.Containers,
		ImagesCount:     info.Images,
		DiskUsage:       summariseDiskUsage(df),
	})
}

// toDockerVisibilityContainer projects the SDK's container.Summary
// onto the stable wire shape. Names is a slice in the SDK (one
// container can have multiple names); we strip the leading slash
// (Docker prepends "/" to every name) and pick the first, which
// matches the panel's display convention.
func toDockerVisibilityContainer(c container.Summary) DockerVisibilityContainer {
	name := ""
	if len(c.Names) > 0 {
		// Docker prepends "/" to every container name on the wire
		// — strip exactly one leading slash so test fixtures match
		// the panel's display.
		name = c.Names[0]
		if len(name) > 0 && name[0] == '/' {
			name = name[1:]
		}
	}
	ports := make([]DockerVisibilityPort, 0, len(c.Ports))
	for _, p := range c.Ports {
		ports = append(ports, DockerVisibilityPort{
			PrivatePort: p.PrivatePort,
			PublicPort:  p.PublicPort,
			Protocol:    p.Type,
			IP:          p.IP,
		})
	}
	labels := c.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	return DockerVisibilityContainer{
		ID:        c.ID,
		Name:      name,
		Image:     c.Image,
		State:     string(c.State),
		Status:    c.Status,
		Ports:     ports,
		CreatedAt: c.Created,
		Labels:    labels,
	}
}

// toDockerVisibilityNetwork projects network.Inspect/Summary onto the
// stable wire shape. attached_containers is derived from the
// Containers map (id → EndpointResource); empty when no containers
// are connected.
func toDockerVisibilityNetwork(n network.Summary) DockerVisibilityNetwork {
	configs := make([]DockerVisibilityIPAMConfig, 0, len(n.IPAM.Config))
	for _, cfg := range n.IPAM.Config {
		configs = append(configs, DockerVisibilityIPAMConfig{
			Subnet:  cfg.Subnet,
			Gateway: cfg.Gateway,
		})
	}
	members := make([]DockerVisibilityNetworkMember, 0, len(n.Containers))
	for id, ep := range n.Containers {
		members = append(members, DockerVisibilityNetworkMember{
			ID:   id,
			Name: ep.Name,
			IPv4: ep.IPv4Address,
		})
	}
	return DockerVisibilityNetwork{
		ID:     n.ID,
		Name:   n.Name,
		Driver: n.Driver,
		Scope:  n.Scope,
		IPAM: DockerVisibilityIPAM{
			Driver:  n.IPAM.Driver,
			Configs: configs,
		},
		AttachedContainers: members,
	}
}

// summariseDiskUsage rolls the SDK's per-category slices into total
// bytes — matching `docker system df`'s SUMMARY column. Image total
// uses LayersSize (the deduplicated layer total) when present; the
// per-image Size double-counts shared layers and would inflate the
// number.
func summariseDiskUsage(df types.DiskUsage) DockerVisibilityDiskUsage {
	var containersTotal int64
	for _, c := range df.Containers {
		if c != nil {
			containersTotal += c.SizeRw
		}
	}
	imagesTotal := df.LayersSize
	if imagesTotal == 0 {
		// Older daemon may not report LayersSize; fall back to summing
		// per-image VirtualSize (deprecated but populated everywhere).
		for _, im := range df.Images {
			if im != nil {
				imagesTotal += im.Size
			}
		}
	}
	var volumesTotal int64
	for _, v := range df.Volumes {
		if v != nil && v.UsageData != nil {
			volumesTotal += v.UsageData.Size
		}
	}
	var buildCacheTotal int64
	for _, b := range df.BuildCache {
		if b != nil {
			buildCacheTotal += b.Size
		}
	}
	return DockerVisibilityDiskUsage{
		Containers: containersTotal,
		Images:     imagesTotal,
		Volumes:    volumesTotal,
		BuildCache: buildCacheTotal,
	}
}

// dockerVisibilityError wraps a daemon-side failure into the existing
// error response shape so the upstream proxy (powerlab-mcp) sees the
// canonical detail string and pivots to audit/journal.
func dockerVisibilityError(err error) map[string]string {
	return map[string]string{
		"code":    "docker_unavailable",
		"message": err.Error(),
	}
}
