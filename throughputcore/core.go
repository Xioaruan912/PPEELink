// Package throughputcore runs short, authenticated direct Agent-to-Agent measurements.
package throughputcore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"math"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Job struct {
	ID         uint      `json:"id"`
	Role       string    `json:"role"`
	Host       string    `json:"host"`
	Port       int       `json:"port"`
	Protocol   string    `json:"protocol"`
	Direction  string    `json:"direction"`
	Family     int       `json:"family"`
	Duration   int       `json:"duration"`
	Streams    int       `json:"streams"`
	RateMbps   float64   `json:"rateMbps"`
	Username   string    `json:"username"`
	Password   string    `json:"password"`
	PublicKey  string    `json:"publicKey,omitempty"`
	PrivateKey string    `json:"privateKey,omitempty"`
	ExpiresAt  time.Time `json:"expiresAt"`
}
type Report struct {
	ID     uint    `json:"id"`
	Role   string  `json:"role"`
	Status string  `json:"status"`
	Error  string  `json:"error,omitempty"`
	Result *Result `json:"result,omitempty"`
}
type Capability struct {
	Available           bool   `json:"available"`
	Path                string `json:"path"`
	Version             string `json:"version"`
	AuthSupported       bool   `json:"authSupported"`
	BidirSupported      bool   `json:"bidirSupported"`
	JSONStreamSupported bool   `json:"jsonStreamSupported"`
	Error               string `json:"error,omitempty"`
}
type Result struct {
	ProgressIntervals   []Interval       `json:"progressIntervals,omitempty"`
	ProgressDirection   string           `json:"progressDirection,omitempty"`
	ProgressObservation string           `json:"progressObservation,omitempty"`
	Protocol            string           `json:"protocol"`
	Direction           string           `json:"direction"`
	Duration            float64          `json:"duration"`
	Streams             int              `json:"streams"`
	RateMbps            float64          `json:"rateMbps"`
	CapReached          bool             `json:"capReached"`
	Forward             *DirectionResult `json:"forward,omitempty"`
	Reverse             *DirectionResult `json:"reverse,omitempty"`
	CPUHost             float64          `json:"cpuHost"`
	CPURemote           float64          `json:"cpuRemote"`
	Warnings            []string         `json:"warnings,omitempty"`
}
type DirectionResult struct {
	RTTMs         *float64   `json:"rttMs,omitempty"`
	BitsPerSecond float64    `json:"bitsPerSecond"`
	Bytes         uint64     `json:"bytes"`
	Retransmits   uint64     `json:"retransmits"`
	JitterMs      float64    `json:"jitterMs"`
	LostPackets   uint64     `json:"lostPackets"`
	Packets       uint64     `json:"packets"`
	LossPercent   float64    `json:"lossPercent"`
	Intervals     []Interval `json:"intervals"`
}
type Interval struct {
	Start         float64 `json:"start"`
	End           float64 `json:"end"`
	BitsPerSecond float64 `json:"bitsPerSecond"`
	Bytes         uint64  `json:"bytes"`
	JitterMs      float64 `json:"jitterMs"`
	LossPercent   float64 `json:"lossPercent"`
}

var versionPattern = regexp.MustCompile(`(?i)iperf\s+(\d+)\.(\d+)(?:\.(\d+))?`)

func ParseCapability(path, version, help string) Capability {
	c := Capability{Available: path != "", Path: path}
	m := versionPattern.FindStringSubmatch(version)
	if len(m) > 2 {
		c.Version = m[1] + "." + m[2]
		if m[3] != "" {
			c.Version += "." + m[3]
		}
		major, _ := strconv.Atoi(m[1])
		minor, _ := strconv.Atoi(m[2])
		modern := major > 3 || (major == 3 && minor >= 22)
		c.AuthSupported = modern && strings.Contains(help, "--rsa-private-key-path") && strings.Contains(help, "--rsa-public-key-path") && strings.Contains(help, "--authorized-users-path")
		c.BidirSupported = modern && strings.Contains(help, "--bidir")
		c.JSONStreamSupported = modern && strings.Contains(help, "--json-stream")
	}
	if !c.Available {
		c.Error = "iperf3 is not installed; prepare the Agent explicitly"
	} else if !c.AuthSupported {
		c.Error = "iperf3 >=3.22 with RSA OAEP authentication is required; weak padding is disabled"
	}
	return c
}
func Inspect(ctx context.Context) Capability {
	path, err := exec.LookPath("iperf3")
	if st, e := os.Stat(toolInstallPath); e == nil && st.Mode().IsRegular() {
		path = toolInstallPath
		err = nil
	}
	if err != nil {
		return ParseCapability("", "", "")
	}
	return inspectPath(ctx, path)
}
func inspectPath(ctx context.Context, path string) Capability {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	run := func(arg string) string {
		cmd := exec.CommandContext(ctx, path, arg)
		configureCommand(cmd)
		out := &limitedBuffer{max: 64 * 1024}
		cmd.Stdout = out
		cmd.Stderr = out
		if cmd.Run() != nil || out.Overflow() {
			return ""
		}
		return string(out.Bytes())
	}
	v := run("--version")
	h := run("--help")
	return ParseCapability(path, v, h)
}
func Validate(j Job) error {
	if j.Role != "client" && j.Role != "server" {
		return errors.New("unsupported throughput role")
	}
	if j.ID == 0 || j.Port < 1024 || j.Port > 65535 || j.Duration < 5 || j.Duration > 30 || j.Streams < 1 || j.Streams > 8 || j.RateMbps <= 0 || j.RateMbps > 10000 || math.IsNaN(j.RateMbps) || math.IsInf(j.RateMbps, 0) {
		return errors.New("throughput job exceeds safe port/duration/stream/rate bounds")
	}
	if j.Protocol != "tcp" && j.Protocol != "udp" {
		return errors.New("unsupported protocol")
	}
	if j.Direction != "forward" && j.Direction != "reverse" && j.Direction != "bidir" {
		return errors.New("unsupported direction")
	}
	if j.Family != 4 && j.Family != 6 {
		return errors.New("address family must be 4 or 6")
	}
	if j.ExpiresAt.IsZero() || !j.ExpiresAt.After(time.Now()) || time.Until(j.ExpiresAt) > 3*time.Minute {
		return errors.New("invalid or expired throughput lease")
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`).MatchString(j.Username) || len(j.Password) < 1 || len(j.Password) > 256 || strings.ContainsAny(j.Password, "\r\n\x00") {
		return errors.New("invalid credentials")
	}
	if j.Role == "client" {
		if len(j.Host) == 0 || len(j.Host) > 253 || strings.HasPrefix(j.Host, "-") || strings.ContainsAny(j.Host, " \t\r\n/\\\x00") {
			return errors.New("invalid throughput host")
		}
		if ip := net.ParseIP(j.Host); ip != nil && ((ip.To4() != nil) != (j.Family == 4)) {
			return errors.New("host address does not match selected family")
		}
		if j.PublicKey == "" {
			return errors.New("missing RSA public key")
		}
	} else if j.PrivateKey == "" {
		return errors.New("missing RSA private key")
	}
	return nil
}
func ClientArgs(j Job, key string) ([]string, error) {
	if err := Validate(j); err != nil {
		return nil, err
	}
	rate := j.RateMbps * 1e6 / float64(j.Streams)
	if j.Direction == "bidir" {
		rate /= 2
	}
	a := []string{"-c", j.Host, "-p", strconv.Itoa(j.Port), "-" + strconv.Itoa(j.Family), "-t", strconv.Itoa(j.Duration), "-P", strconv.Itoa(j.Streams), "-b", strconv.FormatFloat(rate, 'f', 0, 64), "-i", "1", "-J", "--get-server-output", "--connect-timeout", "5000", "--username", j.Username, "--rsa-public-key-path", key}
	if j.Protocol == "udp" {
		a = append(a, "-u")
	}
	if j.Direction == "reverse" {
		a = append(a, "-R")
	}
	if j.Direction == "bidir" {
		a = append(a, "--bidir")
	}
	return a, nil
}
func ServerArgs(j Job, f *AuthFiles) ([]string, error) {
	if err := Validate(j); err != nil {
		return nil, err
	}
	return []string{"-s", "-1", "-p", strconv.Itoa(j.Port), "-" + strconv.Itoa(j.Family), "-J", "--rsa-private-key-path", f.Key, "--authorized-users-path", f.Users, "--server-bitrate-limit", strconv.FormatFloat(j.RateMbps*1.25e6, 'f', 0, 64)}, nil
}

type AuthFiles struct{ Dir, Key, Users string }

func NewAuthFiles(j Job) (*AuthFiles, error) {
	dir, e := os.MkdirTemp("", "subl-throughput-")
	if e != nil {
		return nil, e
	}
	f := &AuthFiles{Dir: dir, Key: filepath.Join(dir, "key.pem"), Users: filepath.Join(dir, "users.csv")}
	key := j.PublicKey
	if j.Role == "server" {
		key = j.PrivateKey
	}
	h := sha256.Sum256([]byte("{" + j.Username + "}" + j.Password))
	if e = os.WriteFile(f.Key, []byte(key), 0600); e == nil {
		e = os.WriteFile(f.Users, []byte(j.Username+","+hex.EncodeToString(h[:])+"\n"), 0600)
	}
	if e != nil {
		f.Close()
		return nil, e
	}
	return f, nil
}
func (f *AuthFiles) Close() {
	if f != nil && f.Dir != "" {
		_ = os.RemoveAll(f.Dir)
	}
}
func jobContext(parent context.Context, j Job) (context.Context, context.CancelFunc) {
	deadline := time.Now().Add(120 * time.Second)
	if j.Role == "client" {
		deadline = time.Now().Add(time.Duration(j.Duration+15) * time.Second)
	}
	if j.ExpiresAt.Before(deadline) {
		deadline = j.ExpiresAt
	}
	return context.WithDeadline(parent, deadline)
}
