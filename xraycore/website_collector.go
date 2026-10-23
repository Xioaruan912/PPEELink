package xraycore

// This is a payload meter, not a packet/IP-to-website classifier. A SOCKS domain
// is the only source of website attribution. IP literals (including sniffed TLS
// connections whose original destination is an IP) are deliberately unknown.
import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	websiteUnknown       = "unknown"
	websiteMaxSessions   = 1024
	websiteMaxUDPRemotes = 64   // per authenticated association
	websiteMaxRecords    = 4096 // undrained (hour, inbound, domain) tuples
	websiteMaxDomains    = 512  // per (hour, inbound); excess is honestly unknown
	websiteIdleTimeout   = 2 * time.Minute
)

type websiteKey struct {
	bucket      int64
	tag, domain string
}
type websiteCounters struct {
	mu      sync.Mutex
	records map[websiteKey]Counter
	domains map[websiteKey]int
}

func newWebsiteCounters() *websiteCounters {
	return &websiteCounters{records: make(map[websiteKey]Counter), domains: make(map[websiteKey]int)}
}

// reserve happens BEFORE sending payload. At the memory limit new traffic fails
// closed rather than being forwarded unmetered. Per-domain cardinality overflow
// is attributed to unknown, never to another website.
func (s *websiteCounters) reserve(tag, domain string, seconds int) (websiteKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := websiteKey{time.Now().UTC().Unix() / int64(seconds) * int64(seconds), tag, domain}
	group := websiteKey{k.bucket, tag, ""}
	if _, ok := s.records[k]; ok {
		return k, nil
	}
	if s.domains[group] >= websiteMaxDomains {
		k.domain = websiteUnknown
	}
	if _, ok := s.records[k]; ok {
		return k, nil
	}
	if len(s.records) >= websiteMaxRecords {
		return k, errors.New("website statistics queue full")
	}
	s.records[k] = Counter{}
	s.domains[group]++
	return k, nil
}
func (s *websiteCounters) add(k websiteKey, n int, up bool) {
	if n <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.records[k]
	if up {
		c.Up += int64(n)
	} else {
		c.Down += int64(n)
	}
	s.records[k] = c
}
func (s *websiteCounters) drain() []WebsiteTrafficBucket {
	s.mu.Lock()
	records := s.records
	// Detach the snapshot under the lock; aggregation and sorting do not block relays.
	s.records = make(map[websiteKey]Counter)
	s.domains = make(map[websiteKey]int)
	s.mu.Unlock()
	out := make([]WebsiteTrafficBucket, 0, len(records))
	for k, c := range records {
		if c.Up != 0 || c.Down != 0 {
			out = append(out, WebsiteTrafficBucket{BucketStart: k.bucket, InboundTag: k.tag, Domain: k.domain, Up: c.Up, Down: c.Down})
		}
	}
	// An in-flight write may commit into the next drain; its original time bucket
	// remains intact. Overshoot is bounded by the capped concurrent sessions.
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.BucketStart != b.BucketStart {
			return a.BucketStart < b.BucketStart
		}
		if a.InboundTag != b.InboundTag {
			return a.InboundTag < b.InboundTag
		}
		return a.Domain < b.Domain
	})
	return out
}

type websiteIdentity struct{ tag, password string }

// WebsiteCollector is an authenticated, IPv4-loopback-only SOCKS5 relay. Start
// it only for proxy traffic; do not configure system/application proxy settings
// to use it. Credentials are ephemeral and never accepted from the panel.
// Close tears down active TCP/UDP forwarding and waits for all relay goroutines.
// Statistics are in-memory deltas; the caller must retain drained batches until
// acknowledged by its reporting endpoint (and persist them if crash durability
// is required). Domain attribution is exact requested host, not eTLD+1.
type WebsiteCollector struct {
	listener     net.Listener
	identities   map[string]websiteIdentity
	stats        *websiteCounters
	seconds      int
	blockPrivate bool
	ctx          context.Context
	cancel       context.CancelFunc
	mu           sync.Mutex
	closed       bool
	connections  map[io.Closer]struct{}
	sessions     chan struct{}
	udpSockets   chan struct{} // global cap, independent of association count
	wg           sync.WaitGroup
	closeOnce    sync.Once
}

func NewWebsiteCollector(spec Spec) (*WebsiteCollector, error) {
	return newWebsiteCollector(spec, newWebsiteCounters())
}
func newWebsiteCollector(spec Spec, stats *websiteCounters) (*WebsiteCollector, error) {
	seconds := 3600
	if spec.WebsiteTraffic != nil && spec.WebsiteTraffic.BucketSeconds != 0 {
		seconds = spec.WebsiteTraffic.BucketSeconds
	}
	if seconds < 60 || seconds > 86400 || 86400%seconds != 0 {
		return nil, errors.New("website bucketSeconds must divide 86400 and be between 60 and 86400")
	}
	identities := make(map[string]websiteIdentity)
	tags := make(map[string]bool)
	for i, in := range spec.Inbounds {
		if in.Tag == "" || in.Tag == "api" || tags[in.Tag] {
			return nil, errors.New("website collection requires unique nonempty inbound tags (not api)")
		}
		tags[in.Tag] = true
		secret := make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			return nil, err
		}
		identities[strconv.Itoa(i)] = websiteIdentity{in.Tag, hex.EncodeToString(secret)}
	}
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	c := &WebsiteCollector{listener: ln, identities: identities, stats: stats, seconds: seconds, blockPrivate: spec.BlockPrivate, ctx: ctx, cancel: cancel, connections: make(map[io.Closer]struct{}), sessions: make(chan struct{}, websiteMaxSessions), udpSockets: make(chan struct{}, 1024)}
	c.wg.Add(1)
	go c.accept()
	return c, nil
}
func (c *WebsiteCollector) Addr() string                  { return c.listener.Addr().String() }
func (c *WebsiteCollector) Drain() []WebsiteTrafficBucket { return c.stats.drain() }
func (c *WebsiteCollector) track(conn io.Closer) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		_ = conn.Close()
		return false
	}
	c.connections[conn] = struct{}{}
	return true
}
func (c *WebsiteCollector) untrack(conn io.Closer) {
	c.mu.Lock()
	delete(c.connections, conn)
	c.mu.Unlock()
	_ = conn.Close()
}
func (c *WebsiteCollector) Close() error {
	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.closed = true
		c.cancel()
		_ = c.listener.Close()
		for conn := range c.connections {
			_ = conn.Close()
		}
		c.mu.Unlock()
		c.wg.Wait()
	})
	return nil
}
func (c *WebsiteCollector) accept() {
	defer c.wg.Done()
	for {
		conn, err := c.listener.Accept()
		if err != nil {
			return
		}
		if !conn.RemoteAddr().(*net.TCPAddr).IP.IsLoopback() {
			_ = conn.Close()
			continue
		}
		select {
		case c.sessions <- struct{}{}:
		default:
			_ = conn.Close()
			continue
		}
		if !c.track(conn) {
			<-c.sessions
			return
		}
		c.wg.Add(1)
		go func() { defer c.wg.Done(); defer func() { <-c.sessions }(); defer c.untrack(conn); c.serve(conn) }()
	}
}

func (c *WebsiteCollector) serve(conn net.Conn) {
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	var head [2]byte
	if _, err := io.ReadFull(conn, head[:]); err != nil || head[0] != 5 || head[1] == 0 {
		return
	}
	methods := make([]byte, int(head[1]))
	if _, err := io.ReadFull(conn, methods); err != nil {
		return
	}
	auth := false
	for _, m := range methods {
		if m == 2 {
			auth = true
		}
	}
	if !auth {
		_, _ = conn.Write([]byte{5, 255})
		return
	}
	if _, err := conn.Write([]byte{5, 2}); err != nil {
		return
	}
	if _, err := io.ReadFull(conn, head[:]); err != nil || head[0] != 1 || head[1] == 0 {
		return
	}
	user := make([]byte, int(head[1]))
	if _, err := io.ReadFull(conn, user); err != nil {
		return
	}
	if _, err := io.ReadFull(conn, head[:1]); err != nil || head[0] == 0 {
		return
	}
	pass := make([]byte, int(head[0]))
	if _, err := io.ReadFull(conn, pass); err != nil {
		return
	}
	identity, ok := c.identities[string(user)]
	if !ok || subtle.ConstantTimeCompare([]byte(identity.password), pass) != 1 {
		_, _ = conn.Write([]byte{1, 1})
		return
	}
	if _, err := conn.Write([]byte{1, 0}); err != nil {
		return
	}
	var req [4]byte
	if _, err := io.ReadFull(conn, req[:]); err != nil || req[0] != 5 || req[2] != 0 {
		return
	}
	host, port, err := readWebsiteAddress(conn, req[3])
	if err != nil {
		return
	}
	switch req[1] {
	case 1:
		c.tcp(conn, identity.tag, host, port)
	case 3:
		c.udp(conn, identity.tag, host, port)
	default:
		_ = websiteReply(conn, 7, nil)
	}
}
func readWebsiteAddress(r io.Reader, typ byte) (string, int, error) {
	size := 0
	switch typ {
	case 1:
		size = 4
	case 4:
		size = 16
	case 3:
		var n [1]byte
		if _, err := io.ReadFull(r, n[:]); err != nil {
			return "", 0, err
		}
		size = int(n[0])
		if size == 0 {
			return "", 0, errors.New("empty domain")
		}
	default:
		return "", 0, errors.New("invalid address type")
	}
	b := make([]byte, size+2)
	if _, err := io.ReadFull(r, b); err != nil {
		return "", 0, err
	}
	host := string(b[:size])
	if typ != 3 {
		host = net.IP(b[:size]).String()
	}
	return host, int(binary.BigEndian.Uint16(b[size:])), nil
}
func websiteDomain(host string) string {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if net.ParseIP(host) != nil || len(host) > 253 || host == "" {
		return websiteUnknown
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return websiteUnknown
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
				return websiteUnknown
			}
		}
	}
	return host
}
func websiteReply(w io.Writer, code byte, addr *net.UDPAddr) error {
	ip := net.IPv4(127, 0, 0, 1).To4()
	port := 0
	if addr != nil {
		ip = addr.IP.To4()
		port = addr.Port
	}
	b := []byte{5, code, 0, 1, ip[0], ip[1], ip[2], ip[3], byte(port >> 8), byte(port)}
	_, err := w.Write(b)
	return err
}

// Resolve once, validate every address, and dial the validated IP itself. This
// prevents private-address/DNS-rebinding bypass of Xray's pre-relay IP rules.
func (c *WebsiteCollector) resolve(ctx context.Context, host string, port int) ([]string, error) {
	if port < 1 || port > 65535 {
		return nil, errors.New("invalid port")
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, ip := range ips {
		if c.blockPrivate && websitePrivateIP(ip.IP) {
			continue
		}
		out = append(out, net.JoinHostPort(ip.IP.String(), strconv.Itoa(port)))
	}
	if len(out) == 0 {
		return nil, errors.New("destination denied")
	}
	return out, nil
}

var websiteDeniedPrefixes = func() []netip.Prefix {
	ranges := []string{"100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32", "64:ff9b::/96", "64:ff9b:1::/48", "2002::/16"}
	prefixes := make([]netip.Prefix, 0, len(ranges))
	for _, value := range ranges {
		prefixes = append(prefixes, netip.MustParsePrefix(value))
	}
	return prefixes
}()

var websiteCopyBuffers = sync.Pool{New: func() any { return new([32 * 1024]byte) }}

func websitePrivateIP(ip net.IP) bool {
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}
	addr = addr.Unmap()
	for _, prefix := range websiteDeniedPrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}
func (c *WebsiteCollector) dial(network, host string, port int) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(c.ctx, 10*time.Second)
	defer cancel()
	addrs, err := c.resolve(ctx, host, port)
	if err != nil {
		return nil, err
	}
	var last error
	for _, addr := range addrs {
		conn, err := (&net.Dialer{}).DialContext(ctx, network, addr)
		if err == nil {
			return conn, nil
		}
		last = err
	}
	return nil, last
}
func (c *WebsiteCollector) tcp(client net.Conn, tag, host string, port int) {
	remote, err := c.dial("tcp", host, port)
	if err != nil {
		_ = websiteReply(client, 2, nil)
		return
	}
	if !c.track(remote) {
		return
	}
	defer c.untrack(remote)
	if err := websiteReply(client, 0, nil); err != nil {
		return
	}
	_ = client.SetDeadline(time.Time{})
	_ = remote.SetDeadline(time.Time{})
	done := make(chan struct{})
	domain := websiteDomain(host)
	go func() {
		c.copy(remote, client, tag, domain, true)
		if tcp, ok := remote.(*net.TCPConn); ok {
			_ = tcp.CloseWrite()
		}
		close(done)
	}()
	c.copy(client, remote, tag, domain, false)
	if tcp, ok := client.(*net.TCPConn); ok {
		_ = tcp.CloseWrite()
	}
	<-done
}
func (c *WebsiteCollector) copy(dst, src net.Conn, tag, domain string, up bool) {
	buffer := websiteCopyBuffers.Get().(*[32 * 1024]byte)
	defer websiteCopyBuffers.Put(buffer)
	buf := buffer[:]
	for {
		_ = src.SetReadDeadline(time.Now().Add(websiteIdleTimeout))
		n, err := src.Read(buf)
		if n > 0 {
			k, e := c.stats.reserve(tag, domain, c.seconds)
			if e != nil {
				_ = src.Close()
				_ = dst.Close()
				return
			}
			_ = dst.SetWriteDeadline(time.Now().Add(websiteIdleTimeout))
			written, e := dst.Write(buf[:n])
			c.stats.add(k, written, up)
			// Activity in either direction keeps the stream alive. An idle
			// uplink must not time out a long-running download (and vice versa).
			if written > 0 {
				deadline := time.Now().Add(websiteIdleTimeout)
				_ = src.SetReadDeadline(deadline)
				_ = dst.SetReadDeadline(deadline)
			}
			if e != nil || written != n {
				_ = src.Close()
				_ = dst.Close()
				return
			}
		}
		if err != nil {
			return
		}
	}
}

// UDP uses one connected remote socket per destination, so unsolicited packets
// cannot enter an association and two domain names resolving to the same IP do
// not steal each other's attribution. FRAG != 0 is rejected (RFC 1928).
func (c *WebsiteCollector) udp(control net.Conn, tag, requestedHost string, requestedPort int) {
	ip := net.ParseIP(requestedHost)
	if ip == nil || (!ip.IsUnspecified() && !ip.IsLoopback()) {
		_ = websiteReply(control, 2, nil)
		return
	}
	relay, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		return
	}
	if !c.track(relay) {
		return
	}
	defer c.untrack(relay)
	if err := websiteReply(control, 0, relay.LocalAddr().(*net.UDPAddr)); err != nil {
		return
	}
	_ = control.SetDeadline(time.Time{})
	// TCP lifetime governs the UDP association. Closing it interrupts ReadFromUDP.
	done := make(chan struct{})
	go func() { _, _ = io.Copy(io.Discard, control); _ = relay.Close(); close(done) }()
	defer func() { _ = control.Close(); <-done }()
	type udpRemote struct {
		conn net.Conn
		done chan struct{}
	}
	remotes := make(map[string]udpRemote)
	var remoteWG sync.WaitGroup
	defer func() {
		for _, remote := range remotes {
			_ = remote.conn.Close()
		}
		remoteWG.Wait()
	}()
	client := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: requestedPort}
	buf := make([]byte, 65535)
	for {
		_ = relay.SetReadDeadline(time.Now().Add(websiteIdleTimeout))
		n, source, err := relay.ReadFromUDP(buf)
		if err != nil {
			return
		}
		if !source.IP.IsLoopback() || (client.Port != 0 && source.Port != client.Port) {
			continue
		}
		if n < 4 || buf[0] != 0 || buf[1] != 0 || buf[2] != 0 {
			continue
		}
		reader := bytes.NewReader(buf[4:n])
		host, port, err := readWebsiteAddress(reader, buf[3])
		if err != nil {
			continue
		}
		offset := n - reader.Len()
		payload := buf[offset:n]
		// Bind the unspecified RFC client port only after a structurally valid packet.
		// Loopback untrusted processes still need the unpredictable association port;
		// the authenticated TCP control connection is mandatory for its lifetime.
		if client.Port == 0 {
			client.Port = source.Port
		}
		for key, remote := range remotes {
			select {
			case <-remote.done:
				delete(remotes, key)
			default:
			}
		}
		key := net.JoinHostPort(host, strconv.Itoa(port))
		entry, ok := remotes[key]
		if !ok {
			if len(remotes) >= websiteMaxUDPRemotes {
				continue
			}
			select {
			case c.udpSockets <- struct{}{}:
			default:
				continue
			}
			remote, err := c.dial("udp", host, port)
			if err != nil {
				<-c.udpSockets
				continue
			}
			if !c.track(remote) {
				<-c.udpSockets
				return
			}
			entry = udpRemote{remote, make(chan struct{})}
			remotes[key] = entry
			peer := *client
			domain := websiteDomain(host)
			remoteWG.Add(1)
			go func(conn net.Conn, domain string, done chan struct{}) {
				defer remoteWG.Done()
				defer close(done)
				defer func() { <-c.udpSockets }()
				defer c.untrack(conn)
				reply := make([]byte, 65535)
				header := websiteUDPHeader(conn.RemoteAddr().(*net.UDPAddr))
				for {
					_ = conn.SetReadDeadline(time.Now().Add(websiteIdleTimeout))
					n, err := conn.Read(reply[len(header):])
					if err != nil {
						return
					}
					k, err := c.stats.reserve(tag, domain, c.seconds)
					if err != nil {
						return
					}
					copy(reply, header)
					_ = relay.SetWriteDeadline(time.Now().Add(10 * time.Second))
					written, err := relay.WriteToUDP(reply[:len(header)+n], &peer)
					if err == nil && written == len(header)+n {
						c.stats.add(k, n, false)
						_ = relay.SetReadDeadline(time.Now().Add(websiteIdleTimeout))
					}
					if err != nil {
						return
					}
				}
			}(entry.conn, domain, entry.done)
		}
		k, err := c.stats.reserve(tag, websiteDomain(host), c.seconds)
		if err != nil {
			return
		}
		_ = entry.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		written, err := entry.conn.Write(payload)
		if err == nil && written == len(payload) {
			c.stats.add(k, written, true)
			_ = entry.conn.SetReadDeadline(time.Now().Add(websiteIdleTimeout))
		}
	}
}
func websiteUDPHeader(addr *net.UDPAddr) []byte {
	b := []byte{0, 0, 0}
	if ip := addr.IP.To4(); ip != nil {
		b = append(b, 1)
		b = append(b, ip...)
	} else {
		b = append(b, 4)
		b = append(b, addr.IP.To16()...)
	}
	return append(b, byte(addr.Port>>8), byte(addr.Port))
}

// configOutbounds keeps inbound identity in a separate authenticated SOCKS
// account/outbound. Tags never come from traffic-controlled SOCKS destinations.
func (c *WebsiteCollector) configOutbounds() ([]any, []any) {
	out, rules := []any{}, []any{}
	port := c.listener.Addr().(*net.TCPAddr).Port
	users := make([]string, 0, len(c.identities))
	for user := range c.identities {
		users = append(users, user)
	}
	sort.Strings(users)
	for _, user := range users {
		id := c.identities[user]
		tag := "website-" + user
		out = append(out, map[string]any{"tag": tag, "protocol": "socks", "settings": map[string]any{"servers": []any{map[string]any{"address": "127.0.0.1", "port": port, "users": []any{map[string]any{"user": user, "pass": id.password}}}}}})
		rules = append(rules, map[string]any{"inboundTag": []string{id.tag}, "outboundTag": tag})
	}
	return out, rules
}
