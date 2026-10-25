package xraycore

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
)

type egressBridge struct {
	port           int
	user, password string
}

func newEgressBridge() (*egressBridge, error) {
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	var key [24]byte
	if _, err = rand.Read(key[:]); err != nil {
		return nil, err
	}
	return &egressBridge{port: port, user: "meter", password: hex.EncodeToString(key[:])}, nil
}
func (b *egressBridge) dial(ctx context.Context, network, address string) (net.Conn, error) {
	control, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(b.port)))
	if err != nil {
		return nil, fmt.Errorf("出口连接不可用")
	}
	ok := false
	defer func() {
		if !ok {
			control.Close()
		}
	}()
	control.SetDeadline(time.Now().Add(10 * time.Second))
	if _, err = control.Write([]byte{5, 1, 2}); err != nil {
		return nil, err
	}
	var response [2]byte
	if _, err = io.ReadFull(control, response[:]); err != nil || response != [2]byte{5, 2} {
		return nil, fmt.Errorf("出口认证协商失败")
	}
	auth := append([]byte{1, byte(len(b.user))}, []byte(b.user)...)
	auth = append(auth, byte(len(b.password)))
	auth = append(auth, []byte(b.password)...)
	if _, err = control.Write(auth); err != nil {
		return nil, err
	}
	if _, err = io.ReadFull(control, response[:]); err != nil || response != [2]byte{1, 0} {
		return nil, fmt.Errorf("出口认证失败")
	}
	target, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}
	cmd := byte(1)
	requestTarget := target
	if network == "udp" {
		cmd = 3
		requestTarget = &net.UDPAddr{IP: net.IPv4zero}
	}
	request := append([]byte{5, cmd, 0}, websiteUDPHeader(requestTarget)[3:]...)
	if _, err = control.Write(request); err != nil {
		return nil, err
	}
	var header [4]byte
	if _, err = io.ReadFull(control, header[:]); err != nil || header[0] != 5 || header[1] != 0 {
		return nil, fmt.Errorf("出口无法建立连接")
	}
	host, port, err := readWebsiteAddress(control, header[3])
	if err != nil {
		return nil, err
	}
	control.SetDeadline(time.Time{})
	if network != "udp" {
		ok = true
		return control, nil
	}
	ip := net.ParseIP(host)
	if ip == nil || (!ip.IsLoopback() && !ip.IsUnspecified()) || port < 1 {
		return nil, fmt.Errorf("出口 UDP 地址无效")
	}
	relay, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
	if err != nil {
		return nil, err
	}
	ok = true
	conn := &egressUDPConn{UDPConn: relay, control: control, target: target, header: websiteUDPHeader(target)}
	go func() { io.Copy(io.Discard, control); relay.Close() }()
	return conn, nil
}

type egressUDPConn struct {
	*net.UDPConn
	control net.Conn
	target  *net.UDPAddr
	header  []byte
}

func (c *egressUDPConn) RemoteAddr() net.Addr { return c.target }
func (c *egressUDPConn) Close() error         { c.control.Close(); return c.UDPConn.Close() }
func (c *egressUDPConn) Write(p []byte) (int, error) {
	if len(p)+len(c.header) > 65507 {
		return 0, fmt.Errorf("UDP 数据过大")
	}
	packet := append(append(make([]byte, 0, len(p)+len(c.header)), c.header...), p...)
	n, err := c.UDPConn.Write(packet)
	if err != nil {
		return 0, err
	}
	if n != len(packet) {
		return 0, io.ErrShortWrite
	}
	return len(p), nil
}
func (c *egressUDPConn) Read(p []byte) (int, error) {
	buf := make([]byte, 65535)
	for {
		n, err := c.UDPConn.Read(buf)
		if err != nil {
			return 0, err
		}
		if n < 4 || buf[0] != 0 || buf[1] != 0 || buf[2] != 0 {
			continue
		}
		reader := bytes.NewReader(buf[4:n])
		host, port, err := readWebsiteAddress(reader, buf[3])
		if err != nil || port != c.target.Port || !net.ParseIP(host).Equal(c.target.IP) {
			continue
		}
		data := buf[n-reader.Len() : n]
		if len(data) > len(p) {
			return 0, io.ErrShortBuffer
		}
		return copy(p, data), nil
	}
}
