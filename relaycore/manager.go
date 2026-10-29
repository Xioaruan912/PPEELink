// Package relaycore forwards opaque TCP streams and UDP datagrams to a fixed,
// validated public endpoint. Its counters belong to a line, never a user/site.
package relaycore

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

const (
	MaxRules          = 64
	MaxTCPConnections = 128
	MaxUDPSessions    = 256
	maxPacketSize     = 65535
	tcpIdle           = 5 * time.Minute
)

type Spec struct {
	ID         string `json:"id"`
	ListenPort int    `json:"listenPort"`
	Network    string `json:"network"`
	TargetHost string `json:"targetHost"`
	TargetPort int    `json:"targetPort"`
}

// Rev identifies the intended relay configuration, independently of Xray.
func (s Spec) Rev() string {
	raw, _ := json.Marshal(s)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:8])
}

type Status struct {
	Session        string `json:"session"`
	Running        bool   `json:"running"`
	Error          string `json:"error"`
	Up             int64  `json:"up"`
	Down           int64  `json:"down"`
	TCPConnections int    `json:"tcpConnections"`
	UDPSessions    int    `json:"udpSessions"`
	Revision       string `json:"revision"`
}

type target struct {
	address  string
	udp      *net.UDPAddr
	revision string
}
type udpListener struct {
	conn *net.UDPConn
	rule *rule
}
type udpSession struct {
	conn     *net.UDPConn
	listener *udpListener
	client   *net.UDPAddr
	revision string
}
type rule struct {
	mu          sync.Mutex
	spec        Spec
	target      target
	session     string
	err         string
	tcpErr      string
	udpErr      string
	up, down    int64
	tcp         net.Listener
	udp         *udpListener
	connections map[net.Conn]net.Conn
	sessions    map[string]*udpSession
	wg          sync.WaitGroup
	udpIdle     time.Duration
	tcpIdle     time.Duration
	stopped     bool
	dialCtx     context.Context
	cancelDial  context.CancelFunc
}

type Manager struct {
	mu           sync.Mutex
	dir          string
	rules        map[string]*rule
	history      map[string]Status
	historyOrder []string
	closed       bool
	allowPrivate bool // package-private test hook, never accepted in JSON/config
	udpIdle      time.Duration
	tcpIdle      time.Duration
	resolve      func(context.Context, string) ([]netip.Addr, error)
	ctx          context.Context
	cancel       context.CancelFunc
}

var sessionSequence atomic.Uint64

func uniqueSession() string {
	var id [16]byte
	if _, err := rand.Read(id[:]); err == nil {
		return hex.EncodeToString(id[:])
	}
	// Sessions are counter epochs, not authentication tokens.
	return fmt.Sprintf("%x-%x", time.Now().UnixNano(), sessionSequence.Add(1))
}
func boundedError(err error) string {
	s := err.Error()
	if len(s) <= 500 {
		return s
	}
	n := 500
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
func NewManager(dir string) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{dir: dir, rules: map[string]*rule{}, history: map[string]Status{}, udpIdle: 2 * time.Minute, tcpIdle: tcpIdle, ctx: ctx, cancel: cancel,
		resolve: func(ctx context.Context, host string) ([]netip.Addr, error) {
			return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		}}
}

// Restore starts independently recoverable rules from the persisted configuration
// and reports failures without rewriting that configuration. DNS names are
// validated again on restart; live sockets always dial a pinned numeric address.
func (m *Manager) Restore(ctx context.Context) error {
	f, e := os.Open(filepath.Join(m.dir, "relay-state.json"))
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	defer f.Close()
	raw, e := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if e != nil {
		return fmt.Errorf("restore relay state: %w", e)
	}
	if len(raw) > 1<<20 {
		return errors.New("restore relay state exceeds 1 MiB")
	}
	var specs []Spec
	if e = json.Unmarshal(raw, &specs); e != nil {
		return fmt.Errorf("restore relay state: %w", e)
	}
	// Restoring already-durable state must also work while the disk is read-only
	// or full. Only a new desired configuration needs another durable write.
	return m.apply(ctx, specs, false)
}

func (m *Manager) validate(ctx context.Context, s Spec) (target, error) {
	if strings.TrimSpace(s.ID) == "" || len(s.ID) > 128 {
		return target{}, errors.New("invalid relay ID")
	}
	if s.ListenPort < 1 || s.ListenPort > 65535 || s.TargetPort < 1 || s.TargetPort > 65535 {
		return target{}, errors.New("invalid relay port")
	}
	if s.Network != "tcp" && s.Network != "udp" && s.Network != "both" {
		return target{}, errors.New("network must be tcp, udp or both")
	}
	host := strings.TrimSpace(s.TargetHost)
	if host == "" || len(host) > 253 || strings.ContainsAny(host, " /\\%[]\x00\r\n\t") {
		return target{}, errors.New("invalid target host")
	}
	var ips []netip.Addr
	if ip, e := netip.ParseAddr(host); e == nil {
		ips = []netip.Addr{ip}
	} else {
		lookupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		var e error
		ips, e = m.resolve(lookupCtx, host)
		if e != nil {
			return target{}, fmt.Errorf("resolve target: %w", e)
		}
	}
	if len(ips) == 0 {
		return target{}, errors.New("target has no addresses")
	}
	for _, ip := range ips {
		if !ip.IsValid() || (!m.allowPrivate && (!publicIP(ip) || localIP(ip))) {
			return target{}, fmt.Errorf("target is not a remote public address: %s", ip)
		}
	}
	// Validate every DNS answer, then pin one. Dialing the hostname again would
	// permit DNS rebinding between validation and connection establishment.
	ip := ips[0].Unmap()
	return target{address: net.JoinHostPort(ip.String(), strconv.Itoa(s.TargetPort)), udp: &net.UDPAddr{IP: net.IP(ip.AsSlice()), Port: s.TargetPort}, revision: s.Rev()}, nil
}

var forbiddenPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"), netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"), netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"), netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("240.0.0.0/4"), netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("2001::/23"), netip.MustParsePrefix("64:ff9b::/96"), netip.MustParsePrefix("64:ff9b:1::/48"), netip.MustParsePrefix("2002::/16"),
}

func publicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, p := range forbiddenPrefixes {
		if p.Contains(ip) {
			return false
		}
	}
	return true
}
func localIP(ip netip.Addr) bool {
	addrs, e := net.InterfaceAddrs()
	if e != nil {
		return true
	}
	for _, a := range addrs {
		p, e := netip.ParsePrefix(a.String())
		if e == nil && p.Addr().Unmap() == ip.Unmap() {
			return true
		}
	}
	return false
}
func wantsTCP(s Spec) bool { return s.Network == "tcp" || s.Network == "both" }
func wantsUDP(s Spec) bool { return s.Network == "udp" || s.Network == "both" }

// Apply stops removed rules immediately, independently of errors in other rules.
// Edits stage validation and new binds before touching working listeners.
// Target edits affect new TCP connections/UDP sessions; existing sessions drain.
func (m *Manager) Apply(ctx context.Context, specs []Spec) error {
	return m.apply(ctx, specs, true)
}

func (m *Manager) apply(ctx context.Context, specs []Spec, persist bool) error {
	ctx, cancel := context.WithCancel(ctx)
	stopCancel := context.AfterFunc(m.ctx, cancel)
	defer stopCancel()
	defer cancel()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("relay manager is closed")
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	if len(specs) > MaxRules {
		return fmt.Errorf("at most %d relay rules are allowed", MaxRules)
	}
	ids := make(map[string]bool, len(specs))
	for _, s := range specs {
		var e error
		if len(s.ID) > 128 || strings.TrimSpace(s.ID) == "" {
			e = errors.New("invalid relay ID")
		} else if ids[s.ID] {
			e = errors.New("duplicate relay ID")
		}
		if e != nil {
			m.recordError(s.ID, e)
			return fmt.Errorf("relay %s: %w", s.ID, e)
		}
		ids[s.ID] = true
	}
	var removed []string
	for id, r := range m.rules {
		if !ids[id] {
			r.stop(true)
			m.rememberStatus(id, r.status())
			delete(m.rules, id)
			removed = append(removed, id)
		}
	}
	if len(removed) != 0 {
		if persist {
			// Persist the surviving last-good rules before attempting edits. A
			// rejected edit must not resurrect a deleted/paused rule on restart.
			remaining := make([]Spec, 0, len(m.rules))
			for _, r := range m.rules {
				remaining = append(remaining, r.spec)
			}
			if e := m.persist(remaining); e != nil {
				e = fmt.Errorf("persist relay removals: %w", e)
				for _, id := range removed {
					m.recordError(id, e)
				}
				return e
			}
		}
		if len(specs) == 0 {
			return nil
		}
	}
	type staged struct {
		s   Spec
		t   target
		r   *rule
		tcp net.Listener
		udp *udpListener
		new bool
	}
	stagedRules := make([]staged, 0, len(specs))
	ports := map[string]bool{}
	cleanupRule := func(v staged) {
		if v.tcp != nil {
			v.tcp.Close()
		}
		if v.udp != nil {
			v.udp.conn.Close()
		}
		if v.new {
			v.r.cancelDial()
		}
	}
	cleanup := func() {
		for _, v := range stagedRules {
			cleanupRule(v)
		}
	}
	var restoreErrors []error
	fail := func(id string, e error) error {
		m.recordError(id, e)
		e = fmt.Errorf("relay %s: %w", id, e)
		if persist {
			cleanup()
			return e
		}
		// Startup should recover other lines when one DNS target or listening
		// port is unavailable. Keep the failed spec on disk for a later retry.
		if i := len(stagedRules) - 1; i >= 0 && stagedRules[i].s.ID == id {
			cleanupRule(stagedRules[i])
			stagedRules = stagedRules[:i]
		}
		restoreErrors = append(restoreErrors, e)
		return nil
	}

stageRules:
	for _, s := range specs {
		for _, network := range []string{"tcp", "udp"} {
			if (network == "tcp" && !wantsTCP(s)) || (network == "udp" && !wantsUDP(s)) {
				continue
			}
			key := network + ":" + strconv.Itoa(s.ListenPort)
			if ports[key] {
				if e := fail(s.ID, errors.New("duplicate listening port")); e != nil {
					return e
				}
				continue stageRules
			}
			ports[key] = true
		}
		r := m.rules[s.ID]
		var t target
		var e error
		var hasTCP, hasUDP, unchanged bool
		var listenPort int
		if r != nil {
			r.mu.Lock()
			unchanged = r.spec == s
			t = r.target
			hasTCP, hasUDP, listenPort = r.tcp != nil, r.udp != nil, r.spec.ListenPort
			r.mu.Unlock()
		}
		if !unchanged {
			t, e = m.validate(ctx, s)
			if e != nil {
				if e = fail(s.ID, e); e != nil {
					return e
				}
				continue
			}
		}
		isNew := r == nil
		if isNew {
			dialCtx, cancelDial := context.WithCancel(m.ctx)
			r = &rule{session: uniqueSession(), connections: map[net.Conn]net.Conn{}, sessions: map[string]*udpSession{}, udpIdle: m.udpIdle, tcpIdle: m.tcpIdle, dialCtx: dialCtx, cancelDial: cancelDial}
		}
		v := staged{s: s, t: t, r: r, new: isNew}
		stagedRules = append(stagedRules, v)
		i := len(stagedRules) - 1
		if wantsTCP(s) && (!hasTCP || listenPort != s.ListenPort) {
			v.tcp, e = net.Listen("tcp", net.JoinHostPort("", strconv.Itoa(s.ListenPort)))
			if e != nil {
				if e = fail(s.ID, e); e != nil {
					return e
				}
				continue
			}
			stagedRules[i].tcp = v.tcp
		}
		if wantsUDP(s) && (!hasUDP || listenPort != s.ListenPort) {
			conn, e := net.ListenUDP("udp", &net.UDPAddr{Port: s.ListenPort})
			if e != nil {
				if e = fail(s.ID, e); e != nil {
					return e
				}
				continue
			}
			stagedRules[i].udp = &udpListener{conn: conn, rule: r}
		}
	}
	if e := ctx.Err(); e != nil {
		cleanup()
		return e
	}
	var persistErr error
	if persist {
		persistErr = m.persist(specs)
	}
	if persistErr != nil {
		persistErr = fmt.Errorf("persist relay state: %w", persistErr)
		for _, s := range specs {
			m.recordError(s.ID, persistErr)
		}
	}
	if persistErr != nil {
		cleanup()
		return persistErr
	}
	for _, v := range stagedRules {
		r := v.r
		r.mu.Lock()
		oldTCP, oldUDP := r.tcp, r.udp
		if r.target.revision != v.t.revision || !wantsTCP(v.s) || v.tcp != nil {
			r.tcpErr = ""
		}
		if r.target.revision != v.t.revision || !wantsUDP(v.s) || v.udp != nil {
			r.udpErr = ""
		}
		if !wantsTCP(v.s) {
			r.tcp = nil
		} else if v.tcp != nil {
			r.tcp = v.tcp
		}
		if !wantsUDP(v.s) {
			r.udp = nil
		} else if v.udp != nil {
			r.udp = v.udp
		}
		r.spec, r.target, r.err = v.s, v.t, ""
		newTCP, newUDP := r.tcp, r.udp
		r.mu.Unlock()
		if oldTCP != nil && oldTCP != newTCP {
			oldTCP.Close()
		}
		if oldUDP != nil && oldUDP != newUDP {
			oldUDP.close()
		}
		m.rules[v.s.ID] = r
		m.forgetStatus(v.s.ID)
		if v.tcp != nil {
			r.wg.Add(1)
			go r.accept(v.tcp)
		}
		if v.udp != nil {
			r.wg.Add(1)
			go v.udp.read()
		}
	}
	m.trimHistory()
	return errors.Join(restoreErrors...)
}

// Caller holds m.mu. Invalid IDs must not bypass the history's memory bound.
func (m *Manager) recordError(id string, e error) {
	if r := m.rules[id]; r != nil {
		r.mu.Lock()
		r.err = boundedError(e)
		r.mu.Unlock()
	} else if len(id) <= 128 && strings.TrimSpace(id) != "" {
		s := m.history[id]
		if s.Session == "" {
			s.Session = uniqueSession()
		}
		s.Error = boundedError(e)
		m.rememberStatus(id, s)
	}
}

func (m *Manager) forgetStatus(id string) {
	delete(m.history, id)
	for i, old := range m.historyOrder {
		if old == id {
			m.historyOrder = append(m.historyOrder[:i], m.historyOrder[i+1:]...)
			break
		}
	}
}

func (m *Manager) rememberStatus(id string, s Status) {
	m.forgetStatus(id)
	m.history[id] = s
	m.historyOrder = append(m.historyOrder, id)
	m.trimHistory()
}

func (m *Manager) trimHistory() {
	for len(m.history) > MaxRules {
		id := m.historyOrder[0]
		m.historyOrder = m.historyOrder[1:]
		delete(m.history, id)
	}
}
func (m *Manager) persist(specs []Spec) error {
	if e := os.MkdirAll(m.dir, 0700); e != nil {
		return e
	}
	raw, e := json.Marshal(specs)
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(m.dir, ".relay-state-*")
	if e != nil {
		return e
	}
	name := f.Name()
	defer os.Remove(name)
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(raw)
	}
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e != nil {
		return e
	}
	if closeErr != nil {
		return closeErr
	}
	if e = os.Rename(name, filepath.Join(m.dir, "relay-state.json")); e != nil {
		return e
	}
	// Sync the rename as well as the file data, especially for durable removals.
	dir, e := os.Open(m.dir)
	if e != nil {
		return e
	}
	e = dir.Sync()
	closeErr = dir.Close()
	if e != nil {
		return e
	}
	return closeErr
}
func (m *Manager) Status() map[string]Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]Status, len(m.rules)+len(m.history))
	for id, s := range m.history {
		out[id] = s
	}
	for id, r := range m.rules {
		out[id] = r.status()
	}
	return out
}
func (r *rule) status() Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	var errs []string
	for _, e := range []string{r.err, r.tcpErr, r.udpErr} {
		if e != "" {
			errs = append(errs, e)
		}
	}
	return Status{Session: r.session, Running: r.tcp != nil || r.udp != nil, Error: boundedError(errors.New(strings.Join(errs, "; "))), Up: r.up, Down: r.down, TCPConnections: len(r.connections), UDPSessions: len(r.sessions), Revision: r.target.revision}
}
func (m *Manager) Close() {
	// Cancel validation/dials before waiting for Apply's configuration lock.
	m.cancel()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return
	}
	m.closed = true
	for id, r := range m.rules {
		r.stop(true)
		m.rememberStatus(id, r.status())
	}
	m.rules = map[string]*rule{}
	m.trimHistory()
}
func (r *rule) stop(all bool) {
	r.mu.Lock()
	tcp, udp := r.tcp, r.udp
	r.tcp, r.udp = nil, nil
	if all {
		r.stopped = true
		if r.cancelDial != nil {
			r.cancelDial()
		}
		for c, upstream := range r.connections {
			c.Close()
			if upstream != nil {
				upstream.Close()
			}
		}
		for _, s := range r.sessions {
			s.conn.Close()
		}
	}
	r.mu.Unlock()
	if tcp != nil {
		tcp.Close()
	}
	if udp != nil {
		udp.close()
	}
	if all {
		r.wg.Wait()
	}
}
func (u *udpListener) close() {
	u.conn.Close()
	r := u.rule
	r.mu.Lock()
	for key, s := range r.sessions {
		if s.listener == u {
			s.conn.Close()
			delete(r.sessions, key)
		}
	}
	r.mu.Unlock()
}

func (r *rule) accept(l net.Listener) {
	defer r.wg.Done()
	var retryDelay time.Duration
	for {
		c, e := l.Accept()
		if e != nil {
			r.mu.Lock()
			if r.tcp != l {
				r.mu.Unlock()
				return
			}
			r.tcpErr = boundedError(e)
			if temporary, ok := e.(net.Error); ok && temporary.Temporary() {
				r.mu.Unlock()
				if retryDelay == 0 {
					retryDelay = 5 * time.Millisecond
				} else {
					retryDelay = min(retryDelay*2, time.Second)
				}
				timer := time.NewTimer(retryDelay)
				select {
				case <-timer.C:
				case <-r.dialCtx.Done():
					timer.Stop()
					return
				}
				continue
			}
			r.tcp = nil
			r.mu.Unlock()
			l.Close()
			return
		}
		retryDelay = 0
		r.mu.Lock()
		if len(r.connections) >= MaxTCPConnections || r.tcp != l {
			r.mu.Unlock()
			c.Close()
			continue
		}
		r.connections[c] = nil
		t := r.target
		r.wg.Add(1)
		r.mu.Unlock()
		go r.forwardTCP(c, t)
	}
}

// deadlineConn bounds idle connections without imposing a short lifetime on
// active streams. Fixed buffers and connection caps bound forwarding memory.
type deadlineConn struct {
	net.Conn
	idle  time.Duration
	touch func()
}

func (c deadlineConn) Read(b []byte) (int, error) {
	n, e := c.Conn.Read(b)
	if n > 0 {
		c.touch()
	}
	return n, e
}
func (c deadlineConn) Write(b []byte) (int, error) {
	c.Conn.SetWriteDeadline(time.Now().Add(c.idle))
	n, e := c.Conn.Write(b)
	if n > 0 {
		c.touch()
	}
	return n, e
}

var copyBuffers = sync.Pool{New: func() any { return make([]byte, 32*1024) }}

func (r *rule) copy(dst, src net.Conn, up bool, touch func()) error {
	buf := copyBuffers.Get().([]byte)
	defer copyBuffers.Put(buf)
	counter := &countWriter{dst: deadlineConn{Conn: dst, idle: r.tcpIdle, touch: touch}, r: r, up: up}
	_, e := io.CopyBuffer(counter, deadlineConn{Conn: src, idle: r.tcpIdle, touch: touch}, buf)
	if tcp, ok := dst.(*net.TCPConn); ok {
		if closeErr := tcp.CloseWrite(); e == nil {
			e = closeErr
		}
	}
	return e
}

type countWriter struct {
	dst io.Writer
	r   *rule
	up  bool
}

func (w *countWriter) Write(b []byte) (int, error) {
	n, e := w.dst.Write(b)
	w.r.mu.Lock()
	if w.up {
		w.r.up += int64(n)
	} else {
		w.r.down += int64(n)
	}
	w.r.mu.Unlock()
	return n, e
}
func (r *rule) forwardTCP(c net.Conn, t target) {
	defer r.wg.Done()
	defer c.Close()
	defer func() { r.mu.Lock(); delete(r.connections, c); r.mu.Unlock() }()
	// Rule cancellation interrupts in-flight upstream dials during shutdown.
	dialer := net.Dialer{Timeout: 5 * time.Second}
	upstream, e := dialer.DialContext(r.dialCtx, "tcp", t.address)
	if e != nil {
		r.mu.Lock()
		if r.dialCtx.Err() == nil && r.target.revision == t.revision && r.tcp != nil {
			r.tcpErr = boundedError(e)
		}
		r.mu.Unlock()
		return
	}
	defer upstream.Close()
	r.mu.Lock()
	if r.stopped {
		r.mu.Unlock()
		return
	}
	r.connections[c] = upstream
	if r.target.revision == t.revision && r.tcp != nil {
		r.tcpErr = ""
	}
	r.mu.Unlock()
	// Read inactivity is measured across the entire stream: a long upload may
	// legitimately receive no response until EOF, and vice versa for downloads.
	var activityMu sync.Mutex
	touch := func() {
		activityMu.Lock()
		defer activityMu.Unlock()
		deadline := time.Now().Add(r.tcpIdle)
		c.SetReadDeadline(deadline)
		upstream.SetReadDeadline(deadline)
	}
	touch()
	copyStream := func(dst, src net.Conn, up bool) {
		if e := r.copy(dst, src, up, touch); e != nil {
			// Errors/idle expiry abort both directions. A clean EOF only sends
			// FIN, allowing the other direction to finish independently.
			c.Close()
			upstream.Close()
		}
	}
	done := make(chan struct{})
	go func() { copyStream(upstream, c, true); close(done) }()
	copyStream(c, upstream, false)
	<-done
}
func (u *udpListener) read() {
	r := u.rule
	defer r.wg.Done()
	buf := make([]byte, maxPacketSize)
	for {
		n, client, e := u.conn.ReadFromUDP(buf)
		if e != nil {
			r.mu.Lock()
			if r.udp == u {
				r.udpErr = boundedError(e)
				r.udp = nil
			}
			r.mu.Unlock()
			u.close()
			return
		}
		key := client.String()
		r.mu.Lock()
		if r.udp != u {
			r.mu.Unlock()
			return
		}
		s := r.sessions[key]
		if s != nil && s.listener != u {
			// A failed listener's cleanup can overlap its replacement. A client
			// on the new socket must not inherit the retired socket's session.
			s.conn.Close()
			delete(r.sessions, key)
			s = nil
		}
		if s == nil {
			if len(r.sessions) >= MaxUDPSessions {
				r.mu.Unlock()
				continue
			}
			conn, e := net.DialUDP("udp", nil, r.target.udp)
			if e != nil {
				r.udpErr = boundedError(e)
				r.mu.Unlock()
				continue
			}
			s = &udpSession{conn: conn, listener: u, client: client, revision: r.target.revision}
			r.sessions[key] = s
			r.wg.Add(1)
			go r.readUDP(key, s)
		}
		s.conn.SetReadDeadline(time.Now().Add(r.udpIdle))
		s.conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
		r.mu.Unlock()
		written, e := s.conn.Write(buf[:n])
		r.mu.Lock()
		if e == nil {
			r.up += int64(written)
		}
		if e != nil && r.udp == u && s.revision == r.target.revision {
			r.udpErr = boundedError(e)
		}
		r.mu.Unlock()
	}
}
func (r *rule) readUDP(key string, s *udpSession) {
	defer r.wg.Done()
	defer s.conn.Close()
	defer func() {
		r.mu.Lock()
		if r.sessions[key] == s {
			delete(r.sessions, key)
		}
		r.mu.Unlock()
	}()
	buf := make([]byte, maxPacketSize)
	for {
		n, e := s.conn.Read(buf)
		if e != nil {
			// Idle expiry is normal eviction, not an upstream failure.
			if ne, ok := e.(net.Error); !errors.Is(e, net.ErrClosed) && (!ok || !ne.Timeout()) {
				r.mu.Lock()
				if r.udp == s.listener && r.sessions[key] == s && s.revision == r.target.revision {
					r.udpErr = boundedError(e)
				}
				r.mu.Unlock()
			}
			return
		}
		s.conn.SetReadDeadline(time.Now().Add(r.udpIdle))
		s.listener.conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
		written, e := s.listener.conn.WriteToUDP(buf[:n], s.client)
		if e != nil {
			return
		}
		r.mu.Lock()
		r.down += int64(written)
		if r.udp == s.listener && r.sessions[key] == s && s.revision == r.target.revision {
			r.udpErr = ""
		}
		r.mu.Unlock()
	}
}
