package api

import (
	"ppeelink/models"
	"ppeelink/node"
	"strings"
)

// renderClientNodes propagates export errors so clients keep their previous config.
func renderClientNodes(client string, nodes []models.Node, configs node.SqlConfig) ([]byte, string, string, error) {
	urls, flags := collectNodeInputs(nodes)
	urls, _ = node.FilterClientLinks(urls, client)
	if len(urls) == 0 {
		return nil, "txt", "text/plain; charset=utf-8", node.ErrNoCompatibleNodes
	}
	switch clientFormat(client) {
	case "clash":
		b, e := node.EncodeClashWithFlags(urls, flags, configs)
		return b, "yaml", "text/plain; charset=utf-8", e
	case "clash-proxies":
		b, e := node.EncodeClashProxies(urls, flags, configs)
		return b, "yaml", "text/plain; charset=utf-8", e
	case "surge":
		s, e := node.EncodeSurgeWithFlags(urls, flags, configs)
		return []byte(s), "conf", "text/plain; charset=utf-8", e
	case "loon":
		s, e := node.EncodeLoonWithFlags(urls, flags, configs)
		return []byte(s), "conf", "text/plain; charset=utf-8", e
	case "singbox":
		s, e := node.EncodeSingboxWithFlags(urls, flags, configs)
		return []byte(s), "json", "application/json; charset=utf-8", e
	case "qx":
		s, e := node.EncodeQX(urls, configs)
		return []byte(s), "conf", "text/plain; charset=utf-8", e
	default:
		return []byte(node.Base64Encode(strings.Join(urls, "\n") + "\n")), "txt", "text/plain; charset=utf-8", nil
	}
}
