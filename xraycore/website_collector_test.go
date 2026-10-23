package xraycore

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"
)

func websiteTestSpec() Spec {
	return Spec{Enabled: true, APIPort: 18085, WebsiteTraffic: &WebsiteTrafficSpec{Enabled: true}, Inbounds: []InboundSpec{{Tag: "in-7", Template: TemplateRealityVision, Port: 18443, Target: "www.apple.com:443", SNI: "www.apple.com", PrivateKey: "test", ShortID: "a1b2"}, {Tag: "in-8", Template: TemplateRealityVision, Port: 18444, Target: "www.apple.com:443", SNI: "www.apple.com", PrivateKey: "test", ShortID: "a1b2"}}}
}
func websiteTestCollector(t *testing.T, s Spec) *WebsiteCollector {
	t.Helper()
	c, err := NewWebsiteCollector(s)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}
func websiteTestAuth(t *testing.T, c *WebsiteCollector, user string) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("tcp", c.Addr(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	_, _ = conn.Write([]byte{5, 1, 2})
	var reply [2]byte
	if _, err := io.ReadFull(conn, reply[:]); err != nil || reply != [2]byte{5, 2} {
		t.Fatalf("auth method: %v %v", reply, err)
	}
	id := c.identities[user]
	auth := append([]byte{1, byte(len(user))}, []byte(user)...)
	auth = append(auth, byte(len(id.password)))
	auth = append(auth, []byte(id.password)...)
	_, _ = conn.Write(auth)
	if _, err := io.ReadFull(conn, reply[:]); err != nil || reply != [2]byte{1, 0} {
		t.Fatalf("auth: %v %v", reply, err)
	}
	return conn
}
func websiteTestRequest(t *testing.T, conn net.Conn, cmd byte, host string, port int) (byte, *net.UDPAddr) {
	t.Helper()
	req := []byte{5, cmd, 0}
	if ip := net.ParseIP(host); ip != nil && ip.To4() != nil {
		req = append(req, 1)
		req = append(req, ip.To4()...)
	} else {
		req = append(req, 3, byte(len(host)))
		req = append(req, []byte(host)...)
	}
	req = append(req, byte(port>>8), byte(port))
	if _, err := conn.Write(req); err != nil {
		t.Fatal(err)
	}
	var reply [10]byte
	if _, err := io.ReadFull(conn, reply[:]); err != nil {
		t.Fatal(err)
	}
	return reply[1], &net.UDPAddr{IP: net.IP(reply[4:8]), Port: int(binary.BigEndian.Uint16(reply[8:]))}
}
func websiteTestTCPServer(t *testing.T) (int, <-chan error) {
	t.Helper()
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	done := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		in, err := io.ReadAll(conn)
		if err == nil && !bytes.Equal(in, []byte("request")) {
			err = fmt.Errorf("unexpected payload: %q", in)
		}
		if err == nil {
			_, err = conn.Write([]byte("response-body"))
		}
		done <- err
	}()
	return ln.Addr().(*net.TCPAddr).Port, done
}

func TestWebsiteTCPRealBytesHalfCloseAndAttribution(t *testing.T) {
	for _, host := range []string{"localhost", "127.0.0.1"} {
		t.Run(host, func(t *testing.T) {
			c := websiteTestCollector(t, websiteTestSpec())
			port, done := websiteTestTCPServer(t)
			conn := websiteTestAuth(t, c, "1")
			if code, _ := websiteTestRequest(t, conn, 1, host, port); code != 0 {
				t.Fatalf("connect %d", code)
			}
			_, _ = conn.Write([]byte("request"))
			_ = conn.(*net.TCPConn).CloseWrite()
			reply, err := io.ReadAll(conn)
			if err != nil || string(reply) != "response-body" {
				t.Fatalf("response %q: %v", reply, err)
			}
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			_ = c.Close()
			b := c.Drain()
			domain := host
			if host == "127.0.0.1" {
				domain = "unknown"
			}
			if len(b) != 1 || b[0].Domain != domain || b[0].InboundTag != "in-8" || b[0].Up != 7 || b[0].Down != 13 || b[0].BucketStart%3600 != 0 {
				t.Fatalf("wrong counters: %+v", b)
			}
			if len(c.Drain()) != 0 {
				t.Fatal("drain did not reset")
			}
		})
	}
}
func TestWebsiteAuthRequiredAndPasswordChecked(t *testing.T) {
	c := websiteTestCollector(t, websiteTestSpec())
	for _, request := range [][]byte{{5, 1, 0}, {5, 1, 2, 1, 1, '0', 3, 'b', 'a', 'd'}} {
		conn, err := net.Dial("tcp", c.Addr())
		if err != nil {
			t.Fatal(err)
		}
		_ = conn.SetDeadline(time.Now().Add(time.Second))
		_, _ = conn.Write(request)
		reply, _ := io.ReadAll(conn)
		_ = conn.Close()
		if bytes.Equal(request, []byte{5, 1, 0}) {
			if !bytes.Equal(reply, []byte{5, 255}) {
				t.Fatalf("no-auth accepted: %v", reply)
			}
		} else if !bytes.Equal(reply, []byte{5, 2, 1, 1}) {
			t.Fatalf("bad credentials accepted: %v", reply)
		}
	}
	if len(c.Drain()) != 0 {
		t.Fatal("handshake bytes metered")
	}
	host, _, _ := net.SplitHostPort(c.Addr())
	if host != "127.0.0.1" {
		t.Fatalf("non-loopback listener: %s", c.Addr())
	}
}
func TestWebsiteBlockPrivateResolvesDomains(t *testing.T) {
	s := websiteTestSpec()
	s.BlockPrivate = true
	c := websiteTestCollector(t, s)
	for _, host := range []string{"localhost", "127.0.0.1", "::1", "169.254.169.254", "100.64.0.1"} {
		if _, err := c.resolve(context.Background(), host, 80); err == nil {
			t.Fatalf("private target accepted: %s", host)
		}
	}
	conn := websiteTestAuth(t, c, "0")
	if code, _ := websiteTestRequest(t, conn, 1, "localhost", 80); code == 0 {
		t.Fatal("blocked CONNECT succeeded")
	}
	if len(c.Drain()) != 0 {
		t.Fatal("failed connect counted")
	}
}

func websiteUDPRequest(host string, port int, payload []byte) []byte {
	b := []byte{0, 0, 0}
	if ip := net.ParseIP(host); ip != nil {
		b = append(b, 1)
		b = append(b, ip.To4()...)
	} else {
		b = append(b, 3, byte(len(host)))
		b = append(b, []byte(host)...)
	}
	b = append(b, byte(port>>8), byte(port))
	return append(b, payload...)
}
func TestWebsiteUDPBytesIsolationAndControlLifetime(t *testing.T) {
	c := websiteTestCollector(t, websiteTestSpec())
	server, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		b := make([]byte, 2048)
		for {
			n, peer, err := server.ReadFromUDP(b)
			if err != nil {
				return
			}
			_, _ = server.WriteToUDP(append([]byte("reply:"), b[:n]...), peer)
		}
	}()
	client, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	control := websiteTestAuth(t, c, "0")
	code, relay := websiteTestRequest(t, control, 3, "0.0.0.0", client.LocalAddr().(*net.UDPAddr).Port)
	if code != 0 {
		t.Fatalf("associate failed: %d", code)
	}
	// A different local UDP source is not an authenticated association participant.
	intruder, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer intruder.Close()
	port := server.LocalAddr().(*net.UDPAddr).Port
	_, _ = intruder.WriteToUDP(websiteUDPRequest("localhost", port, []byte("forged")), relay)
	fragment := websiteUDPRequest("localhost", port, []byte("fragment"))
	fragment[2] = 1
	_, _ = client.WriteToUDP(fragment, relay)
	for _, host := range []string{"localhost", "127.0.0.1"} {
		packet := websiteUDPRequest(host, port, []byte("hello"))
		if _, err := client.WriteToUDP(packet, relay); err != nil {
			t.Fatal(err)
		}
		_ = client.SetReadDeadline(time.Now().Add(3 * time.Second))
		buf := make([]byte, 2048)
		n, _, err := client.ReadFromUDP(buf)
		if err != nil {
			t.Fatal(err)
		}
		if string(buf[10:n]) != "reply:hello" {
			t.Fatalf("bad UDP response: %v", buf[:n])
		}
	}
	_ = c.Close()
	b := c.Drain()
	if len(b) != 2 {
		t.Fatalf("expected domain and unknown: %+v", b)
	}
	for _, entry := range b {
		if entry.Up != 5 || entry.Down != 11 || entry.InboundTag != "in-7" || (entry.Domain != "localhost" && entry.Domain != "unknown") {
			t.Fatalf("incorrect UDP counters %+v", b)
		}
	}
	_ = control.Close()
	_ = server.Close()
	<-serverDone
	// Close waits for relay goroutines and releases all active association sockets.
	if len(c.connections) != 0 {
		t.Fatalf("connections leaked: %d", len(c.connections))
	}
}
func TestWebsiteLimitsAndDrainConcurrent(t *testing.T) {
	s := newWebsiteCounters()
	for i := 0; i < websiteMaxDomains+10; i++ {
		k, err := s.reserve("in", fmt.Sprintf("d%d.example", i), 3600)
		if err != nil {
			t.Fatal(err)
		}
		s.add(k, 1, true)
	}
	b := s.drain()
	var total, unknown int64
	for _, r := range b {
		total += r.Up
		if r.Domain == "unknown" {
			unknown += r.Up
		}
	}
	if total != websiteMaxDomains+10 || unknown != 10 {
		t.Fatalf("cardinality overflow incorrectly attributed total=%d unknown=%d", total, unknown)
	}
	for i := 0; i < websiteMaxRecords; i++ {
		if _, err := s.reserve(strconv.Itoa(i), "example.com", 3600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.reserve("overflow", "example.com", 3600); err == nil {
		t.Fatal("unbounded pending queue")
	}
	_ = s.drain()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				k, err := s.reserve("in", "example.com", 3600)
				if err != nil {
					panic(err)
				}
				s.add(k, 1, true)
			}
		}()
	}
	finished := make(chan struct{})
	go func() { wg.Wait(); close(finished) }()
	var count int64
	for {
		for _, r := range s.drain() {
			count += r.Up
		}
		select {
		case <-finished:
			for _, r := range s.drain() {
				count += r.Up
			}
			if count != 8000 {
				t.Fatalf("drain race lost data: %d", count)
			}
			return
		default:
		}
	}
}
func TestWebsiteDomainIsNotIPGuessing(t *testing.T) {
	for _, host := range []string{"1.2.3.4", "::1", "evil/path", "a..b", "-foo.com", "foo_.com", ""} {
		if websiteDomain(host) != "unknown" {
			t.Fatalf("invalid/IP attributed: %q", host)
		}
	}
	if websiteDomain("WWW.Example.COM.") != "www.example.com" {
		t.Fatal("domain normalization")
	}
}
func TestWebsiteConfigRequiresCollectorAndPreservesRouteOnly(t *testing.T) {
	s := websiteTestSpec()
	if _, err := BuildConfig(s); err == nil {
		t.Fatal("collection silently bypassed")
	}
	c := websiteTestCollector(t, s)
	raw, err := BuildConfigWithWebsiteCollector(s, c)
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Outbounds []struct{ Tag, Protocol string }
		Routing   struct {
			Rules []struct {
				InboundTag  []string
				OutboundTag string
			}
		}
		Inbounds []struct{ Sniffing struct{ RouteOnly bool } }
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Outbounds[0].Protocol != "blackhole" {
		t.Fatal("unmatched traffic may escape meter")
	}
	for _, out := range cfg.Outbounds {
		if out.Protocol == "freedom" {
			t.Fatal("unmetered freedom outbound")
		}
	}
	if !cfg.Inbounds[1].Sniffing.RouteOnly {
		t.Fatal("sniffing overrides original destination")
	}
	if len(cfg.Routing.Rules) != 3 {
		t.Fatalf("per-node routes missing: %+v", cfg.Routing)
	}
}
func TestWebsiteSpecValidationAndRunnerRetainsFinalDelta(t *testing.T) {
	s := websiteTestSpec()
	s.WebsiteTraffic.BucketSeconds = 61
	if c, err := NewWebsiteCollector(s); err == nil {
		_ = c.Close()
		t.Fatal("invalid bucket accepted")
	}
	s = websiteTestSpec()
	s.Inbounds[1].Tag = s.Inbounds[0].Tag
	if c, err := NewWebsiteCollector(s); err == nil {
		_ = c.Close()
		t.Fatal("duplicate inbound tags accepted")
	}
	s = websiteTestSpec()
	r := NewRunner(t.TempDir(), "test")
	r.systemd = false
	r.websiteStats = newWebsiteCounters()
	c, err := newWebsiteCollector(s, r.websiteStats)
	if err != nil {
		t.Fatal(err)
	}
	r.website = c
	k, _ := r.websiteStats.reserve("in-7", "example.com", 3600)
	r.websiteStats.add(k, 10, true)
	s.Enabled = false
	if err := r.Apply(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if r.website != nil {
		t.Fatal("disabled proxy still forwarding")
	}
	if b := r.CollectWebsiteTraffic(); len(b) != 1 || b[0].Up != 10 {
		t.Fatalf("disabled collector lost final traffic: %+v", b)
	}
	if _, err := net.DialTimeout("tcp", c.Addr(), time.Second); err == nil {
		t.Fatal("listener alive after disabling")
	}
}

type websiteFaultConn struct {
	net.Conn
	input   io.Reader
	count   int
	writes  int
	failure error
}

func (c *websiteFaultConn) Read(b []byte) (int, error)       { return c.input.Read(b) }
func (c *websiteFaultConn) Write(b []byte) (int, error)      { c.writes++; return c.count, c.failure }
func (c *websiteFaultConn) Close() error                     { return nil }
func (c *websiteFaultConn) SetReadDeadline(time.Time) error  { return nil }
func (c *websiteFaultConn) SetWriteDeadline(time.Time) error { return nil }
func TestWebsiteOnlySuccessfulTCPWritesCount(t *testing.T) {
	for _, written := range []int{0, 3, 7} {
		t.Run(strconv.Itoa(written), func(t *testing.T) {
			c := &WebsiteCollector{stats: newWebsiteCounters(), seconds: 3600}
			src := &websiteFaultConn{input: bytes.NewReader([]byte("request"))}
			dst := &websiteFaultConn{count: written, failure: io.ErrClosedPipe}
			c.copy(dst, src, "in-7", "example.com", true)
			batch := c.Drain()
			if written == 0 {
				if len(batch) != 0 {
					t.Fatalf("failed write counted: %+v", batch)
				}
			} else if len(batch) != 1 || batch[0].Up != int64(written) || batch[0].Down != 0 {
				t.Fatalf("partial write incorrect: %+v", batch)
			}
		})
	}
}
func TestWebsiteFullQueueDoesNotForwardPayload(t *testing.T) {
	c := &WebsiteCollector{stats: newWebsiteCounters(), seconds: 3600}
	for i := 0; i < websiteMaxRecords; i++ {
		_, _ = c.stats.reserve(strconv.Itoa(i), "example.com", 3600)
	}
	src := &websiteFaultConn{input: bytes.NewReader([]byte("request"))}
	dst := &websiteFaultConn{count: 7}
	c.copy(dst, src, "overflow", "example.com", true)
	if dst.writes != 0 {
		t.Fatal("queue full but payload was forwarded")
	}
	if batch := c.Drain(); len(batch) != 0 {
		t.Fatalf("queue overflow forwarded unmetered bytes: %+v", batch)
	}
}

// Set XRAY_WEBSITE_E2E_BIN to a verified official binary to validate Xray's
// actual SOCKS implementation, including UDP ASSOCIATE and long auth secrets.
func TestWebsiteXraySOCKSE2E(t *testing.T) {
	bin := os.Getenv("XRAY_WEBSITE_E2E_BIN")
	if bin == "" {
		t.Skip("set XRAY_WEBSITE_E2E_BIN to run real Xray TCP+UDP test")
	}
	s := websiteTestSpec()
	c := websiteTestCollector(t, s)
	out, rules := c.configOutbounds()
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	proxyPort := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	cfg := map[string]any{"log": map[string]any{"loglevel": "debug"}, "inbounds": []any{map[string]any{"tag": "in-7", "listen": "127.0.0.1", "port": proxyPort, "protocol": "socks", "settings": map[string]any{"auth": "noauth", "udp": true, "ip": "127.0.0.1"}}}, "outbounds": out, "routing": map[string]any{"domainStrategy": "AsIs", "rules": rules}}
	raw, _ := json.Marshal(cfg)
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(bin, "run", "-test", "-c", path).CombinedOutput(); err != nil {
		t.Fatalf("Xray configuration invalid: %s %v", output, err)
	}
	cmd := exec.Command(bin, "run", "-c", path)
	var log bytes.Buffer
	cmd.Stdout = &log
	cmd.Stderr = &log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait(); t.Log(log.String()) })
	var proxy net.Conn
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		proxy, err = net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", proxyPort), 100*time.Millisecond)
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("Xray failed to start: %v", err)
	}
	defer proxy.Close()
	_ = proxy.SetDeadline(time.Now().Add(5 * time.Second))
	_, _ = proxy.Write([]byte{5, 1, 0})
	var greet [2]byte
	if _, err := io.ReadFull(proxy, greet[:]); err != nil || greet != [2]byte{5, 0} {
		t.Fatalf("client greeting %v %v", greet, err)
	}
	tcpServer, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer tcpServer.Close()
	serverDone := make(chan error, 1)
	go func() {
		conn, err := tcpServer.Accept()
		if err != nil {
			serverDone <- err
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		request := make([]byte, 7)
		_, err = io.ReadFull(conn, request)
		if err == nil {
			_, err = conn.Write([]byte("response-body"))
		}
		serverDone <- err
	}()
	port := tcpServer.Addr().(*net.TCPAddr).Port
	if code, _ := websiteTestRequest(t, proxy, 1, "localhost", port); code != 0 {
		t.Fatalf("Xray CONNECT %d", code)
	}
	_, _ = proxy.Write([]byte("request"))
	response, err := io.ReadAll(proxy)
	if err != nil || string(response) != "response-body" {
		t.Fatalf("Xray TCP %q %v", response, err)
	}
	if err := <-serverDone; err != nil {
		t.Fatal(err)
	}
	// A local DNS-format request is not needed: this generic UDP echo validates the
	// actual packet forwarding path and verifies payload-only accounting.
	udpServer, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer udpServer.Close()
	udpDone := make(chan error, 1)
	go func() {
		buf := make([]byte, 1024)
		_ = udpServer.SetDeadline(time.Now().Add(5 * time.Second))
		n, peer, err := udpServer.ReadFromUDP(buf)
		if err == nil {
			_, err = udpServer.WriteToUDP(append([]byte("echo:"), buf[:n]...), peer)
		}
		udpDone <- err
	}()
	udpClient, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer udpClient.Close()
	control, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", proxyPort))
	if err != nil {
		t.Fatal(err)
	}
	defer control.Close()
	_ = control.SetDeadline(time.Now().Add(5 * time.Second))
	_, _ = control.Write([]byte{5, 1, 0})
	if _, err := io.ReadFull(control, greet[:]); err != nil {
		t.Fatal(err)
	}
	code, relay := websiteTestRequest(t, control, 3, "0.0.0.0", udpClient.LocalAddr().(*net.UDPAddr).Port)
	if code != 0 {
		t.Fatalf("Xray UDP associate %d", code)
	}
	packet := websiteUDPRequest("localhost", udpServer.LocalAddr().(*net.UDPAddr).Port, []byte("udp"))
	_, _ = udpClient.WriteToUDP(packet, relay)
	_ = udpClient.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, 1024)
	n, _, err := udpClient.ReadFromUDP(buf)
	if err != nil || string(buf[10:n]) != "echo:udp" {
		t.Fatalf("Xray UDP reply %q %v", buf[:n], err)
	}
	if err := <-udpDone; err != nil {
		t.Fatal(err)
	}
	_ = c.Close()
	b := c.Drain()
	if len(b) != 1 || b[0].Domain != "localhost" || b[0].Up != 10 || b[0].Down != 21 {
		t.Fatalf("Xray real counters: %+v", b)
	}
}
