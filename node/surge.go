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

func EncodeSurge(urls []string, sqlconfig SqlConfig) (string, error) {
	return EncodeSurgeWithFlags(urls, nil, sqlconfig)
}

// EncodeSurgeWithFlags 与 EncodeSurge 相同，但允许按节点覆盖 udp/skip-cert-verify。
func EncodeSurgeWithFlags(urls []string, flags map[string]NodeFlags, sqlconfig SqlConfig) (string, error) {
	urls, _ = FilterClientLinks(urls, "surge")
	if len(urls) == 0 {
		return "", ErrNoCompatibleNodes
	}
	var proxys, groups []string
	for _, link := range urls {
		udp := sqlconfig.Udp
		cert := sqlconfig.Cert
		if f, ok := flags[link]; ok {
			udp, cert = f.UDP, f.SkipCertVerify
		}
		Scheme := strings.Split(link, "://")[0]
		switch {
		case Scheme == "ss":
			ss, err := DecodeSSURL(link)
			if err != nil {
				log.Println(err)
				continue
			}
			proxy := map[string]interface{}{
				"name":     ss.Name,
				"server":   ss.Server,
				"port":     ss.Port,
				"cipher":   ss.Param.Cipher,
				"password": ss.Param.Password,
				"udp":      udp,
			}
			ssproxy := fmt.Sprintf("%s = ss, %s, %d, encrypt-method=%s, password=%s, udp-relay=%t",
				proxy["name"], proxy["server"], proxy["port"], proxy["cipher"], proxy["password"], proxy["udp"])
			groups = append(groups, ss.Name)
			proxys = append(proxys, ssproxy)
		case Scheme == "vmess":
			vmess, err := DecodeVMESSURL(link)
			if err != nil {
				log.Println(err)
				continue
			}
			tls := false
			if vmess.Tls != "none" && vmess.Tls != "" {
				tls = true
			}
			port, _ := convertToInt(vmess.Port)
			proxy := map[string]interface{}{
				"name":             vmess.Ps,
				"server":           vmess.Add,
				"port":             port,
				"uuid":             vmess.Id,
				"tls":              tls,
				"network":          vmess.Net,
				"ws-path":          vmess.Path,
				"ws-host":          vmess.Host,
				"udp":              udp,
				"skip-cert-verify": cert,
			}
			vmessproxy := fmt.Sprintf("%s = vmess, %s, %d, username=%s , tls=%t, vmess-aead=true,  udp-relay=%t , skip-cert-verify=%t",
				proxy["name"], proxy["server"], proxy["port"], proxy["uuid"], proxy["tls"], proxy["udp"], proxy["skip-cert-verify"])
			if vmess.Net == "ws" {
				vmessproxy = fmt.Sprintf("%s, ws=true,ws-path=%s", vmessproxy, proxy["ws-path"])
				if vmess.Host != "" && vmess.Host != "none" {
					vmessproxy = fmt.Sprintf("%s, ws-headers=Host:%s", vmessproxy, proxy["ws-host"])
				}
			}
			if vmess.Sni != "" {
				vmessproxy = fmt.Sprintf("%s, sni=%s", vmessproxy, vmess.Sni)
			}
			groups = append(groups, vmess.Ps)
			proxys = append(proxys, vmessproxy)
		case Scheme == "trojan":
			trojan, err := DecodeTrojanURL(link)
			if err != nil {
				log.Println(err)
				continue
			}
			proxy := map[string]interface{}{
				"name":             trojan.Name,
				"server":           trojan.Hostname,
				"port":             trojan.Port,
				"password":         trojan.Password,
				"udp":              udp,
				"skip-cert-verify": cert,
			}
			trojanproxy := fmt.Sprintf("%s = trojan, %s, %d, password=%s, udp-relay=%t, skip-cert-verify=%t",
				proxy["name"], proxy["server"], proxy["port"], proxy["password"], proxy["udp"], proxy["skip-cert-verify"])
			if trojan.Query.Sni != "" {
				trojanproxy = fmt.Sprintf("%s, sni=%s", trojanproxy, trojan.Query.Sni)

			}
			groups = append(groups, trojan.Name)
			proxys = append(proxys, trojanproxy)
		case Scheme == "hysteria2" || Scheme == "hy2":
			hy2, err := DecodeHY2URL(link)
			if err != nil {
				log.Println(err)
				continue
			}

			proxy := map[string]interface{}{
				"name":             hy2.Name,
				"server":           hy2.Host,
				"port":             hy2.Port,
				"password":         hy2.Password,
				"udp":              udp,
				"skip-cert-verify": cert || hy2.Insecure != 0,
			}
			hy2proxy := fmt.Sprintf("%s = hysteria2, %s, %d, password=%s, udp-relay=%t, skip-cert-verify=%t",
				proxy["name"], proxy["server"], proxy["port"], proxy["password"], proxy["udp"], proxy["skip-cert-verify"])
			if hy2.Sni != "" {
				hy2proxy = fmt.Sprintf("%s, sni=%s", hy2proxy, hy2.Sni)

			}
			if hy2.PinSHA256 != "" {
				pin, e := certpin.Normalize(hy2.PinSHA256)
				if e != nil {
					return "", fmt.Errorf("HY2 证书指纹无效")
				}
				hy2proxy = strings.Replace(hy2proxy, "skip-cert-verify=true", "skip-cert-verify=false", 1)
				hy2proxy += ", server-cert-fingerprint-sha256=" + pin
			}
			if hy2.Obfs != "" {
				hy2proxy += fmt.Sprintf(", salamander-password=%s", hy2.ObfsPassword)
			}
			groups = append(groups, hy2.Name)
			proxys = append(proxys, hy2proxy)
		case Scheme == "tuic":
			tuic, err := DecodeTuicURL(link)
			if err != nil {
				log.Println(err)
				continue
			}
			proxy := map[string]interface{}{
				"name":             tuic.Name,
				"server":           tuic.Host,
				"port":             tuic.Port,
				"password":         tuic.Password,
				"udp":              udp,
				"skip-cert-verify": cert,
			}
			tuicproxy := fmt.Sprintf("%s = tuic, %s, %d, token=%s, udp-relay=%t, skip-cert-verify=%t",
				proxy["name"], proxy["server"], proxy["port"], proxy["password"], proxy["udp"], proxy["skip-cert-verify"])
			groups = append(groups, tuic.Name)
			proxys = append(proxys, tuicproxy)
		}
	}
	return DecodeSurge(proxys, groups, sqlconfig.Surge)
}
func DecodeSurge(proxys, groups []string, file string) (string, error) {
	var surge []byte
	var err error
	if strings.Contains(file, "://") {
		resp, err := remoteFetchClient.Get(file)
		if err != nil {
			log.Println("http.Get error", err)
			return "", err
		}
		defer resp.Body.Close()
		surge, err = io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("error: %v", err)
			return "", err
		}
	} else {
		surge, err = os.ReadFile(file)
		if err != nil {
			log.Println(err)
			return "", err
		}
	}

	raw := string(surge)
	proxyPart := replaceSurgeSection(raw, "Proxy", strings.Join(proxys, "\n"))
	groupPart := transformSurgeSection(proxyPart, "Proxy Group", func(body string) string {
		lines := strings.Split(body, "\n")
		grouplist := strings.Join(groups, ", ")
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") || !strings.Contains(trimmed, "=") {
				continue
			}
			// 支持 Xboard/subconverter 风格 filter(正则) 过滤节点
			// 例如: 💬 OpenAi = select, 🚀 节点选择, filter("(?i)openai|chatgpt")
			filterRe, hasFilter := parseSurgeFilter(trimmed)
			if hasFilter {
				cleaned := removeSurgeFilter(trimmed)
				cleaned = strings.TrimRight(cleaned, ", ")
				for _, g := range groups {
					if filterRe.MatchString(g) && !strings.Contains(cleaned, g) {
						cleaned += ", " + g
					}
				}
				lines[i] = cleaned
			} else if !strings.Contains(trimmed, "include-all-proxies") && grouplist != "" {
				lines[i] = strings.TrimRight(trimmed, ", ") + ", " + grouplist
			}
		}
		return strings.Join(lines, "\n")
	})

	return groupPart, nil
}

func replaceSurgeSection(raw, section, body string) string {
	re := regexp.MustCompile(`(?m)^\s*\[[^]\r\n]+\][ \t]*\r?$`)
	spans := re.FindAllStringIndex(raw, -1)
	for i, span := range spans {
		if strings.TrimSpace(raw[span[0]:span[1]]) == "["+section+"]" {
			end := len(raw)
			if i+1 < len(spans) {
				end = spans[i+1][0]
			}
			existing := strings.TrimSpace(raw[span[1]:end])
			merged := body
			if existing != "" {
				if merged != "" {
					merged = body + "\n" + existing
				} else {
					merged = existing
				}
			}
			return raw[:span[0]] + "[" + section + "]\n" + merged + "\n\n" + raw[end:]
		}
	}
	return strings.TrimRight(raw, "\r\n") + "\n\n[" + section + "]\n" + body + "\n"
}

func transformSurgeSection(raw, section string, fn func(string) string) string {
	re := regexp.MustCompile(`(?m)^\s*\[[^]\r\n]+\][ \t]*\r?$`)
	spans := re.FindAllStringIndex(raw, -1)
	for i, span := range spans {
		if strings.TrimSpace(raw[span[0]:span[1]]) == "["+section+"]" {
			end := len(raw)
			if i+1 < len(spans) {
				end = spans[i+1][0]
			}
			body := strings.Trim(raw[span[1]:end], "\r\n")
			updated := fn(body)
			return raw[:span[0]] + "[" + section + "]\n" + updated + "\n\n" + raw[end:]
		}
	}
	return raw
}

// parseSurgeFilter 从 surge 分组行中提取 filter(正则) 标记
// 返回编译后的正则与是否命中标记。支持写法：
//   - filter("(?i)US|USA")
//   - filter((?i)US|USA)
var surgeFilterMark = regexp.MustCompile(`filter\(\s*(?:"([^"]*)"|'([^']*)'|([^)]*))\s*\)`)

func parseSurgeFilter(line string) (*regexp.Regexp, bool) {
	m := surgeFilterMark.FindStringSubmatch(line)
	if m == nil {
		return nil, false
	}
	pat := m[1]
	if pat == "" {
		pat = m[2]
	}
	if pat == "" {
		pat = m[3]
	}
	pat = strings.TrimSpace(pat)
	if pat == "" {
		return nil, false
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		log.Printf("surge filter 正则编译失败 %q: %v", pat, err)
		return nil, false
	}
	return re, true
}

// removeSurgeFilter 移除行内的 filter(...) 标记
func removeSurgeFilter(line string) string {
	return strings.TrimSpace(surgeFilterMark.ReplaceAllString(line, ""))
}
