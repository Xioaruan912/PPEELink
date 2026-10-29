package relaycore

import (
	"context"
	"errors"
	"io"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func tcpEcho(t *testing.T, marker string) (string, int) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			c, e := l.Accept()
			if e != nil {
				return
			}
			go func() {
				defer c.Close()
				b := make([]byte, 4096)
				for {
					n, e := c.Read(b)
					if n > 0 {
						c.Write(append([]byte(marker), b[:n]...))
					}
					if e != nil {
						return
					}
				}
			}()
		}
	}()
	h, p, _ := net.SplitHostPort(l.Addr().String())
	port, _ := strconv.Atoi(p)
	return h, port
}
func freePort(t *testing.T) int {
	t.Helper()
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	p := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return p
}
func testManager(t *testing.T) *Manager {
	t.Helper()
	m := NewManager(t.TempDir())
	m.allowPrivate = true
	t.Cleanup(m.Close)
	return m
}
func tcpClient(t *testing.T, p int) net.Conn {
	t.Helper()
	c, e := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(p)), time.Second)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { c.Close() })
	return c
}
func exchange(t *testing.T, c net.Conn, want string) {
	t.Helper()
	c.SetDeadline(time.Now().Add(time.Second))
	if _, e := c.Write([]byte("ping")); e != nil {
		t.Fatal(e)
	}
	b := make([]byte, len(want))
	if _, e := io.ReadFull(c, b); e != nil {
		t.Fatal(e)
	}
	if string(b) != want {
		t.Fatalf("got %q want %q", b, want)
	}
}
func waitFor(t *testing.T, f func() bool) {
	t.Helper()
	until := time.Now().Add(2 * time.Second)
	for time.Now().Before(until) {
		if f() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not reached")
}
func TestTCPUpdateDrainsExistingAndRetainsOnFailure(t *testing.T) {
	h, p := tcpEcho(t, "a")
	_, p2 := tcpEcho(t, "b")
	m := testManager(t)
	s := Spec{ID: "one", ListenPort: freePort(t), Network: "tcp", TargetHost: h, TargetPort: p}
	if e := m.Apply(context.Background(), []Spec{s}); e != nil {
		t.Fatal(e)
	}
	c := tcpClient(t, s.ListenPort)
	exchange(t, c, "aping")
	first := m.Status()[s.ID]
	s.TargetPort = p2
	if e := m.Apply(context.Background(), []Spec{s}); e != nil {
		t.Fatal(e)
	}
	exchange(t, c, "aping")
	exchange(t, tcpClient(t, s.ListenPort), "bping")
	occupied, e := net.Listen("tcp", "0.0.0.0:0")
	if e != nil {
		t.Fatal(e)
	}
	defer occupied.Close()
	bad := s
	bad.ListenPort = occupied.Addr().(*net.TCPAddr).Port
	if e := m.Apply(context.Background(), []Spec{bad}); e == nil {
		t.Fatal("expected listener conflict")
	}
	exchange(t, tcpClient(t, s.ListenPort), "bping")
	// The peer can receive a write before the forwarding goroutine records its
	// byte count; wait for the asynchronous counters before checking totals.
	waitFor(t, func() bool { st := m.Status()[s.ID]; return st.Up >= 16 && st.Down >= 20 })
	st := m.Status()[s.ID]
	if st.Session != first.Session || st.Up < 16 || st.Down < 20 || !st.Running || st.Revision == first.Revision {
		t.Fatalf("bad cumulative status: %+v", st)
	}
}
func udpEcho(t *testing.T, marker string) (string, int) {
	t.Helper()
	c, e := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { c.Close() })
	go func() {
		b := make([]byte, 4096)
		for {
			n, a, e := c.ReadFromUDP(b)
			if e != nil {
				return
			}
			c.WriteToUDP(append([]byte(marker), b[:n]...), a)
		}
	}()
	return "127.0.0.1", c.LocalAddr().(*net.UDPAddr).Port
}
func TestUDPStickyAndIdleCleanup(t *testing.T) {
	h, p := udpEcho(t, "a")
	_, p2 := udpEcho(t, "b")
	m := testManager(t)
	m.udpIdle = 50 * time.Millisecond
	s := Spec{ID: "udp", ListenPort: freePort(t), Network: "udp", TargetHost: h, TargetPort: p}
	if e := m.Apply(context.Background(), []Spec{s}); e != nil {
		t.Fatal(e)
	}
	dial := func() net.Conn {
		c, e := net.Dial("udp", net.JoinHostPort("127.0.0.1", strconv.Itoa(s.ListenPort)))
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() { c.Close() })
		return c
	}
	c := dial()
	exchange(t, c, "aping")
	s.TargetPort = p2
	if e := m.Apply(context.Background(), []Spec{s}); e != nil {
		t.Fatal(e)
	}
	exchange(t, c, "aping")
	exchange(t, dial(), "bping")
	if m.Status()[s.ID].UDPSessions != 2 {
		t.Fatal("expected two sticky sessions")
	}
	waitFor(t, func() bool { return m.Status()[s.ID].UDPSessions == 0 })
	exchange(t, c, "bping")
}
func TestInvalidTargetsAndRuleLimit(t *testing.T) {
	m := NewManager(t.TempDir())
	defer m.Close()
	for _, host := range []string{"127.0.0.1", "10.1.1.1", "100.64.1.2", "169.254.169.254", "::1", "::ffff:192.168.1.2", "224.0.0.1", "192.0.2.1"} {
		s := Spec{ID: "bad", ListenPort: freePort(t), Network: "tcp", TargetHost: host, TargetPort: 443}
		if e := m.Apply(context.Background(), []Spec{s}); e == nil {
			t.Fatalf("accepted nonpublic target %s", host)
		}
	}
	rules := make([]Spec, 65)
	if e := m.Apply(context.Background(), rules); e == nil {
		t.Fatal("accepted too many rules")
	}
}
func TestRestoreLastGoodAndDisabledState(t *testing.T) {
	h, p := tcpEcho(t, "")
	dir := t.TempDir()
	m := NewManager(dir)
	m.allowPrivate = true
	s := Spec{ID: "persist", ListenPort: freePort(t), Network: "tcp", TargetHost: h, TargetPort: p}
	if e := m.Apply(context.Background(), []Spec{s}); e != nil {
		t.Fatal(e)
	}
	m.Close()
	m = NewManager(dir)
	m.allowPrivate = true
	defer m.Close()
	if e := m.Restore(context.Background()); e != nil {
		t.Fatal(e)
	}
	exchange(t, tcpClient(t, s.ListenPort), "ping")
	if e := m.Apply(context.Background(), nil); e != nil {
		t.Fatal(e)
	}
	if m.Status()[s.ID].Running {
		t.Fatal("disabled relay is running")
	}
	m.Close()
	m = NewManager(dir)
	m.allowPrivate = true
	defer m.Close()
	if e := m.Restore(context.Background()); e != nil {
		t.Fatal(e)
	}
	if len(m.Status()) != 0 {
		t.Fatal("disabled state restored active relay")
	}
}

func TestCloseStopsIdleTCPImmediately(t *testing.T) {
	h, p := tcpEcho(t, "")
	m := NewManager(t.TempDir())
	m.allowPrivate = true
	s := Spec{ID: "idle", ListenPort: freePort(t), Network: "tcp", TargetHost: h, TargetPort: p}
	if e := m.Apply(context.Background(), []Spec{s}); e != nil {
		t.Fatal(e)
	}
	exchange(t, tcpClient(t, s.ListenPort), "ping")
	done := make(chan struct{})
	go func() { m.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("closing manager left an idle upstream goroutine alive")
	}
}
func TestDNSRejectsMixedAnswersAndPinsUnchangedSpec(t *testing.T) {
	m := NewManager(t.TempDir())
	defer m.Close()
	calls := 0
	m.resolve = func(context.Context, string) ([]netip.Addr, error) {
		calls++
		return []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("127.0.0.1")}, nil
	}
	s := Spec{ID: "dns", ListenPort: freePort(t), Network: "tcp", TargetHost: "relay.example", TargetPort: 443}
	if e := m.Apply(context.Background(), []Spec{s}); e == nil {
		t.Fatal("accepted mixed public/private DNS answers")
	}
	m.resolve = func(context.Context, string) ([]netip.Addr, error) {
		calls++
		return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
	}
	if e := m.Apply(context.Background(), []Spec{s}); e != nil {
		t.Fatal(e)
	}
	calls = 0
	m.resolve = func(context.Context, string) ([]netip.Addr, error) {
		calls++
		return []netip.Addr{netip.MustParseAddr("10.0.0.1")}, nil
	}
	if e := m.Apply(context.Background(), []Spec{s}); e != nil {
		t.Fatal(e)
	}
	if calls != 0 {
		t.Fatal("unchanged rules re-resolved a pinned target")
	}
	if m.rules[s.ID].target.address != "8.8.8.8:443" {
		t.Fatal("target was not pinned")
	}
	if m.Status()[s.ID].Revision != s.Rev() {
		t.Fatal("public revision contract does not match status")
	}
}

func TestCloseStopsUpstreamIgnoringHalfClose(t *testing.T) {
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	release := make(chan struct{})
	defer close(release)
	go func() {
		c, e := l.Accept()
		if e != nil {
			return
		}
		defer c.Close()
		b := make([]byte, 4)
		io.ReadFull(c, b)
		c.Write(b)
		<-release
	}()
	m := NewManager(t.TempDir())
	m.allowPrivate = true
	s := Spec{ID: "idle", ListenPort: freePort(t), Network: "tcp", TargetHost: "127.0.0.1", TargetPort: l.Addr().(*net.TCPAddr).Port}
	if e := m.Apply(context.Background(), []Spec{s}); e != nil {
		t.Fatal(e)
	}
	exchange(t, tcpClient(t, s.ListenPort), "ping")
	done := make(chan struct{})
	go func() { m.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("close must close upstream sockets even if peer ignores half-close")
	}
}

func TestTCPAndUDPSessionCaps(t *testing.T) {
	h, p := tcpEcho(t, "")
	_, udpPort := udpEcho(t, "")
	m := testManager(t)
	s := Spec{ID: "cap", ListenPort: freePort(t), Network: "tcp", TargetHost: h, TargetPort: p}
	if e := m.Apply(context.Background(), []Spec{s}); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < MaxTCPConnections; i++ {
		exchange(t, tcpClient(t, s.ListenPort), "ping")
	}
	waitFor(t, func() bool { return m.Status()[s.ID].TCPConnections == MaxTCPConnections })
	extra := tcpClient(t, s.ListenPort)
	extra.SetReadDeadline(time.Now().Add(time.Second))
	if _, e := extra.Read(make([]byte, 1)); e == nil {
		t.Fatal("excess TCP client was not rejected")
	}
	if m.Status()[s.ID].TCPConnections > MaxTCPConnections {
		t.Fatal("TCP cap exceeded")
	}
	s.Network = "udp"
	s.TargetPort = udpPort
	if e := m.Apply(context.Background(), []Spec{s}); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < MaxUDPSessions; i++ {
		c, e := net.Dial("udp", net.JoinHostPort("127.0.0.1", strconv.Itoa(s.ListenPort)))
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() { c.Close() })
		exchange(t, c, "ping")
	}
	extraUDP, e := net.Dial("udp", net.JoinHostPort("127.0.0.1", strconv.Itoa(s.ListenPort)))
	if e != nil {
		t.Fatal(e)
	}
	defer extraUDP.Close()
	extraUDP.SetDeadline(time.Now().Add(30 * time.Millisecond))
	extraUDP.Write([]byte("drop"))
	if _, e := extraUDP.Read(make([]byte, 16)); e == nil {
		t.Fatal("excess UDP session was accepted")
	}
	if m.Status()[s.ID].UDPSessions != MaxUDPSessions {
		t.Fatal("UDP cap violated")
	}
}

func TestDisableClosesListenersEvenIfStateDiskFails(t *testing.T) {
	h, p := tcpEcho(t, "")
	dir := t.TempDir()
	m := NewManager(dir)
	m.allowPrivate = true
	defer m.Close()
	s := Spec{ID: "disk", ListenPort: freePort(t), Network: "both", TargetHost: h, TargetPort: p}
	if e := m.Apply(context.Background(), []Spec{s}); e != nil {
		t.Fatal(e)
	}
	if e := os.Rename(dir, dir+"-saved"); e != nil {
		t.Fatal(e)
	}
	defer os.Rename(dir+"-saved", dir)
	if e := os.WriteFile(dir, []byte("unavailable directory"), 0600); e != nil {
		t.Fatal(e)
	}
	defer os.Remove(dir)
	if e := m.Apply(context.Background(), nil); e == nil {
		t.Fatal("expected persistence failure to be reported")
	}
	if st := m.Status()[s.ID]; st.Running || st.Error == "" {
		t.Fatalf("disable did not retain persistence failure: %+v", st)
	}
}
func TestFailedBatchReleasesStagedListeners(t *testing.T) {
	h, p := tcpEcho(t, "")
	m := testManager(t)
	old := Spec{ID: "old", ListenPort: freePort(t), Network: "tcp", TargetHost: h, TargetPort: p}
	if e := m.Apply(context.Background(), []Spec{old}); e != nil {
		t.Fatal(e)
	}
	next := old
	next.ID = "new"
	next.ListenPort = freePort(t)
	bad := next
	bad.ID = "bad"
	bad.TargetHost = ""
	bad.ListenPort = freePort(t)
	if e := m.Apply(context.Background(), []Spec{old, next, bad}); e == nil {
		t.Fatal("bad batch accepted")
	}
	exchange(t, tcpClient(t, old.ListenPort), "ping")
	l, e := net.Listen("tcp", net.JoinHostPort("", strconv.Itoa(next.ListenPort)))
	if e != nil {
		t.Fatalf("staged listener leaked: %v", e)
	}
	l.Close()
}

func TestErrorStatusesHaveSessionAndBoundedText(t *testing.T) {
	m := NewManager(t.TempDir())
	defer m.Close()
	m.resolve = func(context.Context, string) ([]netip.Addr, error) {
		return nil, errors.New(strings.Repeat("long error", 100))
	}
	s := Spec{ID: "error", ListenPort: freePort(t), Network: "tcp", TargetHost: "target.example", TargetPort: 443}
	if e := m.Apply(context.Background(), []Spec{s}); e == nil {
		t.Fatal("expected resolver failure")
	}
	st := m.Status()[s.ID]
	if st.Session == "" || len(st.Error) > 500 || st.Running {
		t.Fatalf("invalid error status: %+v", st)
	}
}

func TestDeletedRuleStatusHistoryIsBounded(t *testing.T) {
	h, p := tcpEcho(t, "")
	m := testManager(t)
	port := freePort(t)
	for i := 0; i < MaxRules+2; i++ {
		s := Spec{ID: strconv.Itoa(i), ListenPort: port, Network: "tcp", TargetHost: h, TargetPort: p}
		if e := m.Apply(context.Background(), []Spec{s}); e != nil {
			t.Fatal(e)
		}
		if e := m.Apply(context.Background(), nil); e != nil {
			t.Fatal(e)
		}
	}
	statuses := m.Status()
	if len(statuses) > MaxRules {
		t.Fatal("deleted rule history grew without limit")
	}
	for _, s := range statuses {
		if s.Session == "" || s.Running {
			t.Fatal("invalid deleted-rule status")
		}
	}
	s := Spec{ID: "last", ListenPort: port, Network: "tcp", TargetHost: h, TargetPort: p}
	if e := m.Apply(context.Background(), []Spec{s}); e != nil {
		t.Fatal(e)
	}
	m.Close()
	if len(m.Status()) > MaxRules {
		t.Fatal("closing manager exceeded the history bound")
	}
}

func TestTCPHalfCloseDrainsBothDirections(t *testing.T) {
	for _, first := range []string{"client", "upstream"} {
		t.Run(first, func(t *testing.T) {
			l, e := net.Listen("tcp", "127.0.0.1:0")
			if e != nil {
				t.Fatal(e)
			}
			defer l.Close()
			payload := strings.Repeat("half-close upload\n", 16384)
			response := strings.Repeat("half-close response\n", 16384)
			result := make(chan error, 1)
			go func() {
				c, e := l.Accept()
				if e != nil {
					result <- e
					return
				}
				defer c.Close()
				c.SetDeadline(time.Now().Add(3 * time.Second))
				if first == "upstream" {
					if e = c.(*net.TCPConn).CloseWrite(); e != nil {
						result <- e
						return
					}
				}
				b, e := io.ReadAll(c)
				if e == nil && string(b) != payload {
					e = errors.New("upstream received a truncated upload")
				}
				if e == nil && first == "client" {
					_, e = io.WriteString(c, response)
				}
				result <- e
			}()
			m := testManager(t)
			s := Spec{ID: "half-close", ListenPort: freePort(t), Network: "tcp", TargetHost: "127.0.0.1", TargetPort: l.Addr().(*net.TCPAddr).Port}
			if e := m.Apply(context.Background(), []Spec{s}); e != nil {
				t.Fatal(e)
			}
			c := tcpClient(t, s.ListenPort).(*net.TCPConn)
			c.SetDeadline(time.Now().Add(3 * time.Second))
			if first == "upstream" {
				if b, e := io.ReadAll(c); e != nil || len(b) != 0 {
					t.Fatalf("upstream FIN: %q, %v", b, e)
				}
			}
			if _, e := io.WriteString(c, payload); e != nil {
				t.Fatal(e)
			}
			if e := c.CloseWrite(); e != nil {
				t.Fatal(e)
			}
			if first == "client" {
				if b, e := io.ReadAll(c); e != nil || string(b) != response {
					t.Fatalf("response was truncated: %d bytes, %v", len(b), e)
				}
			}
			select {
			case e := <-result:
				if e != nil {
					t.Fatal(e)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("half-closed stream did not drain")
			}
			waitFor(t, func() bool { return m.Status()[s.ID].TCPConnections == 0 })
		})
	}
}

func TestTCPActivityKeepsSilentDirectionAlive(t *testing.T) {
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	go func() {
		c, e := l.Accept()
		if e != nil {
			return
		}
		defer c.Close()
		c.SetDeadline(time.Now().Add(3 * time.Second))
		if b, e := io.ReadAll(c); e == nil {
			c.Write(b)
		}
	}()
	m := testManager(t)
	m.tcpIdle = 200 * time.Millisecond
	s := Spec{ID: "upload", ListenPort: freePort(t), Network: "tcp", TargetHost: "127.0.0.1", TargetPort: l.Addr().(*net.TCPAddr).Port}
	if e := m.Apply(context.Background(), []Spec{s}); e != nil {
		t.Fatal(e)
	}
	c := tcpClient(t, s.ListenPort).(*net.TCPConn)
	c.SetDeadline(time.Now().Add(3 * time.Second))
	var sent string
	for until := time.Now().Add(3 * m.tcpIdle); time.Now().Before(until); {
		if _, e := io.WriteString(c, "upload"); e != nil {
			t.Fatalf("active one-way upload was closed: %v", e)
		}
		sent += "upload"
		time.Sleep(10 * time.Millisecond)
	}
	if e := c.CloseWrite(); e != nil {
		t.Fatal(e)
	}
	if b, e := io.ReadAll(c); e != nil || string(b) != sent {
		t.Fatalf("active upload lost data: %d of %d bytes, %v", len(b), len(sent), e)
	}
	// Truly idle connections must still release both copy goroutines.
	tcpClient(t, s.ListenPort)
	waitFor(t, func() bool { return m.Status()[s.ID].TCPConnections == 1 })
	waitFor(t, func() bool { return m.Status()[s.ID].TCPConnections == 0 })
}

func TestRestoreDoesNotRewriteState(t *testing.T) {
	h, p := tcpEcho(t, "")
	m := testManager(t)
	s := Spec{ID: "restore", ListenPort: freePort(t), Network: "tcp", TargetHost: h, TargetPort: p}
	if e := m.Apply(context.Background(), []Spec{s}); e != nil {
		t.Fatal(e)
	}
	m.Close()
	state := filepath.Join(m.dir, "relay-state.json")
	before, e := os.Stat(state)
	if e != nil {
		t.Fatal(e)
	}
	restored := NewManager(m.dir)
	restored.allowPrivate = true
	defer restored.Close()
	if e := restored.Restore(context.Background()); e != nil {
		t.Fatal(e)
	}
	after, e := os.Stat(state)
	if e != nil {
		t.Fatal(e)
	}
	if !os.SameFile(before, after) {
		t.Fatal("restore rewrote already-persisted state")
	}
	exchange(t, tcpClient(t, s.ListenPort), "ping")
}

func TestRestoreRejectsTrailingAndOversizedState(t *testing.T) {
	for name, raw := range map[string]string{"trailing": "null\n{}", "oversized": "null" + strings.Repeat(" ", 1<<20)} {
		t.Run(name, func(t *testing.T) {
			h, p := tcpEcho(t, "")
			m := testManager(t)
			s := Spec{ID: "retain", ListenPort: freePort(t), Network: "tcp", TargetHost: h, TargetPort: p}
			if e := m.Apply(context.Background(), []Spec{s}); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(filepath.Join(m.dir, "relay-state.json"), []byte(raw), 0600); e != nil {
				t.Fatal(e)
			}
			if e := m.Restore(context.Background()); e == nil {
				t.Fatal("restored incomplete/corrupt state")
			}
			exchange(t, tcpClient(t, s.ListenPort), "ping")
		})
	}
}

func TestPersistenceFailureRetainsRuleAndErrorStatus(t *testing.T) {
	h, p := tcpEcho(t, "a")
	_, p2 := tcpEcho(t, "b")
	m := testManager(t)
	s := Spec{ID: "persist", ListenPort: freePort(t), Network: "tcp", TargetHost: h, TargetPort: p}
	if e := m.Apply(context.Background(), []Spec{s}); e != nil {
		t.Fatal(e)
	}
	dir := m.dir
	m.dir = filepath.Join(t.TempDir(), "not-a-directory")
	if e := os.WriteFile(m.dir, nil, 0600); e != nil {
		t.Fatal(e)
	}
	next := s
	next.TargetPort = p2
	added := next
	added.ID, added.ListenPort = "new", freePort(t)
	if e := m.Apply(context.Background(), []Spec{next, added}); e == nil {
		t.Fatal("expected persistence failure")
	}
	st, newStatus := m.Status()[s.ID], m.Status()[added.ID]
	if !st.Running || st.Revision != s.Rev() || st.Error == "" || newStatus.Running || newStatus.Session == "" || newStatus.Error == "" {
		t.Fatalf("persistence failure lost status: old=%+v new=%+v", st, newStatus)
	}
	exchange(t, tcpClient(t, s.ListenPort), "aping")
	m.dir = dir
	if e := m.Apply(context.Background(), []Spec{s}); e != nil {
		t.Fatal(e)
	}
	if m.Status()[s.ID].Error != "" {
		t.Fatal("successful apply did not clear configuration error")
	}
	restored := NewManager(dir)
	restored.allowPrivate = true
	m.Close()
	defer restored.Close()
	if e := restored.Restore(context.Background()); e != nil {
		t.Fatal(e)
	}
	exchange(t, tcpClient(t, s.ListenPort), "aping")
}

func TestCloseCancelsInFlightValidation(t *testing.T) {
	h, p := tcpEcho(t, "")
	m := testManager(t)
	s := Spec{ID: "active", ListenPort: freePort(t), Network: "tcp", TargetHost: h, TargetPort: p}
	if e := m.Apply(context.Background(), []Spec{s}); e != nil {
		t.Fatal(e)
	}
	exchange(t, tcpClient(t, s.ListenPort), "ping")
	started := make(chan struct{})
	m.resolve = func(ctx context.Context, _ string) ([]netip.Addr, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	next := s
	next.ID, next.ListenPort, next.TargetHost = "lookup", freePort(t), "lookup.example"
	applied := make(chan error, 1)
	go func() { applied <- m.Apply(context.Background(), []Spec{s, next}) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("validation did not start")
	}
	closed := make(chan struct{})
	go func() { m.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("Close waited for DNS timeout instead of canceling validation")
	}
	if e := <-applied; !errors.Is(e, context.Canceled) {
		t.Fatalf("apply returned %v, want cancellation", e)
	}
	if st := m.Status()[s.ID]; st.Running || st.TCPConnections != 0 {
		t.Fatalf("Close retained resources: %+v", st)
	}
}

type acceptErrorListener struct {
	net.Listener
	err error
}

func (l *acceptErrorListener) Accept() (net.Conn, error) {
	if l.err != nil {
		e := l.err
		l.err = nil
		return nil, e
	}
	return l.Listener.Accept()
}

func TestTCPAcceptRetriesTemporaryResourceError(t *testing.T) {
	h, p := tcpEcho(t, "")
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	wrapper := &acceptErrorListener{Listener: l, err: &net.OpError{Op: "accept", Net: "tcp", Err: syscall.EMFILE}}
	ctx, cancel := context.WithCancel(context.Background())
	r := &rule{tcp: wrapper, target: target{address: net.JoinHostPort(h, strconv.Itoa(p))}, connections: map[net.Conn]net.Conn{}, sessions: map[string]*udpSession{}, tcpIdle: tcpIdle, dialCtx: ctx, cancelDial: cancel}
	t.Cleanup(func() { r.stop(true) })
	r.wg.Add(1)
	go r.accept(wrapper)
	exchange(t, tcpClient(t, l.Addr().(*net.TCPAddr).Port), "ping")
	if st := r.status(); !st.Running || st.Error != "" {
		t.Fatalf("temporary accept error was not recovered: %+v", st)
	}
}

func TestFailedListenersCanBeReapplied(t *testing.T) {
	for _, network := range []string{"tcp", "udp"} {
		t.Run(network, func(t *testing.T) {
			echo := tcpEcho
			if network == "udp" {
				echo = udpEcho
			}
			h, p := echo(t, "")
			m := testManager(t)
			s := Spec{ID: "listener", ListenPort: freePort(t), Network: network, TargetHost: h, TargetPort: p}
			if e := m.Apply(context.Background(), []Spec{s}); e != nil {
				t.Fatal(e)
			}
			r := m.rules[s.ID]
			r.mu.Lock()
			if network == "tcp" {
				r.tcp.Close()
			} else {
				r.udp.conn.Close()
			}
			r.mu.Unlock()
			waitFor(t, func() bool { st := m.Status()[s.ID]; return !st.Running && st.Error != "" })
			if e := m.Apply(context.Background(), []Spec{s}); e != nil {
				t.Fatal(e)
			}
			c, e := net.Dial(network, net.JoinHostPort("127.0.0.1", strconv.Itoa(s.ListenPort)))
			if e != nil {
				t.Fatal(e)
			}
			defer c.Close()
			exchange(t, c, "ping")
			if st := m.Status()[s.ID]; !st.Running || st.Error != "" {
				t.Fatalf("listener did not recover: %+v", st)
			}
		})
	}
}

func TestTCPDialErrorSurvivesUnchangedApply(t *testing.T) {
	m := testManager(t)
	s := Spec{ID: "dial", ListenPort: freePort(t), Network: "tcp", TargetHost: "127.0.0.1", TargetPort: freePort(t)}
	if e := m.Apply(context.Background(), []Spec{s}); e != nil {
		t.Fatal(e)
	}
	c := tcpClient(t, s.ListenPort)
	c.SetReadDeadline(time.Now().Add(time.Second))
	if _, e := c.Read(make([]byte, 1)); e == nil {
		t.Fatal("connection to missing target was not closed")
	}
	waitFor(t, func() bool { return m.Status()[s.ID].Error != "" })
	if e := m.Apply(context.Background(), []Spec{s}); e != nil {
		t.Fatal(e)
	}
	if m.Status()[s.ID].Error == "" {
		t.Fatal("unchanged apply hid the upstream dial failure")
	}
	l, e := net.Listen("tcp", net.JoinHostPort(s.TargetHost, strconv.Itoa(s.TargetPort)))
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	go func() {
		c, e := l.Accept()
		if e == nil {
			defer c.Close()
			io.Copy(c, c)
		}
	}()
	exchange(t, tcpClient(t, s.ListenPort), "ping")
	if m.Status()[s.ID].Error != "" {
		t.Fatal("successful upstream dial did not clear the error")
	}
}

func TestInvalidIDsAreNotRetainedInHistory(t *testing.T) {
	m := testManager(t)
	s := Spec{ID: strings.Repeat("x", 1<<20), ListenPort: freePort(t), Network: "tcp", TargetHost: "127.0.0.1", TargetPort: 443}
	if e := m.Apply(context.Background(), []Spec{s}); e == nil {
		t.Fatal("expected invalid ID")
	}
	if len(m.Status()) != 0 {
		t.Fatal("invalid oversized ID was retained in status history")
	}
}

func TestPartialDisableSurvivesRejectedUpdate(t *testing.T) {
	for _, failure := range []string{"validation", "persistence"} {
		t.Run(failure, func(t *testing.T) {
			h, p := tcpEcho(t, "")
			m := testManager(t)
			keep := Spec{ID: "keep", ListenPort: freePort(t), Network: "tcp", TargetHost: h, TargetPort: p}
			remove := keep
			remove.ID, remove.ListenPort = "remove", freePort(t)
			if e := m.Apply(context.Background(), []Spec{keep, remove}); e != nil {
				t.Fatal(e)
			}
			exchange(t, tcpClient(t, remove.ListenPort), "ping")
			dir := m.dir
			next := keep
			if failure == "validation" {
				next.TargetHost = ""
			} else {
				m.dir = filepath.Join(t.TempDir(), "not-a-directory")
				if e := os.WriteFile(m.dir, nil, 0600); e != nil {
					t.Fatal(e)
				}
			}
			if e := m.Apply(context.Background(), []Spec{next}); e == nil {
				t.Fatal("expected rejected update")
			}
			st := m.Status()[remove.ID]
			if st.Running || st.TCPConnections != 0 {
				t.Fatalf("unrelated update failure prevented removal: %+v", st)
			}
			if failure == "persistence" && st.Error == "" {
				t.Fatal("removed rule lost persistence error")
			}
			exchange(t, tcpClient(t, keep.ListenPort), "ping")
			if failure == "validation" {
				m.Close()
				restored := NewManager(dir)
				restored.allowPrivate = true
				defer restored.Close()
				if e := restored.Restore(context.Background()); e != nil {
					t.Fatal(e)
				}
				if len(restored.Status()) != 1 || !restored.Status()[keep.ID].Running {
					t.Fatalf("rejected update resurrected removed rule: %+v", restored.Status())
				}
				exchange(t, tcpClient(t, keep.ListenPort), "ping")
			}
		})
	}
}

func TestErrorHistoryRetainsMostRecentFailures(t *testing.T) {
	m := testManager(t)
	const extra = 8
	for i := 0; i < MaxRules+extra; i++ {
		s := Spec{ID: strconv.Itoa(i), ListenPort: 10000, Network: "tcp", TargetHost: "", TargetPort: 443}
		if e := m.Apply(context.Background(), []Spec{s}); e == nil {
			t.Fatal("expected invalid target")
		}
	}
	statuses := m.Status()
	if len(statuses) != MaxRules {
		t.Fatalf("unexpected history size %d", len(statuses))
	}
	for i := extra; i < MaxRules+extra; i++ {
		if st, ok := statuses[strconv.Itoa(i)]; !ok || st.Error == "" {
			t.Fatalf("recent failure %d was discarded: %+v", i, st)
		}
	}
}

func TestUDPReplacementDoesNotInheritRetiredListenerSession(t *testing.T) {
	h, oldPort := udpEcho(t, "old")
	_, newPort := udpEcho(t, "new")
	m := testManager(t)
	s := Spec{ID: "replace", ListenPort: freePort(t), Network: "udp", TargetHost: h, TargetPort: newPort}
	if e := m.Apply(context.Background(), []Spec{s}); e != nil {
		t.Fatal(e)
	}
	c, e := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.ParseIP(h), Port: s.ListenPort})
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	oldConn, e := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.ParseIP(h), Port: oldPort})
	if e != nil {
		t.Fatal(e)
	}
	defer oldConn.Close()
	oldSocket, e := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP(h)})
	if e != nil {
		t.Fatal(e)
	}
	defer oldSocket.Close()
	r := m.rules[s.ID]
	retired := &udpListener{conn: oldSocket, rule: r}
	key := c.LocalAddr().String()
	// Reproduce the interval after a failed listener is replaced but before its
	// cleanup has acquired r.mu to remove its old client sessions.
	r.mu.Lock()
	r.sessions[key] = &udpSession{conn: oldConn, listener: retired, client: c.LocalAddr().(*net.UDPAddr)}
	r.mu.Unlock()
	exchange(t, c, "newping")
	retired.close()
	exchange(t, c, "newping")
	if m.Status()[s.ID].UDPSessions != 1 {
		t.Fatal("retired listener cleanup removed the replacement's session")
	}
}

func TestUDPUpstreamErrorSurvivesUnchangedApply(t *testing.T) {
	backend, e := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if e != nil {
		t.Fatal(e)
	}
	port := backend.LocalAddr().(*net.UDPAddr).Port
	backend.Close()
	m := testManager(t)
	s := Spec{ID: "udp-error", ListenPort: freePort(t), Network: "udp", TargetHost: "127.0.0.1", TargetPort: port}
	if e := m.Apply(context.Background(), []Spec{s}); e != nil {
		t.Fatal(e)
	}
	c, e := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.ParseIP(s.TargetHost), Port: s.ListenPort})
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	if _, e := c.Write([]byte("ping")); e != nil {
		t.Fatal(e)
	}
	waitFor(t, func() bool { st := m.Status()[s.ID]; return st.Error != "" && st.UDPSessions == 0 })
	if e := m.Apply(context.Background(), []Spec{s}); e != nil {
		t.Fatal(e)
	}
	if m.Status()[s.ID].Error == "" {
		t.Fatal("unchanged apply hid the upstream UDP failure")
	}
	backend, e = net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP(s.TargetHost), Port: port})
	if e != nil {
		t.Fatal(e)
	}
	defer backend.Close()
	go func() {
		b := make([]byte, 16)
		n, client, e := backend.ReadFromUDP(b)
		if e == nil {
			backend.WriteToUDP(b[:n], client)
		}
	}()
	exchange(t, c, "ping")
	waitFor(t, func() bool { return m.Status()[s.ID].Error == "" })
}

func TestRestoreRecoversHealthyRulesWhenAnotherRuleFails(t *testing.T) {
	for _, failure := range []string{"dns", "udp-bind"} {
		t.Run(failure, func(t *testing.T) {
			h, p := tcpEcho(t, "")
			m := testManager(t)
			bad := Spec{ID: "bad", ListenPort: freePort(t), Network: "tcp", TargetHost: h, TargetPort: p}
			if failure == "dns" {
				bad.TargetHost = "restore.example"
				m.resolve = func(context.Context, string) ([]netip.Addr, error) {
					return []netip.Addr{netip.MustParseAddr(h)}, nil
				}
			} else {
				bad.Network = "both"
			}
			good := Spec{ID: "good", ListenPort: freePort(t), Network: "tcp", TargetHost: h, TargetPort: p}
			if e := m.Apply(context.Background(), []Spec{bad, good}); e != nil {
				t.Fatal(e)
			}
			m.Close()
			state := filepath.Join(m.dir, "relay-state.json")
			before, e := os.ReadFile(state)
			if e != nil {
				t.Fatal(e)
			}
			restored := NewManager(m.dir)
			restored.allowPrivate = true
			defer restored.Close()
			recovered := false
			restored.resolve = func(context.Context, string) ([]netip.Addr, error) {
				if !recovered {
					return nil, errors.New("temporary DNS failure")
				}
				return []netip.Addr{netip.MustParseAddr(h)}, nil
			}
			var occupied *net.UDPConn
			if failure == "udp-bind" {
				occupied, e = net.ListenUDP("udp", &net.UDPAddr{Port: bad.ListenPort})
				if e != nil {
					t.Fatal(e)
				}
				defer occupied.Close()
			}
			if e := restored.Restore(context.Background()); e == nil {
				t.Fatal("expected partial restore error")
			}
			if st := restored.Status()[bad.ID]; st.Running || st.Error == "" || st.Session == "" {
				t.Fatalf("failed rule lost its error status: %+v", st)
			}
			c := tcpClient(t, good.ListenPort)
			exchange(t, c, "ping")
			if failure == "udp-bind" {
				l, e := net.Listen("tcp", net.JoinHostPort("", strconv.Itoa(bad.ListenPort)))
				if e != nil {
					t.Fatalf("partial restore leaked staged TCP listener: %v", e)
				}
				l.Close()
				occupied.Close()
			}
			recovered = true
			if e := restored.Restore(context.Background()); e != nil {
				t.Fatal(e)
			}
			exchange(t, c, "ping")
			if st := restored.Status()[bad.ID]; !st.Running || st.Error != "" {
				t.Fatalf("failed rule could not recover: %+v", st)
			}
			after, e := os.ReadFile(state)
			if e != nil {
				t.Fatal(e)
			}
			if string(before) != string(after) {
				t.Fatal("partial restore overwrote the desired persisted configuration")
			}
		})
	}
}
