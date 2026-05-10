package model

import "time"

// Path is one entry in a directory listing — name + full path,
// directory flag, mtime, size, type-hint, write-permission flag,
// plus a free-form Extensions map for driver-specific metadata.
type Path struct {
	Name       string                 `json:"name"`   //File name or document name
	Path       string                 `json:"path"`   //Full path to file or folder
	IsDir      bool                   `json:"is_dir"` //Is it a folder
	Date       time.Time              `json:"date"`
	Size       int64                  `json:"size"` //File Size
	Type       string                 `json:"type,omitempty"`
	Label      string                 `json:"label,omitempty"`
	Write      bool                   `json:"write"`
	Extensions map[string]interface{} `json:"extensions"`
}

// DeviceInfo is the public-facing device descriptor returned by the
// device discovery endpoint and surfaced on the login screen.
// LanIpv4 holds the addresses the device is reachable on; Hash is
// the device's stable per-install id.
type DeviceInfo struct {
	LanIpv4     []string `json:"lan_ipv4"`
	Port        int      `json:"port"`
	DeviceName  string   `json:"device_name"`
	DeviceModel string   `json:"device_model"`
	DeviceSN    string   `json:"device_sn"`
	Initialized bool     `json:"initialized"`
	OS_Version  string   `json:"os_version"`
	Hash        string   `json:"hash"`
}
