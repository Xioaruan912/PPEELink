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
	if in.EgressNodeID == 0 {
		return nil, nil
	}
	var servers []models.ProxyServer
	var inbounds []models.ProxyInbound
	if err := models.DB.Find(&servers).Error; err != nil {
		return nil, err
	}
	if err := models.DB.Find(&inbounds).Error; err != nil {
		return nil, err
	}
	byServer := map[uint]models.ProxyServer{}
	for _, s := range servers {
		byServer[s.ID] = s
	}
	for i := range inbounds {
		if in.ID != 0 && inbounds[i].ID == in.ID {
			inbounds[i] = in
		}
	}
	if in.ID == 0 {
		inbounds = append(inbounds, in)
	}
	seen := map[int]bool{}
	var walk func(models.ProxyInbound, int) (*xraycore.EgressSpec, error)
	walk = func(current models.ProxyInbound, depth int) (*xraycore.EgressSpec, error) {
		if depth > 32 || seen[current.EgressNodeID] {
			return nil, fmt.Errorf("出口链路存在循环或过长")
		}
		seen[current.EgressNodeID] = true
		defer delete(seen, current.EgressNodeID)
		var n models.Node
		if err := models.DB.First(&n, current.EgressNodeID).Error; err != nil {
			return nil, fmt.Errorf("出口节点不存在，请重新选择")
		}
		e, err := nodeEgress(n)
		if err != nil {
			return nil, err
		}
		for _, next := range inbounds {
			host := byServer[next.ServerID].PublicHost
			if strings.EqualFold(strings.Trim(host, "[]"), strings.Trim(e.Address, "[]")) && next.Port == e.Port {
				if next.ServerID == server.ID {
					return nil, fmt.Errorf("出口不能指回入口服务器")
				}
				if next.EgressNodeID != 0 {
					if _, err := walk(next, depth+1); err != nil {
						return nil, err
					}
				}
			}
		}
		return e, nil
	}
	return walk(in, 0)
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
