package api

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"ppeelink/models"

	"github.com/gin-gonic/gin"
)

// SmartClashDefaultFilename 内置智能分流 Clash 默认模板文件名。
const SmartClashDefaultFilename = "clash-smart.yaml"

// SmartClashGroupSummary 记录智能分流模板各策略组的实时解锁联动状态。
type SmartClashGroupSummary struct {
	Name        string   `json:"name"`
	ServiceKey  string   `json:"serviceKey"`
	Filter      string   `json:"filter"`
	LiveMatched bool     `json:"liveMatched"`
	Fallback    bool     `json:"fallback"`
	Included    []string `json:"included"`
	Excluded    []string `json:"excluded"`
}

type smartGroupSpec struct {
	Name           string
	ServiceKey     string
	FallbackFilter string
}

var smartClashGroupSpecs = []smartGroupSpec{
	{
		Name:           "ChatGPT",
		ServiceKey:     "chatgpt",
		FallbackFilter: "(?i)US|USA|United States|美国|JP|Japan|日本|SG|Singapore|新加坡|UK|GB|英国",
	},
	{
		Name:           "Claude",
		ServiceKey:     "claude",
		FallbackFilter: "(?i)US|USA|United States|美国|JP|Japan|日本|SG|Singapore|新加坡|UK|GB|英国",
	},
	{
		Name:           "Gemini",
		ServiceKey:     "gemini",
		FallbackFilter: "(?i)US|USA|United States|美国|JP|Japan|日本|SG|Singapore|新加坡|TW|台湾",
	},
	{
		Name:           "Netflix",
		ServiceKey:     "netflix",
		FallbackFilter: "(?i)US|USA|JP|HK|SG|TW|UK",
	},
	{
		Name:           "Disney+",
		ServiceKey:     "disney",
		FallbackFilter: "(?i)US|USA|JP|HK|SG|TW|UK",
	},
	{
		Name:           "TikTok",
		ServiceKey:     "tiktok",
		FallbackFilter: "(?i)US|USA|JP|SG|TW|UK",
	},
}

func smartClashRuleProviders() []RuleProvider {
	return []RuleProvider{
		{Name: "115", Type: "http", Behavior: "classical", Interval: 3600, Format: "yaml", Proxy: "日常使用", Path: "./rules/115.yaml", URL: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/115/115.yaml"},
		{Name: "Advertising", Type: "http", Behavior: "classical", Interval: 86400, Format: "yaml", Proxy: "日常使用", Path: "./rules/Advertising.yaml", URL: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Advertising/Advertising.yaml"},
		{Name: "Anthropic", Type: "http", Behavior: "classical", Interval: 3600, Format: "yaml", Proxy: "日常使用", Path: "./ruleset/Anthropic.yaml", URL: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Anthropic/Anthropic.yaml"},
		{Name: "China", Type: "http", Behavior: "classical", Interval: 86400, Format: "yaml", Proxy: "日常使用", Path: "./rules/China.yaml", URL: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/China/China.yaml"},
		{Name: "Claude", Type: "http", Behavior: "classical", Interval: 86400, Format: "yaml", Proxy: "日常使用", Path: "./rules/Claude.yaml", URL: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Claude/Claude.yaml"},
		{Name: "Direct", Type: "http", Behavior: "classical", Interval: 3600, Format: "yaml", Proxy: "日常使用", Path: "./rules/Direct.yaml", URL: "https://kelee.one/Tool/Clash/Rule/Direct.yaml"},
		{Name: "Discord", Type: "http", Behavior: "classical", Interval: 3600, Format: "yaml", Proxy: "日常使用", Path: "./rules/Discord.yaml", URL: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Discord/Discord.yaml"},
		{Name: "Disney", Type: "http", Behavior: "classical", Interval: 86400, Format: "yaml", Proxy: "日常使用", Path: "./rules/Disney.yaml", URL: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Disney/Disney.yaml"},
		{Name: "ESET_China", Type: "http", Behavior: "classical", Interval: 3600, Format: "yaml", Proxy: "日常使用", Path: "./rules/ESET_China.yaml", URL: "https://kelee.one/Tool/Clash/Rule/ESET_China.yaml"},
		{Name: "Gemini", Type: "http", Behavior: "classical", Interval: 3600, Format: "yaml", Proxy: "日常使用", Path: "./rules/Gemini.yaml", URL: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/65e8adff06ea66c81bc61ed64329dae4fbfa7438/rule/Clash/Gemini/Gemini.yaml"},
		{Name: "Google", Type: "http", Behavior: "classical", Interval: 3600, Format: "yaml", Proxy: "日常使用", Path: "./rules/Google.yaml", URL: "https://rule.kelee.one/Clash/Google.yaml"},
		{Name: "LAN", Type: "http", Behavior: "classical", Interval: 3600, Format: "yaml", Proxy: "日常使用", Path: "./rules/Lan.yaml", URL: "https://kelee.one/Tool/Clash/Rule/LAN_SPLITTER.yaml"},
		{Name: "Netflix", Type: "http", Behavior: "classical", Interval: 86400, Format: "yaml", Proxy: "日常使用", Path: "./rules/Netflix.yaml", URL: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Netflix/Netflix.yaml"},
		{Name: "OpenAI", Type: "http", Behavior: "classical", Interval: 86400, Format: "yaml", Proxy: "日常使用", Path: "./rules/OpenAI.yaml", URL: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/OpenAI/OpenAI.yaml"},
		{Name: "Proxy", Type: "http", Behavior: "classical", Interval: 3600, Format: "yaml", Proxy: "日常使用", Path: "./rules/Proxy.yaml", URL: "https://kelee.one/Tool/Clash/Rule/Proxy.yaml"},
		{Name: "Reddit", Type: "http", Behavior: "classical", Interval: 3600, Format: "yaml", Proxy: "日常使用", Path: "./rules/Reddit.yaml", URL: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Reddit/Reddit.yaml"},
		{Name: "TikTok", Type: "http", Behavior: "classical", Interval: 86400, Format: "yaml", Proxy: "日常使用", Path: "./rules/TikTok.yaml", URL: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/TikTok/TikTok.yaml"},
		{Name: "Twitter", Type: "http", Behavior: "classical", Interval: 3600, Format: "yaml", Proxy: "日常使用", Path: "./rules/Twitter.yaml", URL: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Twitter/Twitter.yaml"},
		{Name: "Whatsapp", Type: "http", Behavior: "classical", Interval: 3600, Format: "yaml", Proxy: "日常使用", Path: "./rules/Whatsapp.yaml", URL: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Whatsapp/Whatsapp.yaml"},
		{Name: "Xbox", Type: "http", Behavior: "classical", Interval: 3600, Format: "yaml", Proxy: "日常使用", Path: "./rules/Xbox.yaml", URL: "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Xbox/Xbox.yaml"},
	}
}

func smartClashRules() []string {
	return []string{
		"RULE-SET,Advertising,REJECT",
		"RULE-SET,LAN,DIRECT",
		"RULE-SET,Direct,DIRECT",
		"RULE-SET,ESET_China,DIRECT",
		"RULE-SET,115,DIRECT",
		"RULE-SET,OpenAI,ChatGPT",
		"RULE-SET,Claude,Claude",
		"RULE-SET,Anthropic,Claude",
		"RULE-SET,Gemini,Gemini",
		"RULE-SET,Google,Gemini",
		"RULE-SET,Netflix,Netflix",
		"RULE-SET,Disney,Disney+",
		"RULE-SET,TikTok,TikTok",
		"RULE-SET,Reddit,ChatGPT",
		"RULE-SET,Whatsapp,ChatGPT",
		"RULE-SET,Xbox,日常使用",
		"RULE-SET,Discord,日常使用",
		"RULE-SET,Twitter,日常使用",
		"RULE-SET,Proxy,日常使用",
		"RULE-SET,China,DIRECT",
		"GEOIP,CN,DIRECT",
		"MATCH,日常使用",
	}
}

// smartClashBuilderRequest 构造「智能分流 Clash」模板表单，
// 以现有生产 clash.yaml 的端口/DNS/TUN/Sniffer/规则体系为基底，
// 并实时联动 Server Agent 原生解锁矩阵填充各 AI / 流媒体策略组 filter。
func smartClashBuilderRequest() (BuilderRequest, []SmartClashGroupSummary, int, int) {
	req := defaultBuilderRequest()
	req.Filename = SmartClashDefaultFilename
	req.Source = "default"
	req.RuleProviders = smartClashRuleProviders()
	req.Rules = smartClashRules()

	type checkedSrv struct {
		name string
		rep  *serverOpsUnlockReport
	}
	var checked []checkedSrv
	totalServers := 0
	if models.DB != nil {
		var servers []models.ProxyServer
		if models.DB.Where("kind = ? AND removing = ?", models.ServerKindAgent, false).Order("id asc").Find(&servers).Error == nil {
			totalServers = len(servers)
			for _, s := range servers {
				if s.UnlockJSON == "" {
					continue
				}
				var rep serverOpsUnlockReport
				if json.Unmarshal([]byte(s.UnlockJSON), &rep) == nil && len(rep.Items) > 0 {
					normalizeUnlockReportRegions(&rep)
					checked = append(checked, checkedSrv{name: s.Name, rep: &rep})
				}
			}
		}
	}

	groups := make([]BuilderGroup, 0, len(smartClashGroupSpecs)+1)
	groups = append(groups, BuilderGroup{
		Name:                "日常使用",
		Type:                "select",
		IncludeAllProviders: true,
		Proxies:             []string{},
	})

	summaries := make([]SmartClashGroupSummary, 0, len(smartClashGroupSpecs))
	for _, spec := range smartClashGroupSpecs {
		sum := SmartClashGroupSummary{
			Name:       spec.Name,
			ServiceKey: spec.ServiceKey,
			Included:   []string{},
			Excluded:   []string{},
		}
		for _, ps := range checked {
			ok, known := serverUnlocksService(ps.rep, spec.ServiceKey)
			if !known {
				continue
			}
			if ok {
				sum.Included = append(sum.Included, ps.name)
			} else {
				sum.Excluded = append(sum.Excluded, ps.name)
			}
		}

		bg := BuilderGroup{
			Name:                spec.Name,
			Type:                "select",
			IncludeAllProviders: true,
			Proxies:             []string{},
		}
		if liveFilter, applied := resolveSmartUnlockProviderFilter(spec.Name, spec.FallbackFilter); applied && liveFilter != "" {
			bg.Filter = liveFilter
			sum.Filter = liveFilter
			sum.LiveMatched = true
		} else if len(sum.Included) == 0 && len(sum.Excluded) > 0 {
			// 已检测的服务器均不支持该平台时，回退至「日常使用」，防止 Clash 因空策略组报错。
			bg.Proxies = []string{"日常使用"}
			sum.Fallback = true
		} else {
			bg.Filter = spec.FallbackFilter
			sum.Filter = spec.FallbackFilter
			sum.Fallback = true
		}

		groups = append(groups, bg)
		summaries = append(summaries, sum)
	}

	req.Groups = groups
	return req, summaries, len(checked), totalServers
}

// BuildSmartClashTemplateYAML 根据当前原生解锁矩阵实时生成「智能分流 Clash」YAML 文本。
func BuildSmartClashTemplateYAML() (string, []SmartClashGroupSummary, int, int, error) {
	req, summaries, checkedServers, totalServers := smartClashBuilderRequest()
	out, err := buildClashYAML(req)
	if err != nil {
		return "", nil, checkedServers, totalServers, err
	}
	return out, summaries, checkedServers, totalServers, nil
}

// SyncSmartClashTemplateFile 将实时联动的「智能分流 Clash」模板写入模板目录。
// createIfMissing=false 时仅在 clash-smart.yaml 已存在时更新，尊重用户手动删除行为。
func SyncSmartClashTemplateFile(createIfMissing bool) error {
	fullPath, err := safeFilePath(SmartClashDefaultFilename)
	if err != nil {
		return err
	}
	if !createIfMissing {
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			return nil
		}
	}
	text, _, _, _, err := BuildSmartClashTemplateYAML()
	if err != nil {
		return err
	}
	if existing, err := os.ReadFile(fullPath); err == nil && string(existing) == text {
		return nil
	}
	if err := atomicWrite(fullPath, text); err != nil {
		return err
	}
	if models.DB != nil {
		_ = models.SaveTemplateVersion(SmartClashDefaultFilename, text, "smart_sync")
	}
	return nil
}

// GenerateSmartClashTemplate 显式生成/同步「智能分流 Clash」模板，并可选一键绑定至全部订阅。
// POST /api/v1/template/smart-clash/generate
func GenerateSmartClashTemplate(c *gin.Context) {
	var req struct {
		Filename     string `json:"filename"`
		ApplyAllSubs bool   `json:"applyAllSubs"`
	}
	_ = c.ShouldBindJSON(&req)

	filename := strings.TrimSpace(req.Filename)
	if filename == "" {
		filename = SmartClashDefaultFilename
	}
	if !strings.HasSuffix(strings.ToLower(filename), ".yaml") && !strings.HasSuffix(strings.ToLower(filename), ".yml") {
		filename += ".yaml"
	}

	fullPath, err := safeFilePath(filename)
	if err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": "文件名非法: " + err.Error()})
		return
	}

	text, summaries, checkedServers, totalServers, err := BuildSmartClashTemplateYAML()
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "生成智能分流模板失败: " + err.Error()})
		return
	}
	if err := atomicWrite(fullPath, text); err != nil {
		log.Printf("写入智能分流模板失败: %v", err)
		c.JSON(500, gin.H{"code": "50000", "msg": "保存模板失败"})
		return
	}
	if models.DB != nil {
		_ = models.SaveTemplateVersion(filename, text, "smart_generate")
	}

	templateRelPath := "./template/" + filename
	updatedSubs := 0
	if req.ApplyAllSubs && models.DB != nil {
		var subs []models.Subcription
		if models.DB.Find(&subs).Error == nil {
			for _, sub := range subs {
				cfgMap := map[string]any{}
				if strings.TrimSpace(sub.Config) != "" {
					_ = json.Unmarshal([]byte(sub.Config), &cfgMap)
				}
				cfgMap["clash"] = templateRelPath
				raw, merr := json.Marshal(cfgMap)
				if merr != nil {
					continue
				}
				if models.DB.Model(&models.Subcription{}).Where("id = ?", sub.ID).Update("config", string(raw)).Error == nil {
					updatedSubs++
				}
			}
		}
	}

	msg := fmt.Sprintf("已同步智能分流模板 %s（联动 %d/%d 台服务器解锁数据）", filename, checkedServers, totalServers)
	if req.ApplyAllSubs {
		msg = fmt.Sprintf("已同步 %s 并应用至 %d 个订阅", filename, updatedSubs)
	}
	c.JSON(200, gin.H{
		"code": "00000",
		"msg":  msg,
		"data": gin.H{
			"file":           filename,
			"path":           templateRelPath,
			"text":           text,
			"groups":         summaries,
			"updatedSubs":    updatedSubs,
			"checkedServers": checkedServers,
			"totalServers":   totalServers,
		},
	})
}
