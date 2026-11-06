package node

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

const unlockUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36"
const unlockBodyLimit = 2 * 1024 * 1024

var unlockCountry = regexp.MustCompile(`^[A-Z]{2}$`)
var netflixRegion = regexp.MustCompile(`^/([a-zA-Z]{2})(?:-[a-zA-Z]{2})?(?:/|$)`)
var daznAllowed = regexp.MustCompile(`"isAllowed"\s*:\s*(true|false)`)
var daznCountry = regexp.MustCompile(`(?i)"GeolocatedCountry"\s*:\s*"([a-z]{2})"`)

func unlockEvidence(kind string) string {
	switch kind {
	case "openai", "dazn", "tvb":
		return "api"
	case "trace":
		return "region"
	case "gemini", "netflix", "disney":
		return "page"
	default:
		return "reach"
	}
}

func probeUnlock(ctx context.Context, c *http.Client, s UnlockService) UnlockCheckResult {
	if s.Check != nil {
		return s.Check(ctx, c)
	}
	result := UnlockCheckResult{Status: "unknown", Evidence: unlockEvidence(s.Kind)}
	method := s.Method
	if method == "" {
		method = http.MethodGet
	}
	req, err := http.NewRequestWithContext(ctx, method, s.URL, strings.NewReader(s.Body))
	if err != nil {
		result.Note = "检测地址无效"
		return result
	}
	req.Header.Set("User-Agent", unlockUA)
	if s.Body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			result.Note = "请求已取消或超时"
		} else {
			result.Note = "连接失败"
		}
		return result
	}
	defer resp.Body.Close()
	if s.Kind == "reach" {
		if resp.StatusCode >= 200 && resp.StatusCode < 400 {
			result.Status = "reachable"
			result.Note = "站点可访问，未验证登录或服务功能"
		} else {
			result.Note = fmt.Sprintf("HTTP %d，无法判断", resp.StatusCode)
		}
		return result
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, unlockBodyLimit+1))
	if err != nil || len(data) > unlockBodyLimit {
		result.Note = "响应读取失败或超过大小限制"
		return result
	}
	result = interpretUnlock(s, resp.StatusCode, string(data))
	if s.Kind == "netflix" && result.Status == "reachable" && resp.Request != nil && resp.Request.URL != nil {
		if m := netflixRegion.FindStringSubmatch(resp.Request.URL.Path); len(m) > 1 {
			return inferUnlockRegion(s, strings.ToUpper(m[1]))
		}
	}
	return result
}

func interpretUnlock(s UnlockService, code int, text string) UnlockCheckResult {
	r := UnlockCheckResult{Status: "unknown", Evidence: unlockEvidence(s.Kind), Note: "响应不符合检测特征，无法判断"}
	lower := strings.ToLower(text)
	var obj map[string]json.RawMessage
	validJSON := json.Unmarshal([]byte(text), &obj) == nil && obj != nil
	if s.Kind == "openai" && validJSON && strings.Contains(lower, "unsupported_country") {
		r.Status = "blocked"
		r.Note = "服务接口明确拒绝此地区"
		return r
	}
	if s.Kind == "dazn" && code == 403 && strings.Contains(lower, "security policy has been breached") {
		r.Status = "blocked"
		r.Note = "服务接口拒绝此出口"
		return r
	}
	if code < 200 || code >= 300 {
		r.Note = fmt.Sprintf("HTTP %d，无法判断", code)
		return r
	}
	for _, challenge := range []string{"just a moment", "cf-chl-", "challenge-platform", "verify you are human", "checking your browser"} {
		if strings.Contains(lower, challenge) {
			r.Note = "目标要求浏览器验证，无法判断"
			return r
		}
	}
	switch s.Kind {
	case "trace":
		for _, line := range strings.Split(text, "\n") {
			key, value, ok := strings.Cut(line, "=")
			if ok && strings.TrimSpace(key) == "loc" {
				return inferUnlockRegion(s, strings.ToUpper(strings.TrimSpace(value)))
			}
		}
	case "openai":
		var required bool
		if raw, ok := obj["required"]; validJSON && ok && string(raw) != "null" && json.Unmarshal(raw, &required) == nil && obj["error"] == nil {
			r.Ok = true
			r.Status = "available"
			r.Note = "地区接口通过；账号、登录与功能权限需另行验证"
		}
	case "dazn":
		if m := daznAllowed.FindStringSubmatch(text); validJSON && len(m) > 1 {
			r.Ok = m[1] == "true"
			r.Status = "blocked"
			if r.Ok {
				r.Status = "available"
			}
			r.Note = "地区接口判定，不代表订阅或片库权限"
			if country := daznCountry.FindStringSubmatch(text); len(country) > 1 {
				r.Region = strings.ToUpper(country[1])
			}
		}
	case "tvb":
		var allowed bool
		if raw, ok := obj["allow_in_this_country"]; validJSON && ok && string(raw) != "null" && json.Unmarshal(raw, &allowed) == nil {
			r.Ok = allowed
			r.Status = "blocked"
			if allowed {
				r.Status = "available"
			}
			r.Note = "地区接口判定，不代表账号或播放权限"
			var country string
			if json.Unmarshal(obj["country"], &country) == nil && unlockCountry.MatchString(strings.ToUpper(country)) {
				r.Region = strings.ToUpper(country)
			}
		}
	case "gemini", "netflix", "disney":
		for _, marker := range []string{"not available in your country", "unavailable in your country", "not available in your region", "unavailable in your location", "service is not available in your country"} {
			if strings.Contains(lower, marker) {
				r.Status = "blocked"
				r.Note = "页面明确提示地区限制"
				return r
			}
		}
		if s.Kind == "gemini" && strings.Contains(text, "45631641,null,true") {
			r.Ok = true
			r.Status = "available"
			r.Note = "页面包含可用标记；未验证账号与实际生成功能"
			return r
		}
		r.Status = "reachable"
		r.Evidence = "reach"
		r.Note = "首页可访问，无法确认服务是否解锁"
		if s.Kind == "netflix" {
			r.Note = "首页可访问，未确认地区、完整片库或播放权限"
		}
	}
	return r
}

func inferUnlockRegion(s UnlockService, country string) UnlockCheckResult {
	r := UnlockCheckResult{Status: "unknown", Evidence: "region", Note: "未取得有效地区，无法判断"}
	if !unlockCountry.MatchString(country) || country == "XX" || country == "ZZ" || country == "T1" {
		return r
	}
	r.Region = country
	if s.Allow == "" && s.Deny == "" {
		r.Status = "reachable"
		r.Note = "已取得目标地区，未配置地区可用性规则"
		return r
	}
	allowed := true
	if s.Allow != "" {
		allowed = strings.Contains(" "+s.Allow+" ", " "+country+" ")
	}
	if strings.Contains(" "+s.Deny+" ", " "+country+" ") {
		allowed = false
	}
	r.Status = "inferred"
	if !allowed {
		r.Status = "region_blocked"
	}
	r.Note = "根据该服务返回的地区及地区规则推测；未验证账号、风控或实际功能"
	if s.Kind == "netflix" {
		r.Note = "根据跳转地区推测；不能证明完整片库或视频可播放"
	}
	return r
}
