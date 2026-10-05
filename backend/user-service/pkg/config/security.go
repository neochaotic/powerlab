package config

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/neochaotic/powerlab/backend/common/utils/jwt"
	"github.com/neochaotic/powerlab/backend/user-service/model"
)

// Bounds for the [security] token lifetimes (#484). A value outside
// its range, or one that does not parse, falls back to the default.
const (
	MinAccessTokenTTL  = 15 * time.Minute
	MaxAccessTokenTTL  = 24 * time.Hour
	MinRefreshTokenTTL = 24 * time.Hour
	MaxRefreshTokenTTL = 30 * 24 * time.Hour

	// maxTTLDays caps the "Nd" form so the multiplication below can
	// never overflow time.Duration and wrap back into range.
	maxTTLDays = 100000
)

var (
	// SecurityInfo is the raw [security] section of user-service.conf.
	SecurityInfo = &model.SecurityModel{}

	// AccessTokenTTL and RefreshTokenTTL are the validated lifetimes
	// the login handler signs tokens with. They hold the defaults
	// until InitSetup resolves the config.
	AccessTokenTTL  = jwt.DefaultAccessTokenTTL
	RefreshTokenTTL = jwt.DefaultRefreshTokenTTL
)

// ParseTTL parses a token lifetime. It accepts Go duration syntax
// ("3h", "90m", "1h30m") plus a whole-day form ("7d").
func ParseTTL(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if days, ok := strings.CutSuffix(s, "d"); ok {
		n, err := strconv.Atoi(days)
		if err != nil || n < 0 || n > maxTTLDays {
			return 0, fmt.Errorf("invalid day duration %q", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

// ResolveTTL turns a raw config value into a lifetime within
// [lo, hi]. An empty value yields def with a nil error. A malformed or
// out-of-range value also yields def, with an error explaining why so
// the caller can warn.
func ResolveTTL(raw string, def, lo, hi time.Duration) (time.Duration, error) {
	if strings.TrimSpace(raw) == "" {
		return def, nil
	}
	ttl, err := ParseTTL(raw)
	if err != nil {
		return def, err
	}
	if ttl < lo || ttl > hi {
		return def, fmt.Errorf("%q is outside the allowed range %s..%s", raw, lo, hi)
	}
	return ttl, nil
}

// applySecurity resolves SecurityInfo into AccessTokenTTL and
// RefreshTokenTTL, logging a warning for each value it rejects.
func applySecurity() {
	AccessTokenTTL = resolveOrWarn("AccessTokenTTL", SecurityInfo.AccessTokenTTL,
		jwt.DefaultAccessTokenTTL, MinAccessTokenTTL, MaxAccessTokenTTL)
	RefreshTokenTTL = resolveOrWarn("RefreshTokenTTL", SecurityInfo.RefreshTokenTTL,
		jwt.DefaultRefreshTokenTTL, MinRefreshTokenTTL, MaxRefreshTokenTTL)
}

func resolveOrWarn(key, raw string, def, lo, hi time.Duration) time.Duration {
	ttl, err := ResolveTTL(raw, def, lo, hi)
	if err != nil {
		log.Printf("WARNING: [security] %s: %v; using default %s", key, err, def)
	}
	return ttl
}
