package api

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"ppeelink/models"
	"ppeelink/node"
	"ppeelink/xraycore"
	"strings"
	"time"
)

func nodeEgress(n models.Node) (*xraycore.EgressSpec, error) {
	if n.PausedUntil != nil && n.PausedUntil.After(time.Now()) {
		return nil, fmt.Errorf("出口节点已暂停")
	}
	var flags node.NodeFlags
	if n.Flags != "" {
		if err := json.Unmarshal([]byte(n.Flags), &flags); err != nil {
			return nil, fmt.Errorf("出口节点参数无效")
		}
	}
	if flags.DialerProxyID != 0 || flags.SkipCertVerify || flags.TFO {
		return nil, fmt.Errorf("出口节点不能附带客户端链式代理、跳过证书或 TFO 设置")
	}
	if !strings.HasPrefix(n.Link, "vless://") {
		return nil, fmt.Errorf("固定出口首版支持 VLESS Reality TCP/XHTTP 节点")
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
		if _, err := nodeEgress(n); err == nil {
			out = append(out, gin.H{"id": n.ID, "name": n.Name})
		}
	}
	c.JSON(200, gin.H{"code": "00000", "data": out})
}
