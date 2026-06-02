package composevalidator

import (
	"strings"
	"testing"
)

// A valid PowerLab-style compose passes cleanly. This is the only
// happy-path case — every other test exercises a rejection. We pick
// a realistic shape (Plex) using /DATA paths that PowerLab manages.
func TestValidate_AcceptsSafePowerLabCompose(t *testing.T) {
	yamlBody := `
services:
  plex:
    image: lscr.io/linuxserver/plex:latest
    container_name: plex
    environment:
      - PUID=1000
      - PGID=1000
      - VERSION=docker
    volumes:
      - /DATA/PowerLabAppData/plex/config:/config
      - /DATA/Media:/media
    ports:
      - "32400:32400"
    restart: unless-stopped
`
	got := Validate([]byte(yamlBody))
	if !got.OK {
		t.Fatalf("safe compose rejected: %+v", got.Violations)
	}
}

// One negative test per ADR-0046 §4 forbidden pattern. Table-driven
// so a regression that flips one rule off is loud + immediate.
func TestValidate_RejectsForbiddenPatterns(t *testing.T) {
	cases := []struct {
		name           string
		yaml           string
		wantCode       string
		wantInDetail   string
		wantSvc        string
	}{
		// container escape
		{
			name: "privileged true",
			yaml: `services:
  app:
    image: x
    privileged: true`,
			wantCode: "privileged_true",
			wantInDetail: "container escape",
			wantSvc: "app",
		},
		// docker socket abuse
		{
			name: "/var/run/docker.sock bind (short form)",
			yaml: `services:
  app:
    image: x
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock`,
			wantCode: "forbidden_volume_source",
			wantInDetail: "control of the host",
			wantSvc: "app",
		},
		{
			name: "/var/run/docker.sock bind (long form)",
			yaml: `services:
  app:
    image: x
    volumes:
      - type: bind
        source: /var/run/docker.sock
        target: /var/run/docker.sock`,
			wantCode: "forbidden_volume_source",
			wantInDetail: "control of the host",
			wantSvc: "app",
		},
		// host namespace sharing
		{
			name: "network_mode host",
			yaml: `services:
  app:
    image: x
    network_mode: host`,
			wantCode: "host_namespace_share",
			wantInDetail: "network_mode",
			wantSvc: "app",
		},
		{
			name: "network_mode container:<id>",
			yaml: `services:
  app:
    image: x
    network_mode: container:abc123`,
			wantCode: "host_namespace_share",
			wantInDetail: "container:abc123",
			wantSvc: "app",
		},
		{
			name: "pid host",
			yaml: `services:
  app:
    image: x
    pid: host`,
			wantCode: "host_namespace_share",
			wantInDetail: "pid",
			wantSvc: "app",
		},
		{
			name: "ipc host",
			yaml: `services:
  app:
    image: x
    ipc: host`,
			wantCode: "host_namespace_share",
			wantInDetail: "ipc",
			wantSvc: "app",
		},
		{
			name: "uts host",
			yaml: `services:
  app:
    image: x
    uts: host`,
			wantCode: "host_namespace_share",
			wantInDetail: "uts",
			wantSvc: "app",
		},
		{
			name: "userns_mode host",
			yaml: `services:
  app:
    image: x
    userns_mode: host`,
			wantCode: "host_namespace_share",
			wantInDetail: "userns_mode",
			wantSvc: "app",
		},
		// dangerous capabilities
		{
			name: "cap_add SYS_ADMIN",
			yaml: `services:
  app:
    image: x
    cap_add:
      - SYS_ADMIN`,
			wantCode: "dangerous_cap_add",
			wantInDetail: "SYS_ADMIN",
			wantSvc: "app",
		},
		{
			name: "cap_add net_admin (case-insensitive)",
			yaml: `services:
  app:
    image: x
    cap_add:
      - net_admin`,
			wantCode: "dangerous_cap_add",
			wantInDetail: "net_admin",
			wantSvc: "app",
		},
		{
			name: "cap_add ALL",
			yaml: `services:
  app:
    image: x
    cap_add:
      - ALL`,
			wantCode: "dangerous_cap_add",
			wantInDetail: "ALL",
			wantSvc: "app",
		},
		{
			name: "cap_add CAP_SYS_ADMIN (CAP_ prefix tolerated)",
			yaml: `services:
  app:
    image: x
    cap_add:
      - CAP_SYS_ADMIN`,
			wantCode: "dangerous_cap_add",
			wantInDetail: "CAP_SYS_ADMIN",
			wantSvc: "app",
		},
		// raw device passthrough
		{
			name: "devices block present",
			yaml: `services:
  app:
    image: x
    devices:
      - /dev/nvidia0:/dev/nvidia0`,
			wantCode: "devices_block",
			wantInDetail: "passthrough",
			wantSvc: "app",
		},
		// sensitive host paths
		{
			name: "/etc bind",
			yaml: `services:
  app:
    image: x
    volumes:
      - /etc:/etc:ro`,
			wantCode: "forbidden_volume_source",
			wantInDetail: "system configuration",
			wantSvc: "app",
		},
		{
			name: "/root bind",
			yaml: `services:
  app:
    image: x
    volumes:
      - /root:/host-root`,
			wantCode: "forbidden_volume_source",
			wantInDetail: "root home",
			wantSvc: "app",
		},
		{
			name: "/proc bind",
			yaml: `services:
  app:
    image: x
    volumes:
      - /proc:/host/proc`,
			wantCode: "forbidden_volume_source",
			wantInDetail: "kernel pseudo-filesystem",
			wantSvc: "app",
		},
		{
			name: "/sys bind",
			yaml: `services:
  app:
    image: x
    volumes:
      - /sys:/host/sys:ro`,
			wantCode: "forbidden_volume_source",
			wantInDetail: "kernel pseudo-filesystem",
			wantSvc: "app",
		},
		{
			name: "/var/lib bind",
			yaml: `services:
  app:
    image: x
    volumes:
      - /var/lib/docker:/data`,
			wantCode: "forbidden_volume_source",
			wantInDetail: "persistent state",
			wantSvc: "app",
		},
		{
			name: "/dev bind subdir",
			yaml: `services:
  app:
    image: x
    volumes:
      - /dev/snd:/dev/snd`,
			wantCode: "forbidden_volume_source",
			wantInDetail: "host hardware",
			wantSvc: "app",
		},
		// per-service rejection lands on the right service name
		{
			name: "multi-service — only the offender is flagged",
			yaml: `services:
  good:
    image: ok
    volumes:
      - /DATA/PowerLabAppData/good/c:/config
  bad:
    image: not-ok
    privileged: true`,
			wantCode: "privileged_true",
			wantInDetail: "privileged",
			wantSvc: "bad",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Validate([]byte(tc.yaml))
			if got.OK {
				t.Fatalf("expected rejection for %s; got OK=true", tc.name)
			}
			if !hasViolationFor(got.Violations, tc.wantCode, tc.wantSvc) {
				t.Fatalf("missing violation %s on service %q (got: %+v)", tc.wantCode, tc.wantSvc, got.Violations)
			}
			if tc.wantInDetail != "" {
				if !violationDetailContains(got.Violations, tc.wantCode, tc.wantInDetail) {
					t.Fatalf("violation %s detail missing %q (got: %+v)", tc.wantCode, tc.wantInDetail, got.Violations)
				}
			}
		})
	}
}

// Invalid YAML must produce a single invalid_yaml violation rather
// than a panic or silent pass — better to fail loudly than try a
// partial parse.
func TestValidate_RejectsInvalidYAML(t *testing.T) {
	got := Validate([]byte("services: { app:\n  image: x"))
	if got.OK {
		t.Fatalf("invalid YAML accepted: %+v", got)
	}
	if len(got.Violations) != 1 || got.Violations[0].Code != "invalid_yaml" {
		t.Fatalf("expected exactly one invalid_yaml violation; got: %+v", got.Violations)
	}
}

// A document with no `services:` key (e.g. only `networks:` or just
// version) must reject — there's nothing for app-management to
// install. Prevents the validator from accepting "empty" compose
// files an attacker could use to probe install_app's downstream
// failure modes.
func TestValidate_RejectsDocumentWithoutServices(t *testing.T) {
	got := Validate([]byte(`networks:
  default:
    driver: bridge`))
	if got.OK {
		t.Fatalf("service-less doc accepted: %+v", got)
	}
	if got.Violations[0].Code != "no_services" {
		t.Fatalf("expected no_services; got %+v", got.Violations)
	}
}

// Multiple violations on one service all surface — we don't stop at
// the first finding. The agent + the operator see the full story.
func TestValidate_ReportsMultipleViolations(t *testing.T) {
	got := Validate([]byte(`services:
  evil:
    image: x
    privileged: true
    network_mode: host
    cap_add:
      - SYS_ADMIN`))
	if got.OK {
		t.Fatalf("compound-violation compose accepted")
	}
	codes := map[string]bool{}
	for _, v := range got.Violations {
		codes[v.Code] = true
	}
	for _, want := range []string{"privileged_true", "host_namespace_share", "dangerous_cap_add"} {
		if !codes[want] {
			t.Fatalf("expected violation %s in compound result (got codes %v)", want, codes)
		}
	}
}

func hasViolationFor(vs []Violation, code, svc string) bool {
	for _, v := range vs {
		if v.Code == code && (svc == "" || v.Service == svc) {
			return true
		}
	}
	return false
}

func violationDetailContains(vs []Violation, code, substr string) bool {
	for _, v := range vs {
		if v.Code == code && strings.Contains(v.Detail, substr) {
			return true
		}
	}
	return false
}

// REGRESSION (Batch B prompt #2, 2026-06-02): the catalog ships
// jellyfin/emby/*-nvidia with `devices: [/dev/dri:/dev/dri]` for
// hardware video transcode. Pre-fix, the validator rejected ANY
// non-empty devices block, so the agent install path couldn't ship
// catalog apps the UI install path accepts. Allowlist /dev/dri
// (Intel/AMD/PI render-node + card devices) since it's the
// widely-accepted safe-passthrough surface. All other /dev/* paths
// stay rejected.
func TestValidate_AllowsDriDevicePassthrough(t *testing.T) {
	cases := []struct {
		name string
		yaml string
	}{
		{
			"renderD128 explicit",
			"services:\n  app:\n    image: x\n    devices:\n      - /dev/dri/renderD128:/dev/dri/renderD128\n",
		},
		{
			"card0 explicit",
			"services:\n  app:\n    image: x\n    devices:\n      - /dev/dri/card0:/dev/dri/card0\n",
		},
		{
			"dri whole tree (jellyfin catalog form)",
			"services:\n  app:\n    image: x\n    devices:\n      - /dev/dri:/dev/dri\n",
		},
		{
			"dri short form (no colon)",
			"services:\n  app:\n    image: x\n    devices:\n      - /dev/dri\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := Validate([]byte(tc.yaml))
			for _, v := range res.Violations {
				if v.Code == "devices_block" {
					t.Errorf("expected /dev/dri allowlist to pass; got devices_block %q", v.Detail)
				}
			}
		})
	}
}

// Anything not in the allowlist MUST still be rejected.
func TestValidate_RejectsNonAllowlistedDevices(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{"nvidia raw", "/dev/nvidia0"},
		{"kmem", "/dev/kmem"},
		{"sda block device", "/dev/sda"},
		{"random tty", "/dev/ttyUSB0"},
		{"dri-lookalike outside tree", "/dev/dripper"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			yaml := "services:\n  app:\n    image: x\n    devices:\n      - " + tc.path + "\n"
			res := Validate([]byte(yaml))
			found := false
			for _, v := range res.Violations {
				if v.Code == "devices_block" {
					found = true
				}
			}
			if !found {
				t.Errorf("expected devices_block violation for %s; got OK=%v violations=%+v", tc.path, res.OK, res.Violations)
			}
		})
	}
}

// One bad apple in a multi-device list MUST still reject the whole
// block (no partial-accept semantics — the operator should see the
// concrete violation and decide).
func TestValidate_DriPlusRogueIsRejected(t *testing.T) {
	yaml := "services:\n  app:\n    image: x\n    devices:\n      - /dev/dri:/dev/dri\n      - /dev/kmem:/dev/kmem\n"
	res := Validate([]byte(yaml))
	if res.OK {
		t.Fatalf("OK=true with /dev/kmem present; want violations")
	}
	found := false
	for _, v := range res.Violations {
		if v.Code == "devices_block" && strings.Contains(v.Detail, "kmem") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected devices_block violation naming /dev/kmem; got %+v", res.Violations)
	}
}
