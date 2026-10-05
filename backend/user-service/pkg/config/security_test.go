package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/neochaotic/powerlab/backend/common/utils/jwt"
	"gopkg.in/ini.v1"
)

func TestParseTTL(t *testing.T) {
	tests := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{in: "3h", want: 3 * time.Hour},
		{in: "15m", want: 15 * time.Minute},
		{in: "1h30m", want: 90 * time.Minute},
		{in: " 8h ", want: 8 * time.Hour},
		{in: "1d", want: 24 * time.Hour},
		{in: "7d", want: 7 * 24 * time.Hour},
		{in: "30d", want: 30 * 24 * time.Hour},
		{in: "0d", want: 0},
		{in: "", wantErr: true},
		{in: "abc", wantErr: true},
		{in: "1.5d", wantErr: true},
		{in: "-1d", wantErr: true},
		{in: "d", wantErr: true},
		{in: "99999999999d", wantErr: true},
		{in: "3", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseTTL(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseTTL(%q) = %v, want error", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseTTL(%q) error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Fatalf("ParseTTL(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestResolveTTLBounds(t *testing.T) {
	access := func(raw string) (time.Duration, error) {
		return ResolveTTL(raw, jwt.DefaultAccessTokenTTL, MinAccessTokenTTL, MaxAccessTokenTTL)
	}
	refresh := func(raw string) (time.Duration, error) {
		return ResolveTTL(raw, jwt.DefaultRefreshTokenTTL, MinRefreshTokenTTL, MaxRefreshTokenTTL)
	}
	tests := []struct {
		name     string
		resolve  func(string) (time.Duration, error)
		raw      string
		want     time.Duration
		wantWarn bool
	}{
		{"access empty uses default", access, "", 3 * time.Hour, false},
		{"access lower bound", access, "15m", 15 * time.Minute, false},
		{"access upper bound", access, "24h", 24 * time.Hour, false},
		{"access in range", access, "8h", 8 * time.Hour, false},
		{"access below min", access, "14m", 3 * time.Hour, true},
		{"access above max", access, "25h", 3 * time.Hour, true},
		{"access 2d above max", access, "2d", 3 * time.Hour, true},
		{"access zero", access, "0s", 3 * time.Hour, true},
		{"access negative", access, "-1h", 3 * time.Hour, true},
		{"access garbage", access, "forever", 3 * time.Hour, true},
		{"refresh empty uses default", refresh, "", 7 * 24 * time.Hour, false},
		{"refresh lower bound", refresh, "1d", 24 * time.Hour, false},
		{"refresh lower bound hours", refresh, "24h", 24 * time.Hour, false},
		{"refresh upper bound", refresh, "30d", 30 * 24 * time.Hour, false},
		{"refresh in range", refresh, "14d", 14 * 24 * time.Hour, false},
		{"refresh below min", refresh, "23h", 7 * 24 * time.Hour, true},
		{"refresh above max", refresh, "31d", 7 * 24 * time.Hour, true},
		{"refresh zero days", refresh, "0d", 7 * 24 * time.Hour, true},
		{"refresh garbage", refresh, "week", 7 * 24 * time.Hour, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.resolve(tt.raw)
			if got != tt.want {
				t.Fatalf("ResolveTTL(%q) = %v, want %v", tt.raw, got, tt.want)
			}
			if (err != nil) != tt.wantWarn {
				t.Fatalf("ResolveTTL(%q) err = %v, wantWarn %v", tt.raw, err, tt.wantWarn)
			}
		})
	}
}

func TestInitSetupReadsSecuritySection(t *testing.T) {
	tests := []struct {
		name        string
		conf        string
		wantAccess  time.Duration
		wantRefresh time.Duration
	}{
		{
			name:        "no security section keeps defaults",
			conf:        "[common]\nRuntimePath=/tmp\n",
			wantAccess:  3 * time.Hour,
			wantRefresh: 7 * 24 * time.Hour,
		},
		{
			name:        "valid values are applied",
			conf:        "[security]\nAccessTokenTTL = 8h\nRefreshTokenTTL = 14d\n",
			wantAccess:  8 * time.Hour,
			wantRefresh: 14 * 24 * time.Hour,
		},
		{
			name:        "out of range values fall back",
			conf:        "[security]\nAccessTokenTTL = 0\nRefreshTokenTTL = 90d\n",
			wantAccess:  3 * time.Hour,
			wantRefresh: 7 * 24 * time.Hour,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origCfg, origPath, origSec := Cfg, ConfigFilePath, SecurityInfo
			t.Cleanup(func() {
				Cfg, ConfigFilePath, SecurityInfo = origCfg, origPath, origSec
				AccessTokenTTL, RefreshTokenTTL = jwt.DefaultAccessTokenTTL, jwt.DefaultRefreshTokenTTL
			})

			path := filepath.Join(t.TempDir(), "user-service.conf")
			if err := os.WriteFile(path, []byte(tt.conf), 0o600); err != nil {
				t.Fatalf("write conf: %v", err)
			}
			InitSetup(path, "")

			if AccessTokenTTL != tt.wantAccess {
				t.Errorf("AccessTokenTTL = %v, want %v", AccessTokenTTL, tt.wantAccess)
			}
			if RefreshTokenTTL != tt.wantRefresh {
				t.Errorf("RefreshTokenTTL = %v, want %v", RefreshTokenTTL, tt.wantRefresh)
			}
		})
	}
}

// The sample config ships the [security] keys commented out, so a
// fresh install keeps the code defaults.
func TestSampleConfigKeepsDefaultTTLs(t *testing.T) {
	cfg, err := ini.Load(filepath.Join("..", "..", "build", "sysroot", "etc", "powerlab", "user-service.conf.sample"))
	if err != nil {
		t.Fatalf("load sample: %v", err)
	}
	sec := cfg.Section("security")
	if sec.HasKey("AccessTokenTTL") || sec.HasKey("RefreshTokenTTL") {
		t.Fatalf("sample config sets token TTLs; defaults should stay in code")
	}
}
