package model

import (
	"time"
)

// ServerAppListCollection is the curated app-store landing payload —
// three lists shown on the homepage tabs (full catalog, editor-
// curated recommendations, community submissions).
type ServerAppListCollection struct {
	List      []ServerAppList `json:"list"`
	Recommend []ServerAppList `json:"recommend"`
	Community []ServerAppList `json:"community"`
}

// StateEnum reports whether an app from the catalog is installed
// locally. Used on the catalog detail page to switch the CTA between
// "Install" and "Open".
type StateEnum int

// State values for ServerAppList.State.
const (
	StateEnumNotInstalled StateEnum = iota
	StateEnumInstalled
)

// @tiger - 对于用于出参的数据结构，静态信息（例如 title）和
//
//	动态信息（例如 state、query_count）应该划分到不同的数据结构中
//
//	这样的好处是
//	1 - 多次获取动态信息时可以减少出参复杂度，因为静态信息只获取一次就好
//	2 - 在未来的迭代中，可以降低维护成本（所有字段都展开放在一个层级维护成本略高）
//
//	另外，一些针对性字段，例如 Docker 相关的，可以用 map 来保存。
//	这样在未来增加多态 App，例如 Snap，不需要维护多个结构，或者一个结构保存不必要的字段
//
// ServerAppList is one app-catalog row — the joined view of the
// app's static metadata (title, icon, description, screenshots) and
// the dynamic per-instance state (State, QueryCount). The original
// design comments (in Chinese) flag that this should be split into
// static + dynamic structs in a future refactor.
type ServerAppList struct {
	ID             uint      `gorm:"column:id;primary_key" json:"id"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	Tagline        string    `json:"tagline"`
	Tags           Strings   `gorm:"type:json" json:"tags"`
	Icon           string    `json:"icon"`
	ScreenshotLink Strings   `gorm:"type:json" json:"screenshot_link"`
	Category       string    `json:"category"`
	CategoryID     int       `json:"category_id"`
	CategoryFont   string    `json:"category_font"`
	PortMap        string    `json:"port_map"`
	ImageVersion   string    `json:"image_version"`
	Tip            string    `json:"tip"`
	Envs           EnvArray  `json:"envs"`
	Ports          PortArray `json:"ports"`
	Volumes        PathArray `json:"volumes"`
	Devices        PathArray `json:"devices"`
	NetworkModel   string    `json:"network_model"`
	Image          string    `json:"image"`
	Index          string    `json:"index"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	State          StateEnum `json:"state"`
	Author         string    `json:"author"`
	MinMemory      int       `json:"min_memory"`
	MinDisk        int       `json:"min_disk"`
	Thumbnail      string    `json:"thumbnail"`
	Healthy        string    `json:"healthy"`
	Plugins        Strings   `json:"plugins"`
	Origin         string    `json:"origin"`
	Type           int       `json:"type"`
	QueryCount     int       `json:"query_count"`
	Developer      string    `json:"developer"`
	HostName       string    `json:"host_name"`
	Privileged     bool      `json:"privileged"`
	CapAdd         Strings   `json:"cap_add"`
	Cmd            Strings   `json:"cmd"`
	Architectures  Strings   `json:"architectures"`
	LatestDigest   Strings   `json:"latest_digests"`
}

// MyAppList is one row of the "Apps installed on this machine"
// list. Slimmer than ServerAppList — this is what the homepage
// tile grid binds to.
type MyAppList struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Icon           string `json:"icon"`
	State          string `json:"state"`
	CustomID       string `gorm:"column:custom_id;primary_key" json:"custom_id"`
	Index          string `json:"index"`
	Port           string `json:"port"`
	Slogan         string `json:"slogan"`
	Type           string `json:"type"`
	Image          string `json:"image"`
	Volumes        string `json:"volumes"`
	Latest         bool   `json:"latest"`
	Host           string `json:"host"`
	Protocol       string `json:"protocol"`
	Created        int64  `json:"created"`
	AppStoreID     uint   `json:"appstore_id"`
	IsUncontrolled bool   `json:"is_uncontrolled"`
}

// Ports is a port mapping used in the catalog detail JSON. The Type
// field is the field-classification enum: 1=required, 2=optional,
// 3=default-no-display, 4=system-handled, 5=container-side editable.
type Ports struct {
	ContainerPort uint   `json:"container_port"`
	CommendPort   int    `json:"commend_port"`
	Desc          string `json:"desc"`
	Type          int    `json:"type"` //  1:必选 2:可选 3:默认值不必显示 4:系统处理  5:container内容也可编辑
}

// Volume is a bind-mount declaration in the catalog detail JSON.
// Type uses the same field-classification enum as Ports.
type Volume struct {
	ContainerPath string `json:"container_path"`
	Path          string `json:"path"`
	Desc          string `json:"desc"`
	Type          int    `json:"type"` //  1:必选 2:可选 3:默认值不必显示 4:系统处理   5:container内容也可编辑
}

// Envs is one env-var declaration in the catalog detail JSON.
type Envs struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Desc  string `json:"desc"`
	Type  int    `json:"type"` //  1:必选 2:可选 3:默认值不必显示 4:系统处理 5:container内容也可编辑
}

// Devices is one device pass-through declaration in the catalog
// detail JSON.
type Devices struct {
	ContainerPath string `json:"container_path"`
	Path          string `json:"path"`
	Desc          string `json:"desc"`
	Type          int    `json:"type"` //  1:必选 2:可选 3:默认值不必显示 4:系统处理 5:container内容也可编辑
}

// Strings is a gorm-stored []string — used by JSON columns that
// hold a string array (tags, screenshot URLs, plugins, etc.).
type Strings []string

// MapStrings is a gorm-stored []map[string]string — used by JSON
// columns that hold an array of dictionaries.
type MapStrings []map[string]string
