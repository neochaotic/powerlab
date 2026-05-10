package model

import "time"

// StorageA is a registered storage backend (cloud, local, etc.) as
// the user sees it in the admin UI. Same shape as local-storage's
// StorageA — kept in sync across services. MountPath is the URL-
// prefix the storage is exposed at; Driver names the implementation
// (e.g. "Local", "AliyunDrive"); Addition is driver-specific JSON.
type StorageA struct {
	ID              uint      `json:"id" gorm:"primaryKey"`                        // unique key
	MountPath       string    `json:"mount_path" gorm:"unique" binding:"required"` // must be standardized
	Order           int       `json:"order"`                                       // use to sort
	Driver          string    `json:"driver"`                                      // driver used
	CacheExpiration int       `json:"cache_expiration"`                            // cache expire time
	Status          string    `json:"status"`
	Addition        string    `json:"addition" gorm:"type:text"` // Additional information, defined in the corresponding driver
	Remark          string    `json:"remark"`
	Modified        time.Time `json:"modified"`
	Disabled        bool      `json:"disabled"` // if disabled
	Sort
	Proxy
}

// Sort embeds into StorageA — controls listing order. ExtractFolder
// pins a folder to the top.
type Sort struct {
	OrderBy        string `json:"order_by"`
	OrderDirection string `json:"order_direction"`
	ExtractFolder  string `json:"extract_folder"`
}

// Proxy embeds into StorageA — controls how downloads + WebDAV
// requests are served. WebProxy true means stream through the
// service; false means redirect (302) to the storage's direct URL.
type Proxy struct {
	WebProxy     bool   `json:"web_proxy"`
	WebdavPolicy string `json:"webdav_policy"`
	DownProxyUrl string `json:"down_proxy_url"`
}

// GetStorage returns the receiver — implements an interface used by
// driver code that wraps StorageA in a heterogeneous collection.
func (s *StorageA) GetStorage() *StorageA {
	return s
}

// SetStorage replaces the receiver's value with storage.
func (s *StorageA) SetStorage(storage StorageA) {
	*s = storage
}

// SetStatus mutates the storage's lifecycle status string.
func (s *StorageA) SetStatus(status string) {
	s.Status = status
}

// Webdav302 reports whether WebDAV downloads should be served as
// HTTP 302 redirects to the underlying storage's direct URL.
func (p Proxy) Webdav302() bool {
	return p.WebdavPolicy == "302_redirect"
}

// WebdavProxy reports whether WebDAV downloads should be proxied via
// DownProxyUrl.
func (p Proxy) WebdavProxy() bool {
	return p.WebdavPolicy == "use_proxy_url"
}

// WebdavNative reports whether WebDAV downloads should be served
// directly by this service (no redirect, no proxy).
func (p Proxy) WebdavNative() bool {
	return !p.Webdav302() && !p.WebdavProxy()
}
