package node

import (
	_ "embed"
	"encoding/json"
)

// ChinaTarget 中国各地 TCP 测试目标
type ChinaTarget struct {
	Province string  `json:"province"`
	City     string  `json:"city"`
	ISP      string  `json:"isp"`
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
	IP       string  `json:"ip"`
	Port     int     `json:"port"`
}

// chinaTargetsJSON 中国各地运营商 TCP 测试目标清单（全量，来自用户提供清单）。
//
//go:embed data/china_targets.json
var chinaTargetsJSON []byte

// chinaTargets 启动时从内嵌清单解析；清单损坏属于构建错误，直接终止。
var chinaTargets = func() []ChinaTarget {
	var list []ChinaTarget
	if err := json.Unmarshal(chinaTargetsJSON, &list); err != nil {
		panic("china_targets.json 无效: " + err.Error())
	}
	return list
}()

var zstaticcdnHost = "lf3-ips.zstaticcdn.com"

// FilterChinaTargets 按省/运营商过滤。
// provinces/isps 均为空时：默认每省每运营商取 1 条（去重）。
func FilterChinaTargets(provinces, isps []string) []ChinaTarget {
	if len(provinces) == 0 && len(isps) == 0 {
		return defaultChinaTargets()
	}
	var out []ChinaTarget
	seen := map[string]bool{}
	for _, t := range chinaTargets {
		if len(provinces) > 0 && !contains(provinces, t.Province) {
			continue
		}
		if len(isps) > 0 && !contains(isps, t.ISP) {
			continue
		}
		key := t.Province + "|" + t.ISP
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, t)
	}
	if len(out) == 0 {
		return defaultChinaTargets()
	}
	return out
}

// defaultChinaTargets 返回每省每运营商 1 条
func defaultChinaTargets() []ChinaTarget {
	seen := map[string]bool{}
	var out []ChinaTarget
	for _, t := range chinaTargets {
		key := t.Province + "|" + t.ISP
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, t)
	}
	return out
}

func contains(arr []string, s string) bool {
	for _, v := range arr {
		if v == s {
			return true
		}
	}
	return false
}
