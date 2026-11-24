package api

import (
	"regexp"
	"strings"
	"time"
)

type parsedLogEntry struct {
	ID       int    `json:"id"`
	Time     string `json:"time"`     // HH:mm:ss
	FullTime string `json:"fullTime"` // YYYY-MM-DD HH:mm:ss
	Level    string `json:"level"`    // error | warn | info
	Module   string `json:"module"`   // Reality | Hysteria2 | VLESS | TLS | Agent | 系统 | Xray
	Message  string `json:"message"`
	Hint     string `json:"hint,omitempty"`
	Count    int    `json:"count"`
	Raw      string `json:"raw"`
}

type logDiagnosticFinding struct {
	Level   string `json:"level"` // error | warn | ok
	Title   string `json:"title"`
	Detail  string `json:"detail"`
	Count   int    `json:"count"`
}

type parsedLogSummary struct {
	Total    int                    `json:"total"`
	Errors   int                    `json:"errors"`
	Warnings int                    `json:"warnings"`
	Infos    int                    `json:"infos"`
	Findings []logDiagnosticFinding `json:"findings"`
}

var (
	// Matches journalctl -o short-iso: 2026-11-24T11:40:20+0000 hostname unit[pid]: body
	journalIsoRe = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[+-]\d{4})\s+\S+\s+[^\s:]+:\s*(.*)$`)
	// Matches Go / Xray inner timestamp: 2026/11/24 19:40:20(.123456) body
	innerGoTimeRe = regexp.MustCompile(`^(\d{4}/\d{2}/\d{2}\s+\d{2}:\d{2}:\d{2})(?:\.\d+)?\s+(.*)$`)
	// Matches Xray level tag: [Warning] / [Error] / [Info] / [Debug] (optional [connID])
	xrayLevelRe = regexp.MustCompile(`^\[(Error|Warning|Warn|Info|Debug)\]\s*(?:\[\d+\]\s*)?(.*)$`)
	// Matches Agent bracket tag: [agent] / [xray] / [terminal] / [Probe]
	agentTagRe = regexp.MustCompile(`^\[([A-Za-z0-9_-]+)\]\s*(.*)$`)
)

func parseStructuredLogs(raw string, source string) ([]parsedLogEntry, parsedLogSummary) {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	entries := make([]parsedLogEntry, 0, len(lines))
	summary := parsedLogSummary{Findings: []logDiagnosticFinding{}}

	findingCounts := map[string]*logDiagnosticFinding{}
	findingOrder := make([]string, 0, 6)

	addFinding := func(key, level, title, detail string) {
		if f, ok := findingCounts[key]; ok {
			f.Count++
			return
		}
		f := &logDiagnosticFinding{Level: level, Title: title, Detail: detail, Count: 1}
		findingCounts[key] = f
		findingOrder = append(findingOrder, key)
	}

	shLoc := time.Local
	if loc, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		shLoc = loc
	}

	nextID := 1
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "-- ") {
			continue
		}

		body := line
		shortTime := ""
		fullTime := ""

		// 1. Strip systemd journalctl short-iso header and convert to Asia/Shanghai.
		if m := journalIsoRe.FindStringSubmatch(body); m != nil {
			if ts, err := time.Parse("2006-01-02T15:04:05-0700", m[1]); err == nil {
				lt := ts.In(shLoc)
				shortTime = lt.Format("15:04:05")
				fullTime = lt.Format("2006-01-02 15:04:05")
			}
			body = strings.TrimSpace(m[2])
		}

		// 2. Strip duplicate inner Go/Xray timestamp.
		if m := innerGoTimeRe.FindStringSubmatch(body); m != nil {
			if fullTime == "" {
				fullTime = strings.ReplaceAll(m[1], "/", "-")
				if len(fullTime) >= 19 {
					shortTime = fullTime[11:19]
				}
			}
			body = strings.TrimSpace(m[2])
		}

		level := ""
		module := ""

		// 3. Parse Xray [Warning] / [Error] / [Info] tag.
		if m := xrayLevelRe.FindStringSubmatch(body); m != nil {
			switch strings.ToLower(m[1]) {
			case "error":
				level = "error"
			case "warning", "warn":
				level = "warn"
			default:
				level = "info"
			}
			body = strings.TrimSpace(m[2])
		}

		// 4. Parse Agent [agent] / [xray] / [terminal] tag.
		if m := agentTagRe.FindStringSubmatch(body); m != nil {
			tag := strings.ToLower(m[1])
			switch tag {
			case "agent":
				module = "Agent"
			case "xray":
				module = "Xray"
			case "terminal":
				module = "WebShell"
			case "probe":
				module = "探针"
			default:
				module = m[1]
			}
			body = strings.TrimSpace(m[2])
		}

		if body == "" {
			continue
		}

		low := strings.ToLower(body)
		if module == "" {
			module = classifyLogModule(low, source)
		}
		if level == "" {
			level = classifyLogLevel(low)
		}

		hint, fKey, fLevel, fTitle, fDetail := diagnoseLogLine(low)
		if fKey != "" {
			addFinding(fKey, fLevel, fTitle, fDetail)
			if fLevel == "error" && level == "info" {
				level = "error"
			} else if fLevel == "warn" && level == "info" {
				level = "warn"
			}
		}

		summary.Total++
		switch level {
		case "error":
			summary.Errors++
		case "warn":
			summary.Warnings++
		default:
			summary.Infos++
		}

		// Consecutive deduplication (keeps latest timestamp & increments count).
		if n := len(entries); n > 0 && entries[n-1].Message == body && entries[n-1].Level == level && entries[n-1].Module == module {
			entries[n-1].Count++
			if shortTime != "" {
				entries[n-1].Time = shortTime
				entries[n-1].FullTime = fullTime
			}
			continue
		}

		entries = append(entries, parsedLogEntry{
			ID:       nextID,
			Time:     shortTime,
			FullTime: fullTime,
			Level:    level,
			Module:   module,
			Message:  body,
			Hint:     hint,
			Count:    1,
			Raw:      line,
		})
		nextID++
	}

	for _, k := range findingOrder {
		summary.Findings = append(summary.Findings, *findingCounts[k])
	}
	return entries, summary
}

func classifyLogModule(low, source string) string {
	switch {
	case strings.Contains(low, "reality"):
		return "Reality"
	case strings.Contains(low, "hysteria") || strings.Contains(low, "quic"):
		return "Hysteria2"
	case strings.Contains(low, "vless"):
		return "VLESS"
	case strings.Contains(low, "vmess"):
		return "VMess"
	case strings.Contains(low, "trojan"):
		return "Trojan"
	case strings.Contains(low, "shadowsocks"):
		return "SS-2022"
	case strings.Contains(low, "tls") || strings.Contains(low, "x509") || strings.Contains(low, "cert"):
		return "TLS 证书"
	case strings.Contains(low, "router") || strings.Contains(low, "freedom") || strings.Contains(low, "blackhole") || strings.Contains(low, "outbound"):
		return "路由出站"
	case strings.Contains(low, "systemd") || strings.Contains(low, "started ") || strings.Contains(low, "stopped "):
		return "系统服务"
	case source == "agent":
		return "Agent"
	default:
		return "Xray"
	}
}

func classifyLogLevel(low string) string {
	switch {
	case strings.Contains(low, "panic") || strings.Contains(low, "fatal") ||
		strings.Contains(low, "address already in use") || strings.Contains(low, "failed to start") ||
		strings.Contains(low, "permission denied") || strings.Contains(low, "失败") ||
		strings.Contains(low, "invalid") || strings.Contains(low, "error"):
		return "error"
	case strings.Contains(low, "warn") || strings.Contains(low, "timeout") ||
		strings.Contains(low, "refused") || strings.Contains(low, "rejected") ||
		strings.Contains(low, "reset by peer") || strings.Contains(low, "eof") ||
		strings.Contains(low, "断开"):
		return "warn"
	default:
		return "info"
	}
}

func diagnoseLogLine(low string) (hint, key, level, title, detail string) {
	switch {
	case strings.Contains(low, "address already in use"):
		return "端口已被本机其他进程占用，请在入站编辑页更换端口或点击「预检」",
			"port-in-use", "error", "监听端口冲突", "有入站端口已被服务器其他进程占用，导致 Xray 无法绑定该端口。"
	case strings.Contains(low, "reality") && (strings.Contains(low, "shortid") || strings.Contains(low, "invalid") || strings.Contains(low, "handshake") || strings.Contains(low, "rejected") || strings.Contains(low, "verify")):
		return "检查客户端公钥/ShortID 是否一致、伪装目标是否可达，以及服务器与客户端系统时间误差是否在 30 秒内",
			"reality-handshake", "warn", "Reality 握手异常", "检测到 Reality 握手校验失败，常见于系统时间偏移、伪装目标不可达或客户端参数过旧。"
	case strings.Contains(low, "certificate") || strings.Contains(low, "x509") || (strings.Contains(low, "tls") && strings.Contains(low, "fail")):
		return "检查 TLS 证书文件路径与权限，或切换为无域名自签证书模式",
			"tls-cert", "error", "TLS 证书异常", "检测到 TLS 证书读取或握手错误，请核对入站证书配置。"
	case strings.Contains(low, "too many open files"):
		return "服务器文件描述符上限不足，建议调高 systemd LimitNOFILE",
			"open-files", "error", "文件描述符耗尽", "系统打开文件数达到上限，可能导致新连接被拒绝。"
	case strings.Contains(low, "connection refused") || strings.Contains(low, "i/o timeout"):
		return "上游或出站连接超时/被拒，检查落地节点状态或服务器网络",
			"upstream-timeout", "warn", "上游连接超时或被拒", "转发请求访问上游或落地目标时出现超时/拒绝连接。"
	case strings.Contains(low, "blackhole") || strings.Contains(low, "bittorrent"):
		return "防滥用规则已生效（拦截内网访问或 BT 流量），属正常保护行为",
			"", "", "", ""
	case strings.Contains(low, "broken pipe") || strings.Contains(low, "connection reset by peer") || strings.Contains(low, "unexpected eof"):
		return "客户端网络切换或主动断开连接，偶发属正常现象",
			"", "", "", ""
	case strings.Contains(low, "401") || strings.Contains(low, "令牌无效"):
		return "Agent 令牌认证失败，请在服务器卡片「更多 → 重新生成安装命令」核对令牌",
			"auth-401", "error", "Agent 认证失败", "Agent 与面板通信时返回 401，请检查令牌是否已重置。"
	default:
		return "", "", "", "", ""
	}
}
