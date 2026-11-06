package node

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// Mieru keeps every port/protocol binding and optional parameter when a node is renamed.
type Mieru struct {
	Name string
	Host string
	Port int
	url  url.URL
}

func DecodeMieruURL(link string) (Mieru, error) {
	u, err := url.Parse(link)
	if err != nil || u.Scheme != "mierus" {
		return Mieru{}, fmt.Errorf("无效的 mierus 链接")
	}
	password, hasPassword := u.User.Password()
	if u.Hostname() == "" || u.Port() != "" || u.User.Username() == "" || !hasPassword || password == "" {
		return Mieru{}, fmt.Errorf("mierus 链接缺少有效的服务器或账号密码")
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return Mieru{}, fmt.Errorf("无效的 mierus 查询参数")
	}
	if len(q["profile"]) != 1 || q.Get("profile") == "" {
		return Mieru{}, fmt.Errorf("mierus 链接需要一个 profile")
	}
	ports, protocols := q["port"], q["protocol"]
	if len(ports) == 0 || len(ports) != len(protocols) {
		return Mieru{}, fmt.Errorf("mierus port 与 protocol 必须成对提供")
	}
	firstPort := 0
	for i, raw := range ports {
		port, err := mieruPort(raw)
		if err != nil {
			return Mieru{}, err
		}
		if i == 0 {
			firstPort = port
		}
		if protocols[i] != "TCP" && protocols[i] != "UDP" {
			return Mieru{}, fmt.Errorf("mierus protocol 必须是 TCP 或 UDP")
		}
	}
	name := u.Fragment
	if name == "" {
		name = net.JoinHostPort(u.Hostname(), strconv.Itoa(firstPort))
	}
	return Mieru{Name: name, Host: u.Hostname(), Port: firstPort, url: *u}, nil
}

func mieruPort(raw string) (int, error) {
	parts := strings.Split(raw, "-")
	first, err := strconv.Atoi(parts[0])
	if err != nil || first < 1 || first > 65535 || len(parts) > 2 {
		return 0, fmt.Errorf("mierus 端口或端口范围无效")
	}
	if len(parts) == 2 {
		last, err := strconv.Atoi(parts[1])
		if err != nil || last < first || last > 65535 {
			return 0, fmt.Errorf("mierus 端口范围无效")
		}
	}
	return first, nil
}

func EncodeMieruURL(m Mieru) string {
	u := m.url
	u.Host = m.Host
	if strings.Contains(m.Host, ":") {
		u.Host = "[" + m.Host + "]"
	}
	u.Fragment = m.Name
	q := u.Query()
	if ports := q["port"]; len(ports) > 0 {
		// Endpoint edits replace the first binding; renames preserve ranges and all alternatives.
		if first, _ := mieruPort(ports[0]); first != m.Port {
			ports[0] = strconv.Itoa(m.Port)
			u.RawQuery = q.Encode()
		}
	}
	return u.String()
}
