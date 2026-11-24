package main

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestBuildQUICVersionProbe(t *testing.T) {
	pkt := buildQUICVersionProbe()
	if len(pkt) != 1200 {
		t.Fatalf("want 1200 bytes, got %d", len(pkt))
	}
	if pkt[0]&0xc0 != 0xc0 {
		t.Fatalf("expected long header + fixed bit, got 0x%x", pkt[0])
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

func TestProbeQUICReachable(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	go func() {
		buf := make([]byte, 1500)
		if _, from, err := pc.ReadFrom(buf); err == nil {
			_, _ = pc.WriteTo([]byte{0x80, 0x00, 0x00, 0x00, 0x00, 0x08}, from)
		}
	}()
	addr := pc.LocalAddr().(*net.UDPAddr)
	res := probeOne(context.Background(), probeNode{
		ID: 8, Name: "hy2", Host: "127.0.0.1", Port: addr.Port, Scheme: "hysteria2",
	}, 2*time.Second)
	if !res.Success || res.RTT <= 0 || res.Kind != "quic" || res.IPVersion != 4 {
		t.Fatalf("unexpected QUIC probe result: %+v", res)
	}
}
