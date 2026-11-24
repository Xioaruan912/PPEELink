//go:build !windows

package main

import (
	"net"
	"net/http"
	"time"
)

// newActiveProxyHTTPClient builds Channel B: follows OS routing (utun/TUN) and
// environment proxy variables so unlock & speedtest measure the active proxy.
func newActiveProxyHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           (&net.Dialer{Timeout: 8 * time.Second}).DialContext,
			ForceAttemptHTTP2:     true,
			TLSHandshakeTimeout:   8 * time.Second,
			ResponseHeaderTimeout: timeout,
			MaxIdleConnsPerHost:   8,
		},
	}
}

// newDirectPhysicalHTTPClient builds Channel A: binds physical NIC, uses a
// physical-NIC-bound DNS resolver (bypassing TUN fake-ip), and disables proxy.
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
