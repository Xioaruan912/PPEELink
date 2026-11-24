package main

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestVirtualInterfaceFilter(t *testing.T) {
	if !isVirtualInterface(net.Interface{Name: "Meta", Flags: net.FlagUp, HardwareAddr: net.HardwareAddr{1, 2, 3, 4, 5, 6}}) {
		t.Fatal("Meta TUN interface must be treated as virtual")
	}
	if !isVirtualInterface(net.Interface{Name: "utun3", Flags: net.FlagUp, HardwareAddr: net.HardwareAddr{1, 2, 3, 4, 5, 6}}) {
		t.Fatal("utun3 interface must be treated as virtual")
	}
	if isVirtualInterface(net.Interface{Name: "Ethernet", Flags: net.FlagUp, HardwareAddr: net.HardwareAddr{1, 2, 3, 4, 5, 6}}) {
		t.Fatal("physical Ethernet must not be treated as virtual")
	}
}

func TestProbeTCPReachable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		if c, err := ln.Accept(); err == nil {
			_ = c.Close()
		}
	}()
	addr := ln.Addr().(*net.TCPAddr)
	res := probeOne(context.Background(), probeNode{
		ID: 7, Name: "local", Host: "127.0.0.1", Port: addr.Port, Scheme: "vless",
	}, 2*time.Second)
	if !res.Success || res.RTT <= 0 || res.Kind != "tcp" || res.IPVersion != 4 {
		t.Fatalf("unexpected TCP probe result: %+v", res)
	}
}

func TestFakeIPFilter(t *testing.T) {
	for _, s := range []string{"198.18.0.15", "198.19.255.1", "28.0.0.42"} {
		if !isFakeOrReservedIP(net.ParseIP(s)) {
			t.Fatalf("expected %s to be detected as fake-ip", s)
		}
	}
	for _, s := range []string{"211.80.216.5", "119.29.29.29", "179.255.148.213"} {
		if isFakeOrReservedIP(net.ParseIP(s)) {
			t.Fatalf("expected %s to be treated as real public IP", s)
		}
	}
}
