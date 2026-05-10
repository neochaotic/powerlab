// Package cache wraps go-cache with the core service's default
// expiry/cleanup tuning. Used for ephemeral state shared across
// route handlers.
package cache

import (
	"time"

	"github.com/patrickmn/go-cache"
)

// Init returns a fresh cache with core defaults (5-minute item
// TTL, 60-second background cleanup).
func Init() *cache.Cache {
	return cache.New(5*time.Minute, 60*time.Second)
}
