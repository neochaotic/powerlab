package service

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/mileusna/useragent"
	model2 "github.com/neochaotic/powerlab/backend/core/service/model"
)

// Name is the parsed user-agent + device descriptor surfaced on
// the legacy WebSocket peer-connect handshake — drives the "Other
// devices" peer list display.
type Name struct {
	Model       string `json:"model"`
	OS          string `json:"os"`
	Browser     string `json:"browser"`
	DeviceName  string `json:"deviceName"`
	DisplayName string `json:"displayName"`
}

// GetPeerId returns the persisted peerid cookie if set, otherwise
// the fallback id (typically a freshly-generated UUID).
func GetPeerId(request *http.Request, id string) string {
	cookiePree, err := request.Cookie("peerid")
	if err != nil {
		return id
	}
	if len(cookiePree.Value) > 0 {
		return cookiePree.Value
	}
	return id
}

// GetIP returns the request's client IP, preferring the
// X-Forwarded-For first hop when set. ::1 / IPv6-mapped loopback
// is normalised to 127.0.0.1 so peer dedupe works.
func GetIP(request *http.Request) string {
	ip := ""
	if len(request.Header.Get("x-forwarded-for")) > 0 {
		ip = strings.Split(request.Header.Get("x-forwarded-for"), ",")[0]
	} else {
		ip = request.RemoteAddr
	}

	if ip == "::1" || ip == "::ffff:127.0.0.1" {
		ip = "127.0.0.1"
	}
	return ip
}

// GetName parses the user-agent header into a Name suitable for
// display in the peer list.
func GetName(request *http.Request) Name {
	us := useragent.Parse(request.Header.Get("user-agent"))
	device := ""
	if len(us.Device) > 0 {
		device += us.Device
	} else {
		device += us.Name
	}

	display := ""
	if len(us.Device) > 0 {
		display = us.Device + " " + us.Name
	} else {
		display = us.OS + " " + us.Name
	}

	model := "desktop"
	if us.Mobile {
		model = "mobile"
	}
	if us.Tablet {
		model = "tablet"
	}
	peer := MyService.Peer().GetPeerByName(display)
	if len(peer.ID) > 0 {
		for i := 0; true; i++ {
			peer = MyService.Peer().GetPeerByName(display + "_" + strconv.Itoa(i+1))
			if len(peer.ID) == 0 {
				display = display + "_" + strconv.Itoa(i+1)
				break
			}
		}
	}

	return Name{
		Model:       model,
		OS:          us.OS,
		Browser:     us.Name,
		DeviceName:  device,
		DisplayName: display,
	}
}

// GetNameByDB rebuilds a Name from a persisted peer row — used
// when a known peer reconnects and we want to keep the previously
// displayed name.
func GetNameByDB(m model2.PeerDriveDBModel) Name {
	device := ""
	if len(m.DeviceName) > 0 {
		device += m.DeviceName
	} else {
		device += m.Browser
	}
	return Name{
		Model:       m.Model,
		OS:          m.OS,
		Browser:     m.Browser,
		DeviceName:  device,
		DisplayName: m.DisplayName,
	}
}
