//go:build linux

package main

import "syscall"

func bindSocketToInterface(_ string, c syscall.RawConn, iface *physicalInterface) error {
	if iface == nil || iface.Name == "" {
		return nil
	}
	_ = c.Control(func(fd uintptr) {
		// Best-effort SO_BINDTODEVICE (requires CAP_NET_RAW or root; LocalAddr still binds non-root).
		_ = syscall.SetsockoptString(int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, iface.Name)
	})
	return nil
}
