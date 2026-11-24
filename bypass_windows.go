//go:build windows

package main

import (
	"encoding/binary"
	"syscall"
)

const (
	winIPUnicastIf   = 31
	winIPv6UnicastIf = 31
)

func bindSocketToInterface(network string, c syscall.RawConn, iface *physicalInterface) error {
	if iface == nil || iface.Index <= 0 {
		return nil
	}
	var sockErr error
	err := c.Control(func(fd uintptr) {
		if network == "tcp6" || network == "udp6" {
			sockErr = syscall.SetsockoptInt(syscall.Handle(fd), syscall.IPPROTO_IPV6, winIPv6UnicastIf, iface.Index)
			return
		}
		// On Windows, IP_UNICAST_IF expects the interface index in network byte order (big-endian).
		var buf [4]byte
		binary.BigEndian.PutUint32(buf[:], uint32(iface.Index))
		idxBE := int(binary.LittleEndian.Uint32(buf[:]))
		sockErr = syscall.SetsockoptInt(syscall.Handle(fd), syscall.IPPROTO_IP, winIPUnicastIf, idxBE)
	})
	if err != nil {
		return err
	}
	return sockErr
}
