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
