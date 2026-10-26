package api

import (
	"fmt"
	"ppeelink/models"
	"ppeelink/xraycore"
	"strings"
)

func normalizeInboundDomainRoutes(rules []models.DomainRoute) ([]models.DomainRoute, error) {
	if len(rules) > 32 {
		return nil, fmt.Errorf("网站分流最多 32 条规则")
	}
	out := append([]models.DomainRoute(nil), rules...)
	seen := map[string]bool{}
	for i := range out {
		r := &out[i]
		var d string
		var e error
		if r.Match == "category" {
			d, e = xraycore.NormalizeRouteCategory(r.Domain)
		} else {
			d, e = xraycore.NormalizeRouteDomain(r.Domain)
		}
		if e != nil {
			return nil, fmt.Errorf("第 %d 条分流规则：%w", i+1, e)
		}
		r.Domain = d
		if r.Match != "exact" && r.Match != "suffix" && r.Match != "category" || r.EgressNodeID < 0 {
			return nil, fmt.Errorf("第 %d 条分流规则参数无效", i+1)
		}
		key := r.Match + ":" + r.Domain
		if seen[key] {
			return nil, fmt.Errorf("第 %d 条分流规则重复", i+1)
		}
		seen[key] = true
	}
	return out, nil
}
func inboundExitIDs(in models.ProxyInbound) []int {
	ids := []int{}
	if in.EgressNodeID != 0 {
		ids = append(ids, in.EgressNodeID)
	}
	for _, r := range in.DomainRoutes {
		if r.EgressNodeID != 0 {
			ids = append(ids, r.EgressNodeID)
		}
	}
	return ids
}
func resolveInboundRoutes(in models.ProxyInbound, server models.ProxyServer) (*xraycore.EgressSpec, []xraycore.DomainRouteSpec, error) {
	normalized, err := normalizeInboundDomainRoutes(in.DomainRoutes)
	if err != nil {
		return nil, nil, err
	}
	in.DomainRoutes = normalized
	var servers []models.ProxyServer
	var inbounds []models.ProxyInbound
	cache := map[int]*xraycore.EgressSpec{}
	if len(inboundExitIDs(in)) > 0 {
		if err = models.DB.Find(&servers).Error; err != nil {
			return nil, nil, err
		}
		if err = models.DB.Find(&inbounds).Error; err != nil {
			return nil, nil, err
		}
		byServer := map[uint]models.ProxyServer{}
		for _, s := range servers {
			byServer[s.ID] = s
		}
		found := false
		for i := range inbounds {
			if in.ID != 0 && inbounds[i].ID == in.ID {
				inbounds[i] = in
				found = true
			}
		}
		if !found {
			inbounds = append(inbounds, in)
		}
		visiting, done := map[int]bool{}, map[int]bool{}
		var walk func(int, int) error
		walk = func(id, depth int) error {
			if depth > 32 || visiting[id] {
				return fmt.Errorf("出口链路存在循环或过长")
			}
			if done[id] {
				return nil
			}
			visiting[id] = true
			defer delete(visiting, id)
			var n models.Node
			if e := models.DB.First(&n, id).Error; e != nil {
				return fmt.Errorf("出口节点不存在，请重新选择")
			}
			e, err := nodeEgress(n)
			if err != nil {
				return err
			}
			cache[id] = e
			for _, next := range inbounds {
				host := byServer[next.ServerID].PublicHost
				if strings.EqualFold(strings.Trim(host, "[]"), strings.Trim(e.Address, "[]")) && next.Port == e.Port {
					if next.ServerID == server.ID {
						return fmt.Errorf("出口不能指回入口服务器")
					}
					for _, nextID := range inboundExitIDs(next) {
						if err := walk(nextID, depth+1); err != nil {
							return err
						}
					}
				}
			}
			done[id] = true
			return nil
		}
		for _, id := range inboundExitIDs(in) {
			if err := walk(id, 0); err != nil {
				return nil, nil, err
			}
		}
	}
	var routes []xraycore.DomainRouteSpec
	for _, r := range normalized {
		routes = append(routes, xraycore.DomainRouteSpec{Domain: r.Domain, Match: r.Match, Egress: cache[r.EgressNodeID]})
	}
	return cache[in.EgressNodeID], routes, nil
}

func needsCategoryCapability(rules []models.DomainRoute) bool {
	for _, rule := range rules {
		if rule.Match == "category" {
			return true
		}
	}
	return false
}
