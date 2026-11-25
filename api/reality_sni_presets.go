package api

import (
	"encoding/json"
	"net"
	"strings"

	"ppeelink/models"
)

// regionalRealitySNIPool 经实测验证（TLS 1.3 + HTTP/2 + 非 Cloudflare CDN）的各地区本土 Reality 伪装站。
// 欧区已剔除套 Cloudflare CDN 的 www.nokia.com，替换为非 CF 的 www.siemens.com。
var regionalRealitySNIPool = map[string][]string{
	"US": {
		"www.ebay.com",
		"www.airbnb.com",
		"www.ups.com",
		"www.stanford.edu",
		"www.harvard.edu",
	},
	"CA": {
		"www.utoronto.ca",
		"www.ubc.ca",
		"www.ebay.com",
		"www.airbnb.com",
		"www.ups.com",
	},
	"JP": {
		"www.sony.jp",
		"www.yahoo.co.jp",
		"www.honda.co.jp",
		"www.yamaha.com",
		"www.lovelive-anime.jp",
	},
	"HK": {
		"www.mandarinoriental.com",
		"www.k11musea.com",
		"www.pacificplace.com.hk",
		"www.hko.gov.hk",
		"www.hku.hk",
	},
	"SG": {
		"www.grab.com",
		"www.changiairport.com",
		"nus.edu.sg",
		"www.smu.edu.sg",
		"www.sutd.edu.sg",
	},
	"TW": {
		"www.asus.com",
		"www.msi.com",
		"www.gigabyte.com",
		"www.evaair.com",
		"www.gskill.com",
	},
	"KR": {
		"www.naver.com",
		"www.samsung.com",
		"www.koreanair.com",
		"www.coupang.com",
		"www.cj.net",
	},
	"EU": {
		"www.adidas.com",
		"www.siemens.com",
		"www.mercedes-benz.com",
		"www.tudelft.nl",
		"www.postgresql.org",
	},
	"GB": {
		"www.virginatlantic.com",
		"www.marksandspencer.com",
		"www.unilever.com",
		"www.arm.com",
		"www.cam.ac.uk",
	},
	"GLOBAL": {
		"www.gentoo.org",
		"www.python.org",
		"www.debian.org",
		"www.kernel.org",
		"addons.mozilla.org",
	},
}

var euCountryCodes = map[string]bool{
	"DE": true, "FR": true, "NL": true, "IT": true, "ES": true,
	"SE": true, "FI": true, "NO": true, "CH": true, "PL": true,
	"AT": true, "BE": true, "IE": true, "DK": true, "CZ": true,
	"PT": true, "LU": true, "RO": true, "HU": true, "EU": true,
}

func normalizeSNIRegionCode(raw string) string {
	s := strings.ToUpper(strings.TrimSpace(raw))
	if s == "" {
		return ""
	}
	switch s {
	case "US", "USA", "UNITED STATES", "美国":
		return "US"
	case "CA", "CAN", "CANADA", "加拿大":
		return "CA"
	case "JP", "JPN", "JAPAN", "日本":
		return "JP"
	case "HK", "HKG", "HONG KONG", "香港":
		return "HK"
	case "SG", "SGP", "SINGAPORE", "新加坡", "狮城":
		return "SG"
	case "TW", "TWN", "TAIWAN", "台湾", "台灣":
		return "TW"
	case "KR", "KOR", "KOREA", "韩国", "韓國":
		return "KR"
	case "GB", "UK", "GBR", "UNITED KINGDOM", "英国", "英國":
		return "GB"
	}
	if euCountryCodes[s] {
		return "EU"
	}
	// Substring heuristics for server names like "NoSLA_JP", "DMIT_USA", "AsiaCLoud_HK".
	checks := []struct {
		code   string
		tokens []string
	}{
		{"HK", []string{"香港", "HONGKONG", "_HK", "-HK", " HK"}},
		{"JP", []string{"日本", "东京", "大阪", "JAPAN", "TOKYO", "_JP", "-JP", " JP"}},
		{"US", []string{"美国", "洛杉矶", "圣何塞", "西雅图", "USA", "_US", "-US", " US"}},
		{"SG", []string{"新加坡", "狮城", "SINGAPORE", "_SG", "-SG", " SG"}},
		{"TW", []string{"台湾", "台灣", "台北", "TAIWAN", "_TW", "-TW", " TW"}},
		{"KR", []string{"韩国", "首尔", "KOREA", "SEOUL", "_KR", "-KR", " KR"}},
		{"CA", []string{"加拿大", "多伦多", "温哥华", "CANADA", "_CA", "-CA", " CA"}},
		{"GB", []string{"英国", "伦敦", "LONDON", "_UK", "-UK", "_GB", "-GB"}},
		{"EU", []string{"德国", "法国", "荷兰", "欧洲", "法兰克福", "阿姆斯特丹", "DE", "FR", "NL", "EU"}},
	}
	for _, c := range checks {
		for _, tok := range c.tokens {
			if strings.Contains(s, tok) {
				return c.code
			}
		}
	}
	if strings.HasSuffix(s, "HK") {
		return "HK"
	}
	if strings.HasSuffix(s, "JP") {
		return "JP"
	}
	if strings.HasSuffix(s, "US") {
		return "US"
	}
	if strings.HasSuffix(s, "SG") {
		return "SG"
	}
	if strings.HasSuffix(s, "TW") {
		return "TW"
	}
	if strings.HasSuffix(s, "KR") {
		return "KR"
	}
	return ""
}

func resolveServerSNIRegion(serverID uint) string {
	if models.DB == nil || serverID == 0 {
		return "GLOBAL"
	}
	var srv models.ProxyServer
	if models.DB.First(&srv, serverID).Error != nil {
		return "GLOBAL"
	}
	if srv.UnlockJSON != "" {
		var rep serverOpsUnlockReport
		if json.Unmarshal([]byte(srv.UnlockJSON), &rep) == nil && rep.Country != "" {
			if code := normalizeSNIRegionCode(rep.Country); code != "" {
				return code
			}
		}
	}
	var mon models.Monitor
	if models.DB.Select("group").Where("proxy_server_id = ?", serverID).First(&mon).Error == nil && mon.Group != "" {
		if code := normalizeSNIRegionCode(mon.Group); code != "" {
			return code
		}
	}
	if code := normalizeSNIRegionCode(srv.Name); code != "" {
		return code
	}
	return "GLOBAL"
}

func cleanPanelSNIHost(raw string) string {
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

// buildRegionalSNITargets 组合用户自定义目标 + 对应地区 5 个本土站 + 全球兜底站（最多 12 个供 VPS 就地体检）。
func buildRegionalSNITargets(region string, custom []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, 12)
	add := func(raw string) {
		h := cleanPanelSNIHost(raw)
		if h == "" || seen[h] || len(out) >= 12 {
			return
		}
		seen[h] = true
		out = append(out, h)
	}
	for _, c := range custom {
		add(c)
	}
	if local, ok := regionalRealitySNIPool[region]; ok {
		for _, h := range local {
			add(h)
		}
	}
	for _, h := range regionalRealitySNIPool["GLOBAL"] {
		add(h)
	}
	return out
}
