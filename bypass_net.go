package main

import (
	"net"
	"strings"
)

type physicalInterface struct {
	Index int
	Name  string
	IPv4  net.IP
	IPv6  net.IP
}

var virtualIfaceKeywords = []string{
	"meta", "clash", "mihomo", "wintun", "sing", "tun", "utun", "tap",
	"tailscale", "wireguard", "wg", "zerotier", "vethernet", "hyper-v",
	"vmware", "virtualbox", "vbox", "docker", "br-", "veth", "wsl",
	"loopback", "pseudo", "teredo", "isatap", "npcap", "nordlynx",
	"warp", "cloudflare", "shadowrocket", "surge", "loon", "nekobox",
}

func isVirtualInterface(iface net.Interface) bool {
	if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
		return true
	}
	if iface.Flags&net.FlagPointToPoint != 0 {
		return true
	}
	low := strings.ToLower(iface.Name)
	for _, kw := range virtualIfaceKeywords {
		if strings.Contains(low, kw) {
			return true
		}
	}
	// Physical Ethernet / Wi-Fi adapters always have a non-empty MAC address.
	if len(iface.HardwareAddr) == 0 {
		return true
	}
	return false
}

// findPhysicalInterface selects the best active non-TUN physical NIC for
// direct outbound probing (bypassing Clash/Mihomo TUN virtual adapters).
func findPhysicalInterface(wantIPv6 bool) *physicalInterface {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var best *physicalInterface
	for _, iface := range ifaces {
		if isVirtualInterface(iface) {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		var v4, v6 net.IP
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP == nil {
				continue
			}
			ip := ipNet.IP
			if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
				continue
			}
			if ip4 := ip.To4(); ip4 != nil {
				// Skip Clash/Mihomo fake-ip / TUN subnets (198.18.0.0/15, 198.19.x.x).
				if ip4[0] == 198 && (ip4[1] == 18 || ip4[1] == 19) {
					continue
				}
				if v4 == nil {
					v4 = ip4
				}
			} else if ip.IsGlobalUnicast() && v6 == nil {
				v6 = ip
			}
		}
		if wantIPv6 && v6 == nil {
			continue
		}
		if !wantIPv6 && v4 == nil {
			continue
		}
		cand := &physicalInterface{
			Index: iface.Index,
			Name:  iface.Name,
			IPv4:  v4,
			IPv6:  v6,
		}
		if best == nil {
			best = cand
		}
		// Prefer Wi-Fi / Ethernet over secondary adapters.
		low := strings.ToLower(iface.Name)
		if strings.Contains(low, "wi-fi") || strings.Contains(low, "wlan") ||
			strings.Contains(low, "ethernet") || strings.Contains(low, "以太网") ||
			strings.HasPrefix(low, "en0") || strings.HasPrefix(low, "eth0") {
			return cand
		}
	}
	return best
}

func targetIsIPv6(host string) bool {
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.To4() == nil
}

func isLoopbackHost(host string) bool {
	h := strings.ToLower(strings.Trim(strings.TrimSpace(host), "[]"))
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}
