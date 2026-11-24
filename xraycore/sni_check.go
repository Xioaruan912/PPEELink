package xraycore

import (
	"context"
	"crypto/tls"
	"net"
	"sort"
	"strings"
	"sync"
	"time"
)

// SNICheckResult describes a single Reality masquerade target measured directly from the VPS.
type SNICheckResult struct {
	Host        string `json:"host"`
	Target      string `json:"target"`
	OK          bool   `json:"ok"`
	TLS13       bool   `json:"tls13"`
	H2          bool   `json:"h2"`
	RTTMs       int    `json:"rttMs"`
	Issuer      string `json:"issuer,omitempty"`
	ValidDays   int    `json:"validDays,omitempty"`
	Score       int    `json:"score"`
	Grade       string `json:"grade"` // best | ok | bad
	Reason      string `json:"reason,omitempty"`
}

var defaultRealitySNICandidates = []string{
	"www.apple.com",
	"www.icloud.com",
	"www.microsoft.com",
	"aws.amazon.com",
	"www.amazon.com",
	"www.cloudflare.com",
	"www.samsung.com",
	"cdn.jsdelivr.net",
}

func cleanSNIHost(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	if idx := strings.IndexByte(s, '/'); idx >= 0 {
		s = s[:idx]
	}
	if h, _, err := net.SplitHostPort(s); err == nil {
		s = h
	}
	return strings.ToLower(strings.Trim(s, "[]"))
}

// CheckRealitySNIs probes candidate (and optional custom) SNI hosts directly from the VPS
// to verify TLS 1.3 + HTTP/2 + certificate validity + handshake latency.
func CheckRealitySNIs(ctx context.Context, custom []string) []SNICheckResult {
	seen := map[string]bool{}
	list := make([]string, 0, len(custom)+len(defaultRealitySNICandidates))
	for _, raw := range custom {
		h := cleanSNIHost(raw)
		if h != "" && !seen[h] {
			seen[h] = true
			list = append(list, h)
		}
	}
	for _, h := range defaultRealitySNICandidates {
		if !seen[h] {
			seen[h] = true
			list = append(list, h)
		}
	}
	if len(list) > 12 {
		list = list[:12]
	}

	results := make([]SNICheckResult, len(list))
	var wg sync.WaitGroup
	for i, host := range list {
		wg.Add(1)
		go func(idx int, h string) {
			defer wg.Done()
			results[idx] = probeSingleRealitySNI(ctx, h)
		}(i, host)
	}
	wg.Wait()

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		if results[i].RTTMs == 0 {
			return false
		}
		if results[j].RTTMs == 0 {
			return true
		}
		return results[i].RTTMs < results[j].RTTMs
	})
	return results
}

func probeSingleRealitySNI(ctx context.Context, host string) SNICheckResult {
	res := SNICheckResult{
		Host:   host,
		Target: host + ":443",
		Grade:  "bad",
	}
	dialCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	d := &net.Dialer{Timeout: 3500 * time.Millisecond}
	start := time.Now()
	rawConn, err := d.DialContext(dialCtx, "tcp", net.JoinHostPort(host, "443"))
	if err != nil {
		res.Reason = "443 端口不可达"
		return res
	}
	defer rawConn.Close()

	_ = rawConn.SetDeadline(time.Now().Add(3500 * time.Millisecond))
	tlsConn := tls.Client(rawConn, &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
		NextProtos: []string{"h2", "http/1.1"},
	})
	if err := tlsConn.HandshakeContext(dialCtx); err != nil {
		res.Reason = "TLS 握手失败"
		return res
	}
	rtt := int(time.Since(start).Milliseconds())
	if rtt <= 0 {
		rtt = 1
	}

	state := tlsConn.ConnectionState()
	res.OK = true
	res.RTTMs = rtt
	res.TLS13 = state.Version == tls.VersionTLS13
	res.H2 = state.NegotiatedProtocol == "h2"

	if len(state.PeerCertificates) > 0 {
		cert := state.PeerCertificates[0]
		if len(cert.Issuer.Organization) > 0 {
			res.Issuer = cert.Issuer.Organization[0]
		} else {
			res.Issuer = cert.Issuer.CommonName
		}
		res.ValidDays = int(time.Until(cert.NotAfter).Hours() / 24)
	}

	score := 0
	if res.TLS13 {
		score += 45
	} else {
		res.Reason = "不支持 TLS 1.3"
	}
	if res.H2 {
		score += 25
	} else if res.Reason == "" {
		res.Reason = "未协商 HTTP/2"
	}
	if res.ValidDays >= 15 {
		score += 15
	} else if res.ValidDays > 0 {
		score += 5
	}
	switch {
	case rtt <= 20:
		score += 15
	case rtt <= 50:
		score += 12
	case rtt <= 120:
		score += 8
	default:
		score += 3
	}
	res.Score = score

	switch {
	case res.TLS13 && res.H2 && score >= 85:
		res.Grade = "best"
	case res.TLS13 && score >= 65:
		res.Grade = "ok"
	default:
		res.Grade = "bad"
	}
	return res
}
