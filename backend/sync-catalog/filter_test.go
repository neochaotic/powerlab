package main

import (
	"strings"
	"testing"
)

// knownUmbrelApps is the catalog list the filter consults when
// disambiguating "same-compose sibling" (allow) from "cross-app
// sibling" (Tier 1 reject). Real syncs derive this from the
// upstream repo's directory listing; the test uses a static list
// covering the apps referenced in the fixtures.
var knownUmbrelApps = map[string]bool{
	"bitcoin": true, "lightning": true, "electrs": true,
	"nginx-proxy-manager": true, "nextcloud": true, "jellyfin": true,
	"immich": true, "plex": true, "home-assistant": true,
	"adguard-home": true,
}

// TestFilter_Tier1_HardReject_KnownCryptoApps locks the load-bearing
// rule: every app that uses a `getumbrel/*` image OR pulls env vars
// from a sibling Umbrel app MUST end up in TierHardReject. These three
// are the canonical Tier 1 examples called out in ADR-0024 and the
// Phase 0 audit (`docs/audits/catalog-overlap-2026-05-11.md`).
func TestFilter_Tier1_HardReject_KnownCryptoApps(t *testing.T) {
	f := &Filter{KnownAppIDs: knownUmbrelApps}

	cases := []struct {
		name     string
		appID    string
		compose  ComposeFile
		expected FilterTier
		matchRsn string // substring expected in result.Reason
	}{
		{
			name:  "bitcoin uses ghcr.io/getumbrel/* image",
			appID: "bitcoin",
			compose: ComposeFile{
				Services: map[string]Service{
					"bitcoin": {
						Image: "ghcr.io/getumbrel/umbrel-bitcoin:v1.2.2@sha256:bc72c7fe",
					},
				},
			},
			expected: TierHardReject,
			matchRsn: "getumbrel/",
		},
		{
			name:  "lightning uses getumbrel/* image + cross-app sibling env vars",
			appID: "lightning",
			compose: ComposeFile{
				Services: map[string]Service{
					"lightning": {
						Image: "getumbrel/umbrel-lightning:v1.2.2",
						EnvironmentRaw: map[string]any{
							"BITCOIN_HOST": "${APP_BITCOIN_NODE_IP}",
							"RPC_PORT":     "${APP_BITCOIN_RPC_PORT}",
						},
					},
				},
			},
			expected: TierHardReject,
			matchRsn: "getumbrel/",
		},
		{
			name:  "electrs uses getumbrel/* image + multiple cross-app siblings",
			appID: "electrs",
			compose: ComposeFile{
				Services: map[string]Service{
					"electrs": {
						Image: "getumbrel/umbrel-electrs:v1.0.4",
						EnvironmentRaw: []string{
							"BITCOIN_HOST=${APP_BITCOIN_NODE_IP}",
							"RPC_USER=${APP_BITCOIN_RPC_USER}",
						},
					},
				},
			},
			expected: TierHardReject,
			matchRsn: "getumbrel/",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := f.Apply(tc.appID, UmbrelManifest{ID: tc.appID}, tc.compose)
			if got.Tier != tc.expected {
				t.Errorf("expected %s, got %s (reason: %q)", tc.expected, got.Tier, got.Reason)
			}
			if !strings.Contains(got.Reason, tc.matchRsn) {
				t.Errorf("expected reason to contain %q, got %q", tc.matchRsn, got.Reason)
			}
		})
	}
}

// TestFilter_Tier1_CrossAppSibling_WithoutGetumbrelImage covers the
// "lightning-like app on a non-getumbrel image" edge case — even if
// the image is generic, depending on a sibling Umbrel app (via env
// var `${APP_<OTHER>_*}` where OTHER is in our known catalog) is a
// hard-reject signal because the dep cannot be satisfied on PowerLab
// without the sibling app being installed.
func TestFilter_Tier1_CrossAppSibling_WithoutGetumbrelImage(t *testing.T) {
	f := &Filter{KnownAppIDs: knownUmbrelApps}

	compose := ComposeFile{
		Services: map[string]Service{
			"some-app": {
				Image: "generic/some-app:1.0",
				EnvironmentRaw: map[string]any{
					"BITCOIN_RPC": "${APP_BITCOIN_RPC_PORT}",
				},
			},
		},
	}

	got := f.Apply("some-app", UmbrelManifest{ID: "some-app"}, compose)
	if got.Tier != TierHardReject {
		t.Errorf("cross-app sibling env should hard-reject; got %s (reason: %q)", got.Tier, got.Reason)
	}
	if !strings.Contains(got.Reason, "sibling") {
		t.Errorf("expected reason to mention sibling, got %q", got.Reason)
	}
}

// TestFilter_SameComposeSibling_Allowed is the filter-spec refinement
// flagged in the Phase 0 audit. A compose where service A depends on
// service B IN THE SAME compose project (e.g. nextcloud's web service
// depending on its internal mariadb) is NOT a cross-app sibling and
// must NOT be hard-rejected.
func TestFilter_SameComposeSibling_Allowed(t *testing.T) {
	f := &Filter{KnownAppIDs: knownUmbrelApps}

	compose := ComposeFile{
		Services: map[string]Service{
			"nextcloud": {
				Image:     "nextcloud:33.0.3-apache",
				DependsOn: []string{"db", "redis"},
			},
			"db": {
				Image: "mariadb:10.6.21",
			},
			"redis": {
				Image: "redis:6.2.2",
			},
		},
	}

	got := f.Apply("nextcloud", UmbrelManifest{ID: "nextcloud"}, compose)
	if got.Tier != TierAllow {
		t.Errorf("same-compose siblings must allow; got %s (reason: %q)", got.Tier, got.Reason)
	}
}

// TestFilter_Tier4_Allow_CommonApps locks the seven apps the Phase 0
// audit sampled. If any of these regresses to a non-Allow tier, the
// filter is too aggressive and the catalog import will drop apps
// users expect to find.
func TestFilter_Tier4_Allow_CommonApps(t *testing.T) {
	f := &Filter{KnownAppIDs: knownUmbrelApps}

	cases := []struct {
		appID   string
		image   string
		envVars map[string]any
	}{
		{"nginx-proxy-manager", "jc21/nginx-proxy-manager:2.14.0", nil},
		{"jellyfin", "linuxserver/jellyfin:10.11.8", nil},
		{"immich", "ghcr.io/immich-app/immich-server:v2.7.5", nil},
		{"plex", "linuxserver/plex:1.43.1", nil},
		{"home-assistant", "homeassistant/home-assistant:2026.4.4", nil},
		{"adguard-home", "adguard/adguardhome:v0.107.74", nil},
	}

	for _, tc := range cases {
		t.Run(tc.appID, func(t *testing.T) {
			compose := ComposeFile{
				Services: map[string]Service{
					tc.appID: {Image: tc.image, EnvironmentRaw: tc.envVars},
				},
			}
			got := f.Apply(tc.appID, UmbrelManifest{ID: tc.appID, Category: "Networking"}, compose)
			if got.Tier != TierAllow {
				t.Errorf("expected allow for %s, got %s (reason: %q)", tc.appID, got.Tier, got.Reason)
			}
		})
	}
}

// TestFilter_Tier2_BitcoinCategory_SoftReject covers the Tier 2
// soft-reject by category. Even apps with clean images get blocked
// if their category is on the default-deny list (Bitcoin / Lightning /
// Bitcoin Node) — operators opt back in via the AllowedCategories
// config.
func TestFilter_Tier2_BitcoinCategory_SoftReject(t *testing.T) {
	f := &Filter{KnownAppIDs: knownUmbrelApps}

	compose := ComposeFile{
		Services: map[string]Service{
			"some-wallet": {Image: "generic/wallet:1.0"},
		},
	}

	got := f.Apply("some-wallet", UmbrelManifest{ID: "some-wallet", Category: "Bitcoin"}, compose)
	if got.Tier != TierSoftReject {
		t.Errorf("Bitcoin category must soft-reject by default; got %s (reason: %q)", got.Tier, got.Reason)
	}
	if !strings.Contains(strings.ToLower(got.Reason), "bitcoin") {
		t.Errorf("expected reason to mention Bitcoin, got %q", got.Reason)
	}
}

// TestFilter_Tier2_OptIn_AllowsBitcoinCategory verifies the operator
// escape hatch: when AllowedCategories contains "Bitcoin", a Bitcoin-
// categorised app that otherwise passes Tier 1 lands in Allow.
func TestFilter_Tier2_OptIn_AllowsBitcoinCategory(t *testing.T) {
	f := &Filter{KnownAppIDs: knownUmbrelApps, AllowedCategories: []string{"Bitcoin"}}

	compose := ComposeFile{
		Services: map[string]Service{
			"some-wallet": {Image: "generic/wallet:1.0"},
		},
	}

	got := f.Apply("some-wallet", UmbrelManifest{ID: "some-wallet", Category: "Bitcoin"}, compose)
	if got.Tier != TierAllow {
		t.Errorf("Bitcoin category with opt-in must allow; got %s (reason: %q)", got.Tier, got.Reason)
	}
}

// TestFilter_Tier1_GhcrPrefixed covers the variant where getumbrel
// is namespaced under ghcr.io rather than docker.io — both must
// hard-reject. Real fixture: ghcr.io/getumbrel/umbrel-bitcoin:v1.2.2
func TestFilter_Tier1_GhcrPrefixed(t *testing.T) {
	f := &Filter{KnownAppIDs: knownUmbrelApps}
	compose := ComposeFile{
		Services: map[string]Service{
			"x": {Image: "ghcr.io/getumbrel/whatever:v1.0"},
		},
	}
	got := f.Apply("x", UmbrelManifest{ID: "x"}, compose)
	if got.Tier != TierHardReject {
		t.Errorf("ghcr.io/getumbrel/* must hard-reject; got %s", got.Tier)
	}
}
