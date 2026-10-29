package node

import (
	"fmt"
	"io"
	"log"
	"os"
	"ppeelink/certpin"
	"regexp"
	"strings"
)

// EncodeLoon 将节点链接列表转为 Loon 配置文本（填充 [Proxy] 段）。
// 策略组靠模板内的 [Remote Filter] NameRegex 自动筛选，不做组填充。
func EncodeLoon(urls []string, sqlconfig SqlConfig) (string, error) {
	return EncodeLoonWithFlags(urls, nil, sqlconfig)
}

// EncodeLoonWithFlags 与 EncodeLoon 相同，但允许按节点覆盖 udp/skip-cert-verify。
func EncodeLoonWithFlags(urls []string, flags map[string]NodeFlags, sqlconfig SqlConfig) (string, error) {
	// 未配置 Loon 模板时使用默认本地模板
	if sqlconfig.Loon == "" {
		sqlconfig.Loon = "./template/loon.conf"
	}
	var proxys []string
	relations := map[string]string{}
	for _, link := range urls {
		udp := sqlconfig.Udp
		cert := sqlconfig.Cert
		if f, ok := flags[link]; ok {
			udp, cert = f.UDP, f.SkipCertVerify
		}
		if f, ok := flags[link]; ok && f.DialerProxy != "" {
			parsed, err := ParseOutbound(link)
			if err != nil {
				return "", fmt.Errorf("中转节点解析失败")
			}
			relations[parsed.Name] = f.DialerProxy
		}
		scheme := strings.Split(link, "://")[0]
		switch {
		case scheme == "ss":
			ss, err := DecodeSSURL(link)
			if err != nil {
				log.Println(err)
				continue
			}
			p := fmt.Sprintf("%s = Shadowsocks,%s,%d,%s,\"%s\",fast-open=false,udp=%t",
				ss.Name, ss.Server, ss.Port, ss.Param.Cipher, ss.Param.Password, udp)
			proxys = append(proxys, p)
		case scheme == "vmess":
			v, err := DecodeVMESSURL(link)
			if err != nil {
				log.Println(err)
				continue
			}
			port, _ := convertToInt(v.Port)
			tls := v.Tls != "none" && v.Tls != ""
			cipher := v.Scy
			if cipher == "" {
				cipher = "auto"
			}
			p := fmt.Sprintf("%s = vmess,%s,%d,%s,\"%s\",transport=%s,alterId=0,over-tls=%t,udp=%t",
				v.Ps, v.Add, port, cipher, v.Id, v.Net, tls, udp)
			if v.Net == "ws" {
				p += fmt.Sprintf(",path=%s", v.Path)
				if v.Host != "" && v.Host != "none" {
					p += fmt.Sprintf(",host=%s", v.Host)
				}
			}
			if tls && v.Sni != "" {
				p += fmt.Sprintf(",sni=%s", v.Sni)
			}
			if cert {
				p += ",skip-cert-verify=true"
			}
			proxys = append(proxys, p)
		case scheme == "vless":
			v, err := DecodeVLESSURL(link)
			if err != nil {
				log.Println(err)
				continue
			}
			transport := v.Query.Type
			if transport == "" {
				transport = "tcp"
			}
			tls := v.Query.Security != "" && v.Query.Security != "none"
			p := fmt.Sprintf("%s = VLESS,%s,%d,\"%s\",transport=%s,over-tls=%t,udp=%t",
				v.Name, v.Server, v.Port, v.Uuid, transport, tls, udp)
			// Reality / XTLS Vision
			if v.Query.Flow != "" {
				p += fmt.Sprintf(",flow=%s", v.Query.Flow)
			}
			if v.Query.Pbk != "" {
				p += fmt.Sprintf(",public-key=\"%s\"", v.Query.Pbk)
			}
			if v.Query.Sid != "" {
				p += fmt.Sprintf(",short-id=%s", v.Query.Sid)
			}
			if transport == "ws" {
				p += fmt.Sprintf(",path=%s", v.Query.Path)
				if v.Query.Host != "" {
					p += fmt.Sprintf(",host=%s", v.Query.Host)
				}
			}
			if v.Query.Sni != "" {
				p += fmt.Sprintf(",sni=%s", v.Query.Sni)
			}
			if cert {
				p += ",skip-cert-verify=true"
			}
			proxys = append(proxys, p)
		case scheme == "trojan":
			t, err := DecodeTrojanURL(link)
			if err != nil {
				log.Println(err)
				continue
			}
			p := fmt.Sprintf("%s = trojan,%s,%d,\"%s\",udp=%t",
				t.Name, t.Hostname, t.Port, t.Password, udp)
			if t.Query.Type != "" && t.Query.Type != "tcp" {
				p += fmt.Sprintf(",transport=%s", t.Query.Type)
				if t.Query.Path != "" {
					p += fmt.Sprintf(",path=%s", t.Query.Path)
				}
				if t.Query.Host != "" {
					p += fmt.Sprintf(",host=%s", t.Query.Host)
				}
			}
			if t.Query.Sni != "" {
				p += fmt.Sprintf(",sni=%s", t.Query.Sni)
			}
			if cert {
				p += ",skip-cert-verify=true"
			}
			proxys = append(proxys, p)
		case scheme == "hysteria2" || scheme == "hy2":
			h, err := DecodeHY2URL(link)
			if err != nil {
				log.Println(err)
				continue
			}

			p := fmt.Sprintf("%s = Hysteria2,%s,%d,\"%s\",udp=%t",
				h.Name, h.Host, h.Port, h.Password, udp)
			if h.Sni != "" {
				p += fmt.Sprintf(",sni=%s", h.Sni)
			}
			if h.Obfs != "" {
				p += fmt.Sprintf(",salamander-password=\"%s\"", h.ObfsPassword)
			}
			if h.PinSHA256 != "" {
				pin, e := certpin.Normalize(h.PinSHA256)
				if e != nil {
					return "", fmt.Errorf("HY2 证书指纹无效")
				}
				p += ",skip-cert-verify=false,tls-cert-sha256=" + pin
			} else if cert {
				p += ",skip-cert-verify=true"
			}
			proxys = append(proxys, p)
		// Loon 不支持 tuic，跳过
		case scheme == "tuic":
			continue
		}
	}
	proxys, chains, names, err := loonChains(proxys, relations)
	if err != nil {
		return "", err
	}
	out, err := DecodeLoon(proxys, sqlconfig.Loon)
	if err != nil || len(chains) == 0 {
		return out, err
	}
	// Chains are full-profile policies; remote node-only subscriptions cannot carry this section.
	out = appendLoonSection(out, "Proxy Chain", strings.Join(chains, "\n"))
	group := "中转线路"
	for strings.Contains(out, group+" =") {
		group += "_"
	}
	out = appendLoonSection(out, "Proxy Group", group+" = select,"+strings.Join(names, ","))
	lines := strings.Split(out, "\n")
	inGroups := false
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "[") {
			inGroups = trim == "[Proxy Group]"
			continue
		}
		if inGroups && strings.Contains(line, "=") {
			left, right, _ := strings.Cut(line, "=")
			typ, rest, ok := strings.Cut(strings.TrimSpace(right), ",")
			if strings.EqualFold(strings.TrimSpace(typ), "select") && strings.TrimSpace(left) != group {
				if ok {
					lines[i] = left + "= select," + group + "," + rest
				} else {
					lines[i] = left + "= select," + group
				}
				break
			}
		}
	}
	return strings.Join(lines, "\n"), nil
}

// DecodeLoon 读取 Loon 模板（本地文件或 URL），将 [Proxy] 段替换为生成的节点。
func DecodeLoon(proxys []string, file string) (string, error) {
	var raw []byte
	var err error
	if strings.Contains(file, "://") {
		resp, err := remoteFetchClient.Get(file)
		if err != nil {
			log.Println("http.Get error", err)
			return "", err
		}
		defer resp.Body.Close()
		raw, err = io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
	} else {
		raw, err = os.ReadFile(file)
		if err != nil {
			return "", err
		}
	}

	return replaceLoonSection(string(raw), "Proxy", strings.Join(proxys, "\n")), nil
}

func replaceLoonSection(raw, section, body string) string {
	re := regexp.MustCompile(`(?m)^\s*\[[^]\r\n]+\][ \t]*\r?$`)
	spans := re.FindAllStringIndex(raw, -1)
	for i, span := range spans {
		if strings.TrimSpace(raw[span[0]:span[1]]) == "["+section+"]" {
			end := len(raw)
			if i+1 < len(spans) {
				end = spans[i+1][0]
			}
			return raw[:span[0]] + "[" + section + "]\n" + body + "\n\n" + raw[end:]
		}
	}
	return strings.TrimRight(raw, "\r\n") + "\n\n[" + section + "]\n" + body + "\n"
}
func appendLoonSection(raw, section, body string) string {
	marker := "[" + section + "]"
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == marker {
			lines[i] = line + "\n" + body
			return strings.Join(lines, "\n")
		}
	}
	return replaceLoonSection(raw, section, body)
}
func loonChains(proxies []string, relations map[string]string) ([]string, []string, []string, error) {
	byName := map[string]int{}
	rawName := map[string]string{}
	for i, line := range proxies {
		n, _, ok := strings.Cut(line, " = ")
		if ok {
			byName[n] = i
			rawName[n] = n
		}
	}
	for n := range relations {
		if _, ok := byName[n]; !ok {
			return nil, nil, nil, fmt.Errorf("Loon 无法输出中转落地节点 %s", n)
		}
	}
	// These are direct support nodes, not the selectable relay policies. Keep
	// their names human-readable and country-neutral so region filters do not
	// accidentally select them as relayed Japan/US nodes.
	used := map[string]bool{}
	for n := range byName {
		used[n] = true
	}
	nextAlias := 1
	for _, line := range proxies {
		n, _, _ := strings.Cut(line, " = ")
		if relations[n] == "" {
			continue
		}
		alias := ""
		for {
			alias = fmt.Sprintf("链路底层 %d（直连）", nextAlias)
			nextAlias++
			if !used[alias] {
				break
			}
		}
		used[alias] = true
		rawName[n] = alias
	}
	chains := []string{}
	names := []string{}
	for _, line := range proxies {
		n, _, _ := strings.Cut(line, " = ")
		if relations[n] == "" {
			continue
		}
		path := []string{}
		seen := map[string]bool{}
		cur := n
		for cur != "" {
			if seen[cur] || len(path) >= 4 {
				return nil, nil, nil, fmt.Errorf("Loon 中转路径循环或超过 4 跳")
			}
			seen[cur] = true
			if _, ok := byName[cur]; !ok {
				return nil, nil, nil, fmt.Errorf("Loon 中转入口 %s 未输出，无法生成完整路径", cur)
			}
			if strings.ContainsAny(cur, ",\r\n") {
				return nil, nil, nil, fmt.Errorf("Loon 中转节点名称不能含逗号或换行")
			}
			path = append(path, rawName[cur])
			cur = relations[cur]
		}
		for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
			path[i], path[j] = path[j], path[i]
		}
		chains = append(chains, n+" = "+strings.Join(path, ",")+",udp=true")
		names = append(names, n)
		// QUIC to a later HY2 hop requires UDP forwarding on each earlier hop.
		for member := range seen {
			idx := byName[member]
			proxies[idx] = strings.ReplaceAll(proxies[idx], "udp=false", "udp=true")
			if !strings.Contains(proxies[idx], "block-quic=") {
				proxies[idx] += ",block-quic=false"
			}
		}
	}
	for n, alias := range rawName {
		if n != alias {
			idx := byName[n]
			_, rest, _ := strings.Cut(proxies[idx], " = ")
			proxies[idx] = alias + " = " + rest
		}
	}
	return proxies, chains, names, nil
}
