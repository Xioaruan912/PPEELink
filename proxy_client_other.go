//go:build !windows

package main

import (
	"net"
	"net/http"
	"syscall"
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

// newDirectPhysicalHTTPClient builds Channel A: binds physical NIC and disables
// proxy so requests completely bypass TUN and System Proxy.
func newDirectPhysicalHTTPClient(timeout time.Duration) *http.Client {
	iface := findPhysicalInterface(false)
	dialer := &net.Dialer{
		Timeout: 8 * time.Second,
		Control: func(network, _ string, c syscall.RawConn) error {
			return bindSocketToInterface(network, c, iface)
		},
	}
	if iface != nil && iface.IPv4 != nil {
		dialer.LocalAddr = &net.TCPAddr{IP: iface.IPv4}
	}
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
