package api

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/url"
	"ppeelink/models"
	"ppeelink/node"
	"ppeelink/xraycore"
	"strings"
	"time"
)

func nodeEgress(n models.Node) (*xraycore.EgressSpec, error) {
	if len(n.Link) > 16384 {
		return nil, fmt.Errorf("出口链接过长")
	}
	if strings.HasPrefix(n.Source, "agent-relay:") {
		return nil, fmt.Errorf("纯端口转发节点不能作为网关出口，请选择真实落地节点")
	}
	if n.PausedUntil != nil && n.PausedUntil.After(time.Now()) {
		return nil, fmt.Errorf("出口节点已暂停")
	}
	var flags node.NodeFlags
	if n.Flags != "" {
		if err := json.Unmarshal([]byte(n.Flags), &flags); err != nil {
			return nil, fmt.Errorf("出口节点参数无效")
		}
	}
	if flags.SkipCertVerify || flags.TFO {
		return nil, fmt.Errorf("出口节点不能跳过证书校验或附带 TFO 设置")
	}
	if strings.HasPrefix(n.Link, "hy2://") || strings.HasPrefix(n.Link, "hysteria2://") {
		return hy2NodeEgress(n.Link)
	}
	if !strings.HasPrefix(n.Link, "vless://") {
		return nil, fmt.Errorf("固定出口支持 VLESS Reality TCP/XHTTP 和安全 HY2 节点")
	}
	// The general subscription decoder is permissive. A gateway must not
	// silently drop transport/security settings when selecting a physical node.
	u, err := url.Parse(n.Link)
	if err != nil || u.User == nil || u.Opaque != "" || (u.Path != "" && u.Path != "/") {
		return nil, fmt.Errorf("出口节点链接无效")
	}
	if _, hasPassword := u.User.Password(); hasPassword {
		return nil, fmt.Errorf("出口 Reality 链接不支持密码参数")
	}
	params, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return nil, fmt.Errorf("出口节点链接参数无效")
	}
	allowed := map[string]bool{"security": true, "type": true, "sni": true, "fp": true, "sid": true, "pbk": true, "flow": true, "encryption": true, "headerType": true, "path": true, "host": true, "serviceName": true, "alpn": true, "mode": true}
	for key, values := range params {
		if !allowed[key] || len(values) != 1 {
			return nil, fmt.Errorf("出口 Reality 包含不支持或重复的参数：%s", key)
		}
	}
	v, err := node.DecodeVLESSURL(n.Link)
	if err != nil {
		return nil, fmt.Errorf("出口节点链接无效")
	}
	q := v.Query
	if q.Security != "reality" || q.HeaderType != "" && q.HeaderType != "none" || q.Encryption != "" && q.Encryption != "none" || q.Host != "" || q.ServiceName != "" || len(q.Alpn) > 0 || q.Mode != "" && q.Mode != "auto" {
		return nil, fmt.Errorf("出口节点包含不支持的传输参数")
	}
	transport := q.Type
	if transport == "" || transport == "tcp" {
		transport = "raw"
	}
	e := &xraycore.EgressSpec{Address: v.Server, Port: v.Port, UUID: v.Uuid, SNI: q.Sni, PublicKey: q.Pbk, ShortID: q.Sid, Transport: transport, Flow: q.Flow, Path: q.Path, Fingerprint: q.Fp}
	if err := e.Validate(); err != nil {
		return nil, err
	}
	return e, nil
}

func hy2NodeEgress(link string) (*xraycore.EgressSpec, error) {
	if len(link) > 16384 {
		return nil, fmt.Errorf("出口链接过长")
	}
	u, err := url.Parse(link)
	if err != nil || u.User == nil || u.Opaque != "" || (u.Path != "" && u.Path != "/") {
		return nil, fmt.Errorf("出口 HY2 链接无效")
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return nil, fmt.Errorf("出口 HY2 链接参数无效")
	}
	allowed := map[string]bool{"security": true, "sni": true, "insecure": true, "pinSHA256": true, "subl-cert": true, "alpn": true, "obfs": true, "obfs-password": true, "upmbps": true, "downmbps": true}
	for key, values := range q {
		if !allowed[key] || len(values) != 1 {
			return nil, fmt.Errorf("出口 HY2 包含不支持或重复的参数：%s", key)
		}
	}
	if security := q.Get("security"); security != "" && security != "tls" {
		return nil, fmt.Errorf("HY2 必须使用 TLS")
	}
	if value := q.Get("insecure"); value != "" && value != "0" && value != "1" {
		return nil, fmt.Errorf("出口 HY2 insecure 参数无效")
	}
	for _, key := range []string{"upmbps", "downmbps"} {
		if value := q.Get(key); value != "" && value != "0" {
			return nil, fmt.Errorf("出口 HY2 暂不支持带宽参数")
		}
	}
	// DecodeHY2URL validates the optional embedded certificate. Use the effective SNI for that validation.
	if q.Get("sni") == "" {
		q.Set("sni", u.Hostname())
		u.RawQuery = q.Encode()
	}
	hy, err := node.DecodeHY2URL(u.String())
	if err != nil {
		return nil, fmt.Errorf("出口 HY2 链接或证书无效")
	}
	// Hysteria authentication may contain a literal colon; URL userinfo preserves it.
	password, err := url.PathUnescape(u.User.String())
	if err != nil {
		return nil, fmt.Errorf("出口 HY2 密码无效")
	}
	e := &xraycore.EgressSpec{Protocol: "hysteria2", Insecure: hy.Insecure == 1, Address: hy.Host, Port: hy.Port, Password: password, SNI: hy.Sni, PinSHA256: hy.PinSHA256, Obfs: hy.Obfs, ObfsPassword: hy.ObfsPassword, ALPN: hy.ALPN}
	if err := e.Validate(); err != nil {
		return nil, err
	}
	return e, nil
}

func resolveInboundEgress(in models.ProxyInbound, server models.ProxyServer) (*xraycore.EgressSpec, error) {
	e, _, err := resolveInboundRoutes(in, server)
	return e, err
}
func ProxyEgressOptions(c *gin.Context) {
	var nodes []models.Node
	if err := models.DB.Order("name asc").Find(&nodes).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取出口节点失败"})
		return
	}
	out := []gin.H{}
	for _, n := range nodes {
		if e, err := nodeEgress(n); err == nil {
			protocol := "vless"
			if e.IsHY2() {
				protocol = "hysteria2"
			}
			out = append(out, gin.H{"id": n.ID, "name": n.Name, "protocol": protocol})
		}
	}
	c.JSON(200, gin.H{"code": "00000", "data": out})
}
