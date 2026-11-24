//go:build windows

package main

import (
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"
)

// readWindowsSystemProxy queries HKCU Internet Settings so Channel B follows
// Windows System Proxy even when Clash TUN mode is turned off.
func readWindowsSystemProxy() *url.URL {
	cmd := exec.Command("reg", "query", `HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`)
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	enabled := false
	server := ""
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) < 3 {
			continue
		}
		switch strings.ToLower(f[0]) {
		case "proxyenable":
			v := strings.ToLower(f[2])
			enabled = v == "0x1" || v == "1"
		case "proxyserver":
			server = strings.TrimSpace(f[2])
		}
	}
	if !enabled || server == "" {
		return nil
	}
	// Handle per-protocol format like "http=127.0.0.1:7897;https=127.0.0.1:7897"
	if strings.Contains(server, "=") {
		picked := ""
		for _, part := range strings.Split(server, ";") {
			kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
			if len(kv) != 2 {
				continue
			}
			k := strings.ToLower(strings.TrimSpace(kv[0]))
			if k == "https" || k == "http" || k == "socks" {
				picked = strings.TrimSpace(kv[1])
				if k == "https" {
					break
				}
			}
		}
		server = picked
	}
	if server == "" {
		return nil
	}
	if !strings.Contains(server, "://") {
		server = "http://" + server
	}
	u, err := url.Parse(server)
	if err != nil || u.Host == "" {
		return nil
	}
	return u
}

// newActiveProxyHTTPClient builds Channel B: routes through active Clash TUN
// and/or Windows System Proxy / env proxy so unlock & proxy speedtest measure
// the user's real active proxy node.
func newActiveProxyHTTPClient(timeout time.Duration) *http.Client {
	winProxy := readWindowsSystemProxy()
	proxyFn := func(req *http.Request) (*url.URL, error) {
		if u, err := http.ProxyFromEnvironment(req); err == nil && u != nil {
			return u, nil
		}
		return winProxy, nil
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:                 proxyFn,
			DialContext:           (&net.Dialer{Timeout: 8 * time.Second}).DialContext,
			ForceAttemptHTTP2:     true,
			TLSHandshakeTimeout:   8 * time.Second,
			ResponseHeaderTimeout: timeout,
			MaxIdleConnsPerHost:   8,
		},
	}
}

// newDirectPhysicalHTTPClient builds Channel A: binds physical NIC, uses a
// physical-NIC-bound DNS resolver (bypassing Clash fake-ip), and disables proxy.
func newDirectPhysicalHTTPClient(timeout time.Duration) *http.Client {
	iface := findPhysicalInterface(false)
	dialer := newPhysicalDialer(iface, 8*time.Second)
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     true,
			TLSHandshakeTimeout:   8 * time.Second,
			ResponseHeaderTimeout: timeout,
			MaxIdleConnsPerHost:   8,
		},
	}
}
