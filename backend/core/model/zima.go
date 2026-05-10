package model

import "time"

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
