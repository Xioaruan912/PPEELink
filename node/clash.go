package node

import (
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Proxy struct {
	Fingerprint        string                 `yaml:"fingerprint,omitempty"`
	Name               string                 `yaml:"name,omitempty"`
	Type               string                 `yaml:"type,omitempty"`
	Server             string                 `yaml:"server,omitempty"`
	Port               int                    `yaml:"port,omitempty"`
	Cipher             string                 `yaml:"cipher,omitempty"`
	Password           string                 `yaml:"password,omitempty"`
	Client_fingerprint string                 `yaml:"client-fingerprint,omitempty"`
	Tfo                bool                   `yaml:"tfo,omitempty"`
	Udp                bool                   `yaml:"udp,omitempty"`
	Skip_cert_verify   bool                   `yaml:"skip-cert-verify,omitempty"`
	Tls                bool                   `yaml:"tls,omitempty"`
	Servername         string                 `yaml:"servername,omitempty"`
	Flow               string                 `yaml:"flow,omitempty"`
	AlterId            string                 `yaml:"alterId,omitempty"`
	Network            string                 `yaml:"network,omitempty"`
	Reality_opts       map[string]interface{} `yaml:"reality-opts,omitempty"`
	Ws_opts            map[string]interface{} `yaml:"ws-opts,omitempty"`
	Grpc_opts          map[string]interface{} `yaml:"grpc-opts,omitempty"`
	Auth_str           string                 `yaml:"auth_str,omitempty"`
	Auth               string                 `yaml:"auth,omitempty"`
	Up                 int                    `yaml:"up,omitempty"`
	Down               int                    `yaml:"down,omitempty"`
	Alpn               []string               `yaml:"alpn,omitempty"`
	Sni                string                 `yaml:"sni,omitempty"`
	Obfs               string                 `yaml:"obfs,omitempty"`
	Obfs_password      string                 `yaml:"obfs-password,omitempty"`
	Protocol           string                 `yaml:"protocol,omitempty"`
	Uuid               string                 `yaml:"uuid,omitempty"`
	Peer               string                 `yaml:"peer,omitempty"`
	Congestion_control string                 `yaml:"congestion_control,omitempty"`
	Udp_relay_mode     string                 `yaml:"udp_relay_mode,omitempty"`
	Disable_sni        bool                   `yaml:"disable_sni,omitempty"`
	Username           string                 `yaml:"username,omitempty"`
	Psk                string                 `yaml:"psk,omitempty"`
	Version            string                 `yaml:"version,omitempty"`
	Obfs_opts          map[string]interface{} `yaml:"obfs-opts,omitempty"`
	Ip                 string                 `yaml:"ip,omitempty"`
	Private_key        string                 `yaml:"private-key,omitempty"`
	Public_key         string                 `yaml:"public-key,omitempty"`
	Mtu                int                    `yaml:"mtu,omitempty"`
	Reserved           []int                  `yaml:"reserved,omitempty"`
	Dns                []string               `yaml:"dns,omitempty"`
}

type ProxyGroup struct {
	Proxies []string `yaml:"proxies"`
}
type Config struct {
	Proxies      []Proxy      `yaml:"proxies"`
	Proxy_groups []ProxyGroup `yaml:"proxy-groups"`
}

// 删除opts中的空值
func DeleteOpts(opts map[string]interface{}) {
	for k, v := range opts {
		switch v := v.(type) {
		case string:
			if v == "" {
				delete(opts, k)
			}
		case map[string]interface{}:
			DeleteOpts(v)
			if len(v) == 0 {
				delete(opts, k)
			}
		}
	}
}
func convertToInt(value interface{}) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case float64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	default:
		return 0, fmt.Errorf("unexpected type %T", v)
	}
}

// EncodeClash 用于生成 Clash 配置文件
func EncodeClash(urls []string, sqlconfig SqlConfig) ([]byte, error) {
	return EncodeClashWithFlags(urls, nil, sqlconfig)
}

// EncodeClashWithFlags 与 EncodeClash 相同，但允许按节点覆盖 udp/tfo/skip-cert-verify。
func EncodeClashWithFlags(urls []string, flags map[string]NodeFlags, sqlconfig SqlConfig) ([]byte, error) {
	proxys := BuildClashProxies(urls, flags, sqlconfig)
	if len(proxys) == 0 {
		return nil, ErrNoCompatibleNodes
	}
	return DecodeClash(proxys, sqlconfig.Clash)
}

// EncodeClashProxies 只输出 proxies 列表，供 proxy-provider 拉取。
func EncodeClashProxies(urls []string, flags map[string]NodeFlags, sqlconfig SqlConfig) ([]byte, error) {
	proxys := BuildClashProxies(urls, flags, sqlconfig)
	if len(proxys) == 0 {
		return nil, ErrNoCompatibleNodes
	}
	return yaml.Marshal(map[string]interface{}{"proxies": proxys})
}

// EncodeClashMinimal 不依赖模板的最小 Clash 配置：全部节点放进一个手动选择组，其余直连规则交给客户端默认。
func EncodeClashMinimal(urls []string, flags map[string]NodeFlags) ([]byte, error) {
	proxys := BuildClashProxies(urls, flags, SqlConfig{Udp: true})
	if len(proxys) == 0 {
		return nil, ErrNoCompatibleNodes
	}
	names := make([]string, 0, len(proxys))
	for _, p := range proxys {
		names = append(names, p.Name)
	}
	return yaml.Marshal(map[string]interface{}{
		"mixed-port": 7890,
		"mode":       "rule",
		"proxies":    proxys,
		"proxy-groups": []map[string]interface{}{
			{"name": "PROXY", "type": "select", "proxies": append(names, "DIRECT")},
		},
		"rules": []string{"GEOIP,LAN,DIRECT", "GEOIP,CN,DIRECT", "MATCH,PROXY"},
	})
}

// BuildClashProxies 将分享链接转换为 Clash 代理定义。
func BuildClashProxies(urls []string, flags map[string]NodeFlags, sqlconfig SqlConfig) []Proxy {
	urls, _ = FilterClientLinks(urls, "clash")
	// 传入urls，解析urls，生成proxys
	// yamlfile 为模板文件
	var proxys []Proxy

	for _, link := range urls {
		udp := sqlconfig.Udp
		cert := sqlconfig.Cert
		tfo := false
		if f, ok := flags[link]; ok {
			udp, cert, tfo = f.UDP, f.SkipCertVerify, f.TFO
		}
		before := len(proxys)
		Scheme := strings.Split(link, "://")[0]
		switch {
		case Scheme == "ss":
			ss, err := DecodeSSURL(link)
			if err != nil {
				log.Println(err)
				continue
			}
			// 如果没有名字，就用服务器地址作为名字
			if ss.Name == "" {
				ss.Name = fmt.Sprintf("%s:%d", ss.Server, ss.Port)
			}
			ssproxy := Proxy{
				Name:             ss.Name,
				Type:             "ss",
				Server:           ss.Server,
				Port:             ss.Port,
				Cipher:           ss.Param.Cipher,
				Password:         ss.Param.Password,
				Udp:              udp,
				Skip_cert_verify: cert,
			}
			proxys = append(proxys, ssproxy)
		case Scheme == "ssr":
			ssr, err := DecodeSSRURL(link)
			if err != nil {
				log.Println(err)
			}
			// 如果没有名字，就用服务器地址作为名字
			if ssr.Qurey.Remarks == "" {
				ssr.Qurey.Remarks = fmt.Sprintf("%s:%d", ssr.Server, ssr.Port)
			}
			ssrproxy := Proxy{
				Name:             ssr.Qurey.Remarks,
				Type:             "ssr",
				Server:           ssr.Server,
				Port:             ssr.Port,
				Cipher:           ssr.Method,
				Password:         ssr.Password,
				Obfs:             ssr.Obfs,
				Obfs_password:    ssr.Qurey.Obfsparam,
				Protocol:         ssr.Protocol,
				Udp:              udp,
				Skip_cert_verify: cert,
			}
			proxys = append(proxys, ssrproxy)
		case Scheme == "trojan":
			trojan, err := DecodeTrojanURL(link)
			if err != nil {
				log.Println(err)
				continue
			}
			// 如果没有名字，就用服务器地址作为名字
			if trojan.Name == "" {
				trojan.Name = fmt.Sprintf("%s:%d", trojan.Hostname, trojan.Port)
			}
			ws_opts := map[string]interface{}{
				"path": trojan.Query.Path,
				"headers": map[string]interface{}{
					"Host": trojan.Query.Host,
				},
			}
			DeleteOpts(ws_opts)
			trojanproxy := Proxy{
				Name:               trojan.Name,
				Type:               "trojan",
				Server:             trojan.Hostname,
				Port:               trojan.Port,
				Password:           trojan.Password,
				Client_fingerprint: trojan.Query.Fp,
				Sni:                trojan.Query.Sni,
				Network:            trojan.Query.Type,
				Flow:               trojan.Query.Flow,
				Alpn:               trojan.Query.Alpn,
				Ws_opts:            ws_opts,
				Udp:                udp,
				Skip_cert_verify:   cert,
			}
			proxys = append(proxys, trojanproxy)
		case Scheme == "vmess":
			vmess, err := DecodeVMESSURL(link)
			if err != nil {
				log.Println(err)
				continue
			}
			// 如果没有名字，就用服务器地址作为名字
			if vmess.Ps == "" {
				vmess.Ps = fmt.Sprintf("%s:%s", vmess.Add, vmess.Port)
			}
			ws_opts := map[string]interface{}{
				"path": vmess.Path,
				"headers": map[string]interface{}{
					"Host": vmess.Host,
				},
			}
			DeleteOpts(ws_opts)
			tls := false
			if vmess.Tls != "none" && vmess.Tls != "" {
				tls = true
			}
			port, _ := convertToInt(vmess.Port)
			aid, _ := convertToInt(vmess.Aid)
			vmessproxy := Proxy{
				Name:             vmess.Ps,
				Type:             "vmess",
				Server:           vmess.Add,
				Port:             port,
				Cipher:           vmess.Scy,
				Uuid:             vmess.Id,
				AlterId:          strconv.Itoa(aid),
				Network:          vmess.Net,
				Tls:              tls,
				Ws_opts:          ws_opts,
				Udp:              udp,
				Skip_cert_verify: cert,
			}
			proxys = append(proxys, vmessproxy)
		case Scheme == "vless":
			vless, err := DecodeVLESSURL(link)
			if err != nil {
				log.Println(err)
				continue
			}
			// 如果没有名字，就用服务器地址作为名字
			if vless.Name == "" {
				vless.Name = fmt.Sprintf("%s:%d", vless.Server, vless.Port)
			}
			ws_opts := map[string]interface{}{
				"path": vless.Query.Path,
				"headers": map[string]interface{}{
					"Host": vless.Query.Host,
				},
			}
			reality_opts := map[string]interface{}{
				"public-key": vless.Query.Pbk,
				"short-id":   vless.Query.Sid,
			}
			grpc_opts := map[string]interface{}{
				"grpc-mode":         "gun",
				"grpc-service-name": vless.Query.ServiceName,
			}
			if vless.Query.Mode == "multi" {
				grpc_opts["grpc-mode"] = "multi"
			}
			DeleteOpts(ws_opts)
			DeleteOpts(reality_opts)
			DeleteOpts(grpc_opts)
			tls := false
			if vless.Query.Security != "" {
				tls = true
			}
			if vless.Query.Security == "none" {
				tls = false
			}
			vlessproxy := Proxy{
				Name:               vless.Name,
				Type:               "vless",
				Server:             vless.Server,
				Port:               vless.Port,
				Servername:         vless.Query.Sni,
				Uuid:               vless.Uuid,
				Client_fingerprint: vless.Query.Fp,
				Network:            vless.Query.Type,
				Flow:               vless.Query.Flow,
				Alpn:               vless.Query.Alpn,
				Ws_opts:            ws_opts,
				Reality_opts:       reality_opts,
				Grpc_opts:          grpc_opts,
				Udp:                udp,
				Skip_cert_verify:   cert,
				Tls:                tls,
			}
			proxys = append(proxys, vlessproxy)
		case Scheme == "hy" || Scheme == "hysteria":
			hy, err := DecodeHYURL(link)
			if err != nil {
				log.Println(err)
				continue
			}
			// 如果没有名字，就用服务器地址作为名字
			if hy.Name == "" {
				hy.Name = fmt.Sprintf("%s:%d", hy.Host, hy.Port)
			}
			hyproxy := Proxy{
				Name:             hy.Name,
				Type:             "hysteria",
				Server:           hy.Host,
				Port:             hy.Port,
				Auth_str:         hy.Auth,
				Up:               hy.UpMbps,
				Down:             hy.DownMbps,
				Alpn:             hy.ALPN,
				Peer:             hy.Peer,
				Udp:              udp,
				Skip_cert_verify: cert,
			}
			proxys = append(proxys, hyproxy)
		case Scheme == "hy2" || Scheme == "hysteria2":
			hy2, err := DecodeHY2URL(link)
			if err != nil {
				log.Println(err)
				continue
			}
			// 如果没有名字，就用服务器地址作为名字
			if hy2.Name == "" {
				hy2.Name = fmt.Sprintf("%s:%d", hy2.Host, hy2.Port)
			}
			hyproxy2 := Proxy{
				Fingerprint:      hy2.PinSHA256,
				Name:             hy2.Name,
				Type:             "hysteria2",
				Server:           hy2.Host,
				Port:             hy2.Port,
				Auth_str:         hy2.Auth,
				Sni:              hy2.Sni,
				Alpn:             hy2.ALPN,
				Obfs:             hy2.Obfs,
				Password:         hy2.Password,
				Obfs_password:    hy2.ObfsPassword,
				Udp:              udp,
				Skip_cert_verify: cert,
			}
			if hy2.PinSHA256 != "" {
				hyproxy2.Skip_cert_verify = false
			} else if hy2.Insecure != 0 {
				hyproxy2.Skip_cert_verify = true
			}
			proxys = append(proxys, hyproxy2)
		case Scheme == "tuic":
			tuic, err := DecodeTuicURL(link)
			if err != nil {
				log.Println(err)
				continue
			}
			// 如果没有名字，就用服务器地址作为名字
			if tuic.Name == "" {
				tuic.Name = fmt.Sprintf("%s:%d", tuic.Host, tuic.Port)
			}
			disable_sni := false
			if tuic.Disable_sni == 1 {
				disable_sni = true
			}
			tuicproxy := Proxy{
				Name:               tuic.Name,
				Type:               "tuic",
				Server:             tuic.Host,
				Port:               tuic.Port,
				Password:           tuic.Password,
				Uuid:               tuic.Uuid,
				Congestion_control: tuic.Congestion_control,
				Alpn:               tuic.Alpn,
				Udp_relay_mode:     tuic.Udp_relay_mode,
				Disable_sni:        disable_sni,
				Sni:                tuic.Sni,
				Udp:                udp,
				Skip_cert_verify:   cert,
			}
			proxys = append(proxys, tuicproxy)
		case Scheme == "anytls":
			a, err := DecodeAnyTLSURL(link)
			if err != nil {
				log.Println(err)
				continue
			}
			scv := cert
			if a.Insecure != 0 {
				scv = true
			}
			proxys = append(proxys, Proxy{Name: a.Name, Type: "anytls", Server: a.Host, Port: a.Port, Password: a.Password, Sni: a.Sni, Client_fingerprint: a.Fp, Skip_cert_verify: scv, Udp: udp})
		case Scheme == "snell":
			s, err := DecodeSnellURL(link)
			if err != nil {
				log.Println(err)
				continue
			}
			proxy := Proxy{Name: s.Name, Type: "snell", Server: s.Host, Port: s.Port, Psk: s.Psk, Version: s.Version, Udp: udp}
			if proxy.Version == "" {
				proxy.Version = "4"
			}
			if s.Obfs != "" {
				proxy.Obfs_opts = map[string]interface{}{"mode": s.Obfs, "host": s.ObfsHost}
			}
			proxys = append(proxys, proxy)
		case Scheme == "socks5" || Scheme == "socks5h" || Scheme == "http" || Scheme == "https":
			s, err := DecodeSocksURL(link)
			if err != nil {
				log.Println(err)
				continue
			}
			proxy := Proxy{Name: s.Name, Server: s.Host, Port: s.Port, Username: s.Username, Password: s.Password, Udp: udp}
			if s.Type == "http" {
				proxy.Type = "http"
			} else {
				proxy.Type = "socks5"
			}
			if s.Tls {
				proxy.Tls = true
				proxy.Skip_cert_verify = cert
			}
			proxys = append(proxys, proxy)
		case Scheme == "ssh":
			s, err := DecodeSSHURL(link)
			if err != nil {
				log.Println(err)
				continue
			}
			proxys = append(proxys, Proxy{Name: s.Name, Type: "ssh", Server: s.Host, Port: s.Port, Username: s.User, Password: s.Password, Udp: udp})
		case Scheme == "wireguard" || Scheme == "wg":
			w, err := DecodeWireGuardURL(link)
			if err != nil {
				log.Println(err)
				continue
			}
			proxy := Proxy{Name: w.Name, Type: "wireguard", Server: w.Host, Port: w.Port, Private_key: w.PrivateKey, Public_key: w.PublicKey, Ip: w.Address, Mtu: w.Mtu, Udp: true, Dns: []string{"1.1.1.1", "8.8.8.8"}}
			if w.Reserved != "" {
				proxy.Reserved = parseIntList(w.Reserved)
			}
			proxys = append(proxys, proxy)
		}
		if tfo && len(proxys) > before {
			proxys[len(proxys)-1].Tfo = true
		}
	}
	return proxys
}

// takeGlobalFingerprint 移除模板里的 global-client-fingerprint 并返回其值。
// 新版 mihomo 已删除该顶层配置（存在时拒绝加载），需要改为在每个节点上设置 client-fingerprint。
func takeGlobalFingerprint(config map[interface{}]interface{}) string {
	v, ok := config["global-client-fingerprint"]
	if !ok {
		return ""
	}
	delete(config, "global-client-fingerprint")
	fp, _ := v.(string)
	return strings.TrimSpace(fp)
}

// NormalizeRule 把一行规则整理成 mihomo 能接受的形式：去掉误存进来的 "- " 列表前缀和引号。
// 不像规则（没有逗号）的返回空串。
func NormalizeRule(raw string) string {
	r := strings.TrimSpace(raw)
	for strings.HasPrefix(r, "- ") {
		r = strings.TrimSpace(r[2:])
	}
	r = strings.Trim(r, "'\"")
	kind, _, found := strings.Cut(r, ",")
	if !found || !ruleType.MatchString(strings.TrimSpace(kind)) {
		return ""
	}
	return r
}

// ruleType 规则类型都是大写字母、数字与连字符，如 DOMAIN-SUFFIX、RULE-SET、GEOIP、MATCH。
var ruleType = regexp.MustCompile(`^[A-Z][A-Z0-9-]*$`)

// cleanRules 丢掉模板 rules 里不是规则的条目（例如被误存成规则的配置行），
// 否则 mihomo 会因 "format invalid" 拒绝整份配置。
func cleanRules(config map[interface{}]interface{}) {
	rules, ok := config["rules"].([]interface{})
	if !ok {
		return
	}
	kept := make([]interface{}, 0, len(rules))
	for _, r := range rules {
		text, ok := r.(string)
		if !ok {
			continue
		}
		if rule := NormalizeRule(text); rule != "" {
			kept = append(kept, rule)
		}
	}
	config["rules"] = kept
}

// usesTLS 节点是否走 TLS（指纹只对 TLS 握手有意义）。
func usesTLS(p Proxy) bool {
	switch p.Type {
	case "trojan", "anytls":
		return true
	case "vless", "vmess":
		return p.Tls || p.Reality_opts != nil
	}
	return false
}

// DecodeClash 用于解析 Clash 配置文件
// loadClashTemplate 读取本地或远程 Clash 模板。
func loadClashTemplate(yamlfile string) (map[interface{}]interface{}, error) {
	var data []byte
	var err error
	if strings.Contains(yamlfile, "://") {
		resp, err := remoteFetchClient.Get(yamlfile)
		if err != nil {
			log.Println("http.Get error", err)
			return nil, err
		}
		defer resp.Body.Close()
		data, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
	} else {
		data, err = os.ReadFile(yamlfile)
		if err != nil {
			log.Printf("error: %v", err)
			return nil, err
		}
	}
	config := make(map[interface{}]interface{})
	if err := yaml.Unmarshal(data, &config); err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return config, nil
}

// EncodeClashProvider 生成节点来自 proxy-provider 的 Clash 配置：客户端按 interval
// 自动刷新节点，无需重新下载整份配置。各策略组的 filter 对 provider 同样生效。
func EncodeClashProvider(yamlfile, providerName, providerURL, testURL string) ([]byte, error) {
	config, err := loadClashTemplate(yamlfile)
	if err != nil {
		return nil, err
	}
	takeGlobalFingerprint(config)
	cleanRules(config)
	if testURL == "" {
		testURL = "https://www.gstatic.com/generate_204"
	}
	providers, _ := config["proxy-providers"].(map[string]interface{})
	if providers == nil {
		providers = map[string]interface{}{}
	}
	providers[providerName] = map[string]interface{}{
		"type":     "http",
		"url":      providerURL,
		"interval": 3600,
		"path":     "./providers/" + providerName + ".yaml",
		"health-check": map[string]interface{}{
			"enable":   true,
			"url":      testURL,
			"interval": 600,
		},
	}
	config["proxy-providers"] = providers
	groups, _ := config["proxy-groups"].([]interface{})
	for i, pg := range groups {
		group, ok := pg.(map[string]interface{})
		if !ok {
			continue
		}
		if group["type"] == "relay" {
			break
		}
		uses, _ := group["use"].([]interface{})
		group["use"] = append(uses, providerName)
		if group["proxies"] == nil {
			group["proxies"] = []interface{}{}
		}
		groups[i] = group
	}
	config["proxy-groups"] = groups
	return yaml.Marshal(config)
}

// DecodeClash 用于解析 Clash 配置文件
func DecodeClash(proxys []Proxy, yamlfile string) ([]byte, error) {
	config, err := loadClashTemplate(yamlfile)
	if err != nil {
		return nil, err
	}

	fingerprint := takeGlobalFingerprint(config)
	cleanRules(config)

	// 检查 "proxies" 键是否存在于 config 中
	proxies, ok := config["proxies"].([]interface{})
	if !ok {
		// 如果 "proxies" 键不存在，创建一个新的切片
		proxies = []interface{}{}
	}
	// 定义一个代理列表名字
	ProxiesNameList := []string{}
	// 添加新代理
	for _, p := range proxys {
		ProxiesNameList = append(ProxiesNameList, p.Name)
		if fingerprint != "" && p.Client_fingerprint == "" && usesTLS(p) {
			p.Client_fingerprint = fingerprint
		}
		proxies = append(proxies, p)
	}
	// proxies = append(proxies, newProxy)
	config["proxies"] = proxies
	// 往ProxyGroup中插入代理列表
	// ProxiesNameList := []string{"newProxy", "ceshi"}
	proxyGroups := config["proxy-groups"].([]interface{})
	for i, pg := range proxyGroups {
		proxyGroup, ok := pg.(map[string]interface{})
		if !ok {
			continue
		}
		// 如果 proxyGroup["proxies"] 是 nil，初始化它为一个空的切片
		if proxyGroup["proxies"] == nil {
			proxyGroup["proxies"] = []interface{}{}
		}
		// 如果为链式代理的话则不插入返回
		// log.Print("代理类型为:", proxyGroup["type"])
		if proxyGroup["type"] == "relay" {
			break
		}
		// 清除 nil 值
		var validProxies []interface{}
		for _, p := range proxyGroup["proxies"].([]interface{}) {
			if p != nil {
				validProxies = append(validProxies, p)
			}
		}
		// Xboard 风格 filter 正则匹配：从 filter 字段读取正则，
		// 只把节点名匹配到的节点填入该 group。
		// 示例：filter: "(?i)US|USA|United States|美国"
		// include-all-providers: true 表示对整个订阅全部节点进行匹配过滤
		var filterRegex *regexp.Regexp
		if filterStr, ok := proxyGroup["filter"].(string); ok && filterStr != "" {
			if re, rerr := regexp.Compile(filterStr); rerr == nil {
				filterRegex = re
			} else {
				log.Printf("filter 正则编译失败 %q: %v", filterStr, rerr)
			}
		}
		if filterRegex != nil {
			for _, newProxy := range ProxiesNameList {
				if filterRegex.MatchString(newProxy) {
					validProxies = append(validProxies, newProxy)
				}
			}
		} else {
			// 无 filter 时保持原有全量填充行为
			for _, newProxy := range ProxiesNameList {
				validProxies = append(validProxies, newProxy)
			}
		}
		proxyGroup["proxies"] = validProxies
		proxyGroups[i] = proxyGroup
	}

	config["proxy-groups"] = proxyGroups

	// 将修改后的内容写回文件
	newData, err := yaml.Marshal(config)
	if err != nil {
		log.Printf("error: %v", err)
	}
	return newData, nil
}
