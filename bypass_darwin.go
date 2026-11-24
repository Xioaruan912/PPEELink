//go:build darwin

package main

import "syscall"

const (
	darwinIPBoundIf   = 25
	darwinIPv6BoundIf = 125
)

func bindSocketToInterface(network string, c syscall.RawConn, iface *physicalInterface) error {
	if iface == nil || iface.Index <= 0 {
		return nil
	}
	var sockErr error
	err := c.Control(func(fd uintptr) {
		if network == "tcp6" || network == "udp6" {
			sockErr = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IPV6, darwinIPv6BoundIf, iface.Index)
			return
		}
		sockErr = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IP, darwinIPBoundIf, iface.Index)
	})
	if err != nil {
		return err
	}
	return sockErr
}
