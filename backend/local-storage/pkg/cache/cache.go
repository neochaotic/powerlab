// Package cache wraps go-cache with the local-storage default
// expiry/cleanup tuning. The 5-minute default is matched to the
// LSBLK + smartctl read cadence — long enough to avoid hammering
// the kernel, short enough that the UI sees fresh state on a
// page reload.
package cache

import (
	"time"

	"github.com/patrickmn/go-cache"
)

// Init returns a fresh cache with the local-storage defaults
// (5-minute item TTL, 60-second background cleanup).
func Init() *cache.Cache {
	return cache.New(5*time.Minute, 60*time.Second)
}
