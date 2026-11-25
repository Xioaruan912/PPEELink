package node

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type AppRule struct {
	Platform  string   `json:"platform"`
	App       string   `json:"app,omitempty"`
	Preset    string   `json:"preset,omitempty"`
	Label     string   `json:"label,omitempty"`
	Processes []string `json:"processes,omitempty"`
	Packages  []string `json:"packages,omitempty"`
	Target    string   `json:"target"`
}

var processName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._ +()-]{0,127}$`)
var packageName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*(\.[a-zA-Z][a-zA-Z0-9_]*)+$`)

func expandAppRuleEntries(r AppRule, idx int) (processes []string, packages []string, err error) {
	if r.Target == "" || len(r.Target) > 255 || strings.ContainsAny(r.Target, ",\r\n\x00") {
		return nil, nil, fmt.Errorf("第 %d 条应用规则的目标无效", idx+1)
	}

	if len(r.Processes) > 0 || len(r.Packages) > 0 {
		seenProc := map[string]bool{}
		for _, p := range r.Processes {
			if p == "" || p != strings.TrimSpace(p) || !processName.MatchString(p) {
				return nil, nil, fmt.Errorf("第 %d 条应用规则的进程名无效: %s", idx+1, p)
			}
			if !seenProc[p] {
				seenProc[p] = true
				processes = append(processes, p)
			}
		}
		seenPkg := map[string]bool{}
		for _, pkg := range r.Packages {
			if pkg == "" || pkg != strings.TrimSpace(pkg) || len(pkg) > 255 || !packageName.MatchString(pkg) {
				return nil, nil, fmt.Errorf("第 %d 条应用规则的包名无效: %s", idx+1, pkg)
			}
			if !seenPkg[pkg] {
				seenPkg[pkg] = true
				packages = append(packages, pkg)
			}
		}
		if len(processes) == 0 && len(packages) == 0 {
			return nil, nil, fmt.Errorf("第 %d 条应用规则至少包含一个进程或包名", idx+1)
		}
		return processes, packages, nil
	}

	valid := (r.Platform == "desktop" && processName.MatchString(r.App)) ||
		(r.Platform == "android" && len(r.App) <= 255 && packageName.MatchString(r.App))
	if !valid || r.App != strings.TrimSpace(r.App) {
		return nil, nil, fmt.Errorf("第 %d 条应用规则的应用名称或平台无效", idx+1)
	}
	if r.Platform == "android" {
		return nil, []string{r.App}, nil
	}
	return []string{r.App}, nil, nil
}

func ValidateAppRules(rules []AppRule) error {
	if len(rules) > 64 {
		return fmt.Errorf("应用规则最多 64 条")
	}
	totalEntries := 0
	for i, r := range rules {
		procs, pkgs, err := expandAppRuleEntries(r, i)
		if err != nil {
			return err
		}
		totalEntries += len(procs) + len(pkgs)
		if totalEntries > 512 {
			return fmt.Errorf("应用规则展开后的进程与包名总数不能超过 512 项")
		}
	}
	return nil
}

// ApplyApplicationRules adds first-match application rules to supported complete configurations.
func ApplyApplicationRules(client string, body []byte, cfg SqlConfig) ([]byte, error) {
	if len(cfg.AppRules) == 0 {
		return body, nil
	}
	if err := ValidateAppRules(cfg.AppRules); err != nil {
		return nil, err
	}
	if client != "clash" && client != "singbox" {
		return nil, fmt.Errorf("应用规则仅支持 Mihomo 和 sing-box 完整配置")
	}
	var doc map[string]any
	if client == "clash" {
		if err := yaml.Unmarshal(body, &doc); err != nil {
			return nil, err
		}
	} else {
		if err := json.Unmarshal(body, &doc); err != nil {
			return nil, err
		}
	}
	if doc == nil {
		return nil, fmt.Errorf("配置为空")
	}
	targets := map[string]bool{"DIRECT": true, "REJECT": true, "PROXY": true}
	proxyNames := []string{}
	if client == "clash" {
		if list, ok := doc["proxies"].([]any); ok {
			for _, item := range list {
				if m, ok := item.(map[string]any); ok {
					if n, ok := m["name"].(string); ok {
						targets[n] = true
						proxyNames = append(proxyNames, n)
					}
				}
			}
		}
		if list, ok := doc["proxy-groups"].([]any); ok {
			for _, item := range list {
				if m, ok := item.(map[string]any); ok {
					if n, ok := m["name"].(string); ok {
						targets[n] = true
					}
				}
			}
		}
	} else {
		if list, ok := doc["outbounds"].([]any); ok {
			for _, item := range list {
				if m, ok := item.(map[string]any); ok {
					if n, ok := m["tag"].(string); ok {
						targets[n] = true
					}
				}
			}
		}
	}
	for i, r := range cfg.AppRules {
		if !targets[r.Target] {
			return nil, fmt.Errorf("第 %d 条应用规则的目标不在当前配置中", i+1)
		}
	}
	if client == "clash" {
		proxyTag := "应用代理"
		for targets[proxyTag] {
			proxyTag += "_"
		}
		needsProxy := false
		for _, r := range cfg.AppRules {
			needsProxy = needsProxy || r.Target == "PROXY"
		}
		if needsProxy {
			g := map[string]any{"name": proxyTag, "type": "select"}
			if len(proxyNames) > 0 {
				g["proxies"] = proxyNames
			} else if providers, ok := doc["proxy-providers"].(map[string]any); ok && len(providers) > 0 {
				uses := []string{}
				for name := range providers {
					uses = append(uses, name)
				}
				sort.Strings(uses)
				g["use"] = uses
			} else {
				return nil, fmt.Errorf("没有可用于应用代理的节点")
			}
			groups, _ := doc["proxy-groups"].([]any)
			doc["proxy-groups"] = append(groups, g)
		}
		generated := []any{}
		seenRule := map[string]bool{}
		for i, r := range cfg.AppRules {
			target := r.Target
			if target == "PROXY" {
				target = proxyTag
			}
			procs, pkgs, err := expandAppRuleEntries(r, i)
			if err != nil {
				return nil, err
			}
			for _, item := range append(procs, pkgs...) {
				line := "PROCESS-NAME," + item + "," + target
				if seenRule[line] {
					continue
				}
				seenRule[line] = true
				generated = append(generated, line)
			}
		}
		old, _ := doc["rules"].([]any)
		doc["rules"] = append(generated, old...)
		doc["mode"] = "rule"
		doc["find-process-mode"] = "always"
		return yaml.Marshal(doc)
	}
	route, _ := doc["route"].(map[string]any)
	if route == nil {
		route = map[string]any{}
		doc["route"] = route
	}
	generated := []any{}
	for i, r := range cfg.AppRules {
		procs, pkgs, err := expandAppRuleEntries(r, i)
		if err != nil {
			return nil, err
		}
		target := r.Target
		action := "route"
		switch target {
		case "REJECT":
			action = "reject"
		case "DIRECT":
			target = "direct"
		case "PROXY":
			target = "proxy"
		}
		if action != "reject" && !targets[target] {
			return nil, fmt.Errorf("应用规则缺少对应的出站")
		}
		appendSingboxRule := func(key string, vals []string) {
			if len(vals) == 0 {
				return
			}
			rule := map[string]any{key: vals, "action": action}
			if action != "reject" {
				rule["outbound"] = target
			}
			generated = append(generated, rule)
		}
		appendSingboxRule("process_name", procs)
		appendSingboxRule("package_name", pkgs)
	}
	old, _ := route["rules"].([]any)
	route["rules"] = append(generated, old...)
	route["find_process"] = true
	return json.MarshalIndent(doc, "", "  ")
}
