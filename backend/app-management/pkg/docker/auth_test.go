package docker

import (
	"net/url"
	"strings"
	"testing"
)

// Closes #244 — Docker registry probes used to send
// `User-Agent: CasaOS`, identifying every PowerLab install as
// CasaOS to private registry logs. Header must now be PowerLab-
// branded with the app-management version.
func TestGetChallengeRequest_UserAgentIsPowerLab(t *testing.T) {
	u, err := url.Parse("https://registry.example/v2/")
	if err != nil {
		t.Fatalf("url.Parse failed: %v", err)
	}

	req, err := GetChallengeRequest(*u)
	if err != nil {
		t.Fatalf("GetChallengeRequest returned error: %v", err)
	}

	ua := req.Header.Get("User-Agent")
	if !strings.HasPrefix(ua, "PowerLab/") {
		t.Fatalf("User-Agent = %q, want prefix \"PowerLab/\"", ua)
	}
	if strings.EqualFold(ua, "CasaOS") {
		t.Fatalf("User-Agent regressed to CasaOS literal — #244 must not come back")
	}
}
