package node

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"ppeelink/certpin"
	"strconv"
	"strings"
)

type HY2 struct {
	PinSHA256    string
	Certificate  string
	Password     string
	Host         string
	Port         int
	Insecure     int
	Peer         string
	Auth         string
	UpMbps       int
	DownMbps     int
	ALPN         []string
	Name         string
	Sni          string
	Obfs         string
	ObfsPassword string
}

// 开发者测试 CallHy 调用
func CallHy2() {
	hy2 := HY2{
		Password: "asdasd",
		Host:     "qq.com",
		Port:     11926,
		Insecure: 1,
		Peer:     "youku.com",
		Auth:     "",
		UpMbps:   11,
		DownMbps: 55,
		// ALPN:     "h3",
	}
	fmt.Println(EncodeHY2URL(hy2))
}

// hy2 编码
func EncodeHY2URL(hy2 HY2) string {
	// 如果没有设置 Name，则使用 Host:Port 作为 Fragment
	if hy2.Name == "" {
		hy2.Name = fmt.Sprintf("%s:%d", hy2.Host, hy2.Port)
	}
	u := url.URL{
		Scheme:   "hysteria2",
		User:     url.User(hy2.Password),
		Host:     net.JoinHostPort(strings.Trim(hy2.Host, "[]"), strconv.Itoa(hy2.Port)),
		Fragment: hy2.Name,
	}
	q := u.Query()
	q.Set("security", "tls")
	q.Set("insecure", strconv.Itoa(hy2.Insecure))
	q.Set("sni", hy2.Sni)
	q.Set("pinSHA256", hy2.PinSHA256)
	if hy2.Certificate != "" {
		q.Set("subl-cert", base64.RawURLEncoding.EncodeToString([]byte(hy2.Certificate)))
	}
	q.Set("alpn", strings.Join(hy2.ALPN, ","))
	q.Set("obfs", hy2.Obfs)
	q.Set("obfs-password", hy2.ObfsPassword)
	q.Set("peer", hy2.Peer)
	q.Set("auth", hy2.Auth)
	q.Set("upmbps", strconv.Itoa(hy2.UpMbps))
	q.Set("downmbps", strconv.Itoa(hy2.DownMbps))
	// q.Set("alpn", hy2.ALPN)
	// 检查query是否有空值，有的话删除
	for k, v := range q {
		if v[0] == "" {
			delete(q, k)
			// fmt.Printf("k: %v, v: %v\n", k, v)
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// hy2 解码
func DecodeHY2URL(s string) (HY2, error) {
	u, err := url.Parse(s)
	if err != nil {
		return HY2{}, fmt.Errorf("解析失败的URL: %s,错误:%s", s, err)
	}
	if u.Scheme != "hy2" && u.Scheme != "hysteria2" {
		return HY2{}, fmt.Errorf("非hy2协议: %s", s)
	}
	pin := u.Query().Get("pinSHA256")
	var certificate string
	if pin != "" {
		pin, err = certpin.Normalize(pin)
		if err != nil {
			return HY2{}, err
		}
	}
	if encoded := u.Query().Get("subl-cert"); encoded != "" {
		if len(encoded) > 12000 {
			return HY2{}, fmt.Errorf("公有证书过大")
		}
		raw, e := base64.RawURLEncoding.DecodeString(encoded)
		if e != nil {
			return HY2{}, fmt.Errorf("公有证书编码无效")
		}
		certificate = string(raw)
		if e = certpin.Validate(certificate, pin, u.Query().Get("sni")); e != nil {
			return HY2{}, e
		}
	}
	password := u.User.Username()
	server := u.Hostname()
	port, _ := strconv.Atoi(u.Port())
	insecure, _ := strconv.Atoi(u.Query().Get("insecure"))
	auth := u.Query().Get("auth")
	upMbps, _ := strconv.Atoi(u.Query().Get("upmbps"))
	downMbps, _ := strconv.Atoi(u.Query().Get("downmbps"))
	alpns := u.Query().Get("alpn")
	alpn := strings.Split(alpns, ",")
	if alpns == "" {
		alpn = nil
	}
	sni := u.Query().Get("sni")
	obfs := u.Query().Get("obfs")
	obfsPassword := u.Query().Get("obfs-password")
	name := u.Fragment
	// 如果没有设置 Name，则使用 Host:Port 作为 Fragment
	if name == "" {
		name = server + ":" + u.Port()
	}
	if CheckEnvironment() {
		fmt.Println("password:", password)
		fmt.Println("server:", server)
		fmt.Println("port:", port)
		fmt.Println("insecure:", insecure)
		fmt.Println("auth:", auth)
		fmt.Println("upMbps:", upMbps)
		fmt.Println("downMbps:", downMbps)
		fmt.Println("alpn:", alpn)
		fmt.Println("sni:", sni)
		fmt.Println("obfs:", obfs)
		fmt.Println("obfsPassword:", obfsPassword)
		fmt.Println("name:", name)
	}
	return HY2{
		PinSHA256: pin, Certificate: certificate,
		Password:     password,
		Host:         server,
		Port:         port,
		Insecure:     insecure,
		Auth:         auth,
		UpMbps:       upMbps,
		DownMbps:     downMbps,
		ALPN:         alpn,
		Name:         name,
		Sni:          sni,
		Obfs:         obfs,
		ObfsPassword: obfsPassword,
	}, nil
}
