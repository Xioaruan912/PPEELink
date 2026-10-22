package api

import (
	"strings"

	"ppeelink/models"
	"ppeelink/node"
)

// renderClientNodes 按客户端渲染节点列表。
func renderClientNodes(client string, nodes []models.Node, configs node.SqlConfig) ([]byte, string, string) {
	urls, flags := collectNodeInputs(nodes)
	switch client {
	case "clash":
		b, err := node.EncodeClashWithFlags(urls, flags, configs)
		if err != nil {
			return []byte(err.Error()), "yaml", "text/plain; charset=utf-8"
		}
		return b, "yaml", "text/plain; charset=utf-8"
	case "surge":
		s, err := node.EncodeSurgeWithFlags(urls, flags, configs)
		if err != nil {
			return []byte(err.Error()), "conf", "text/plain; charset=utf-8"
		}
		return []byte(s), "conf", "text/plain; charset=utf-8"
	case "loon":
		s, err := node.EncodeLoonWithFlags(urls, flags, configs)
		if err != nil {
			return []byte(err.Error()), "conf", "text/plain; charset=utf-8"
		}
		return []byte(s), "conf", "text/plain; charset=utf-8"
	case "singbox":
		s, err := node.EncodeSingbox(urls, configs)
		if err != nil {
			return []byte(err.Error()), "json", "text/plain; charset=utf-8"
		}
		return []byte(s), "json", "application/json; charset=utf-8"
	case "qx":
		s, err := node.EncodeQX(urls, configs)
		if err != nil {
			return []byte(err.Error()), "conf", "text/plain; charset=utf-8"
		}
		return []byte(s), "conf", "text/plain; charset=utf-8"
	default: // v2ray / shadowrocket
		var b strings.Builder
		for _, l := range urls {
			b.WriteString(l)
			b.WriteString("\n")
		}
		return []byte(node.Base64Encode(b.String())), "txt", "text/html; charset=utf-8"
	}
}
