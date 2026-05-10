package model

// TCPPorts and UDPPorts are the legacy single-port descriptors used
// by the V1 app-info JSON. Newer code uses PortMap which carries the
// protocol inline.
type TCPPorts struct {
	Desc          string `json:"desc"`
	ContainerPort int    `json:"container_port"`
}

// UDPPorts — legacy UDP equivalent of TCPPorts.
type UDPPorts struct {
	Desc          string `json:"desc"`
	ContainerPort int    `json:"container_port"`
}

// PortMap is a single host:container port pair as configured in the
// app-install form. CommendPort is the host-side port (the typo is
// historical and load-bearing — wire format).
type PortMap struct {
	ContainerPort string `json:"container"`
	CommendPort   string `json:"host"`
	Protocol      string `json:"protocol"`
	Desc          string `json:"desc"`
	Type          int    `json:"type"`
}

// PortArray is the gorm-JSON-stored list of PortMaps for an app.
type PortArray []PortMap

// Env is a single ENV-VAR mapping for the container's env block.
type Env struct {
	Name  string `json:"container"`
	Value string `json:"host"`
	Desc  string `json:"desc"`
	Type  int    `json:"type"`
}

// EnvArray is the gorm-JSON-stored env-var list for an app.
type EnvArray []Env

// PathMap is a single host:container bind-mount declared in an app
// manifest. Path is the host path (resolved relative to APPModel
// .StoragePath); ContainerPath is where it shows up inside the
// container.
type PathMap struct {
	ContainerPath string `json:"container"`
	Path          string `json:"host"`
	Type          int    `json:"type"`
	Desc          string `json:"desc"`
}

// PathArray is the gorm-JSON-stored bind-mount list for an app.
type PathArray []PathMap

/************************************************************************/

//type PostData struct {
//	Envs       EnvArrey  `json:"envs,omitempty"`
//	Udp        PortArrey `json:"udp_ports"`
//	Tcp        PortArrey `json:"tcp_ports"`
//	Volumes    PathArrey `json:"volumes"`
//	Devices    PathArrey `json:"devices"`
//	Port       string    `json:"port,omitempty"`
//	PortMap    string    `json:"port_map"`
//	CpuShares  int64     `json:"cpu_shares,omitempty"`
//	Memory     int64     `json:"memory,omitempty"`
//	Restart    string    `json:"restart,omitempty"`
//	EnableUPNP bool      `json:"enable_upnp"`
//	Label      string    `json:"label"`
//	Position   bool      `json:"position"`
//}

// CustomizationPostData is the body of the V1 install-from-form
// endpoint — every field a user can tweak from the "Install
// custom container" UI before the compose file is generated.
// Field types here must match what the codegen V2 install path
// produces, since both flows feed the same docker-compose
// renderer.
type CustomizationPostData struct {
	ContainerName string    `json:"container_name"`
	CustomID      string    `json:"custom_id"`
	Origin        string    `json:"origin"`
	NetworkModel  string    `json:"network_model"`
	Index         string    `json:"index"`
	Icon          string    `json:"icon"`
	Image         string    `json:"image"`
	Envs          EnvArray  `json:"envs"`
	Ports         PortArray `json:"ports"`
	Volumes       PathArray `json:"volumes"`
	Devices       PathArray `json:"devices"`
	// Port         string    `json:"port,omitempty"`
	PortMap     string   `json:"port_map"`
	CPUShares   int64    `json:"cpu_shares"`
	Memory      int64    `json:"memory"`
	Restart     string   `json:"restart"`
	EnableUPNP  bool     `json:"enable_upnp"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Position    bool     `json:"position"`
	HostName    string   `json:"host_name"`
	Privileged  bool     `json:"privileged"`
	CapAdd      []string `json:"cap_add"`
	Cmd         []string `json:"cmd"`
	Protocol    string   `json:"protocol"`
	Host        string   `json:"host"`
	AppStoreID  uint     `json:"appstore_id"`
}
