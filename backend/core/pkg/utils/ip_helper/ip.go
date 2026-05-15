package ip_helper

import (
	"fmt"
	"net"
)

// GetDeviceAllIPv4 returns a map of interface-name to IPv4 address
// for every non-loopback, currently-up interface. Used by the panel
// to surface "this device is reachable at" when the user installs an
// app and needs to know which LAN IP serves it.
func GetDeviceAllIPv4() map[string]string {
	address := make(map[string]string)
	addrs, err := net.Interfaces()
	if err != nil {
		return address
	}
	for _, a := range addrs {
		if a.Flags&net.FlagLoopback != 0 || a.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := a.Addrs()
		if err != nil {
			fmt.Println("Error:", err)
			continue
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
				address[a.Name] = ipnet.IP.String()
			}
		}
	}
	return address
}
