package api

import (
	"fmt"
	"os"
	"strings"

	"ppeelink/models"
)

// SmartLoonDefaultFilename 与 SmartSurgeDefaultFilename 为 iOS 智能分流默认模板文件名。
const (
	SmartLoonDefaultFilename  = "loon-smart.conf"
	SmartSurgeDefaultFilename = "surge-smart.conf"
)

func resolveSmartServiceFilter(groupName, fallback string) (string, bool) {
	if liveFilter, applied := resolveSmartUnlockProviderFilter(groupName, fallback); applied && liveFilter != "" {
		return liveFilter, true
	}
	return fallback, false
}

// BuildSmartLoonTemplate 根据双源解锁矩阵实时生成 Loon 智能分流配置（loon-smart.conf）。
func BuildSmartLoonTemplate() (string, []SmartClashGroupSummary, int, int) {
	_, summaries, checkedServers, totalServers := smartClashBuilderRequest()
	filterMap := map[string]string{}
	for _, spec := range smartClashGroupSpecs {
		f, _ := resolveSmartServiceFilter(spec.Name, spec.FallbackFilter)
		filterMap[spec.Name] = strings.ReplaceAll(f, `"`, `'`)
	}

	var b strings.Builder
	b.WriteString(`[General]
mitm-on-wifi-access = false
ip-mode = v4-only
dns-server = system
sni-sniffing = true
disable-stun = true
dns-reject-mode = LoopbackIP
domain-reject-mode = DNS
udp-fallback-mode = REJECT
wifi-access-http-port = 7222
wifi-access-socks5-port = 7221
allow-wifi-access = false
interface-mode = auto
test-timeout = 5
disconnect-on-policy-change = false
switch-node-after-failure-times = 3
internet-test-url = http://cp.cloudflare.com/generate_204
proxy-test-url = http://cp.cloudflare.com/generate_204
resource-parser = https://github.com/sub-store-org/Sub-Store/releases/latest/download/sub-store-parser.loon.min.js
geoip-url = https://github.com/Masaiki/GeoIP2-CN/raw/release/Country.mmdb
ipasn-url = https://raw.githubusercontent.com/P3TERX/GeoLite.mmdb/download/GeoLite2-ASN.mmdb
skip-proxy = 192.168.0.0/16,10.0.0.0/8,172.16.0.0/12,100.64.0.0/10,localhost,*.local,e.crashlytics.com,captive.apple.com
bypass-tun = 10.0.0.0/8,100.64.0.0/10,127.0.0.0/8,169.254.0.0/16,172.16.0.0/12,192.0.0.0/24,192.0.2.0/24,192.88.99.0/24,192.168.0.0/16,198.51.100.0/24,203.0.113.0/24,224.0.0.0/4,255.255.255.255/32

[Proxy]

[Remote Proxy]

[Remote Filter]
# 地区筛选
美国 = NameRegex, FilterKey = "(?i)(美国|波特兰|达拉斯|俄勒冈|凤凰城|费利蒙|硅谷|拉斯维加斯|洛杉矶|圣何塞|圣克拉拉|西雅图|芝加哥|🇺🇸|US|USA)"
香港 = NameRegex, FilterKey = "(?i)(香港|hong|HK|HKG|🇭🇰)"
日本 = NameRegex, FilterKey = "(?i)(日本|东京|大阪|泉日|埼玉|🇯🇵|JP|Japan)"
新加坡 = NameRegex, FilterKey = "(?i)(狮城|新加坡|🇸🇬|SG|Singapore)"
台湾 = NameRegex, FilterKey = "(?i)(台湾|台灣|🇹🇼|TW|Taiwan)"
# 解锁矩阵实时联动筛选（由 PPEELink 自动同步）
`)
	b.WriteString(fmt.Sprintf("ChatGPT解锁 = NameRegex, FilterKey = \"%s\"\n", filterMap["ChatGPT"]))
	b.WriteString(fmt.Sprintf("Claude解锁 = NameRegex, FilterKey = \"%s\"\n", filterMap["Claude"]))
	b.WriteString(fmt.Sprintf("Gemini解锁 = NameRegex, FilterKey = \"%s\"\n", filterMap["Gemini"]))
	b.WriteString(fmt.Sprintf("Netflix解锁 = NameRegex, FilterKey = \"%s\"\n", filterMap["Netflix"]))
	b.WriteString(fmt.Sprintf("Disney解锁 = NameRegex, FilterKey = \"%s\"\n", filterMap["Disney+"]))
	b.WriteString(fmt.Sprintf("TikTok解锁 = NameRegex, FilterKey = \"%s\"\n", filterMap["TikTok"]))

	b.WriteString(`
[Proxy Group]
Global = select, US, HK, JP, SG, TW, DIRECT, url = http://cp.cloudflare.com/generate_204, img-url = https://raw.githubusercontent.com/erdongchanyo/icon/main/Policy-Filter/Outside.png
ChatGPT = select, ChatGPT解锁, US, JP, SG, Global, url = http://cp.cloudflare.com/generate_204, img-url = https://raw.githubusercontent.com/erdongchanyo/icon/main/Policy-Filter/ChatGPT.png
Claude = select, Claude解锁, US, JP, SG, Global, url = http://cp.cloudflare.com/generate_204, img-url = https://raw.githubusercontent.com/erdongchanyo/icon/main/Policy-Filter/Claude.png
Gemini = select, Gemini解锁, US, JP, SG, TW, Global, url = http://cp.cloudflare.com/generate_204, img-url = https://raw.githubusercontent.com/erdongchanyo/icon/main/Policy-Filter/Google.png
Netflix = select, Netflix解锁, Global, TW, US, HK, JP, SG, DIRECT, url = http://cp.cloudflare.com/generate_204, img-url = https://raw.githubusercontent.com/erdongchanyo/icon/main/Policy-Filter/Netflix.png
Disney+ = select, Disney解锁, Global, TW, US, HK, JP, SG, DIRECT, url = http://cp.cloudflare.com/generate_204, img-url = https://raw.githubusercontent.com/erdongchanyo/icon/main/Policy-Filter/Disney.png
TikTok = select, TikTok解锁, Global, TW, US, JP, SG, DIRECT, url = http://cp.cloudflare.com/generate_204, img-url = https://raw.githubusercontent.com/erdongchanyo/icon/main/Policy-Filter/TikTok.png
YouTube = select, Global, TW, HK, US, JP, SG, DIRECT, url = http://cp.cloudflare.com/generate_204, img-url = https://raw.githubusercontent.com/erdongchanyo/icon/main/Policy-Filter/Youtube.png
US = select, 美国, url = http://cp.cloudflare.com/generate_204, img-url = https://raw.githubusercontent.com/erdongchanyo/icon/main/Policy-Country/US.png
HK = select, 香港, url = http://cp.cloudflare.com/generate_204, img-url = https://raw.githubusercontent.com/erdongchanyo/icon/main/Policy-Country/HK02.png
JP = select, 日本, url = http://cp.cloudflare.com/generate_204, img-url = https://raw.githubusercontent.com/erdongchanyo/icon/main/Policy-Country/JP.png
SG = select, 新加坡, url = http://cp.cloudflare.com/generate_204, img-url = https://raw.githubusercontent.com/erdongchanyo/icon/main/Policy-Country/SG.png
TW = select, 台湾, url = http://cp.cloudflare.com/generate_204, img-url = https://raw.githubusercontent.com/erdongchanyo/icon/main/Policy-Country/TW.png
Mainland = select, DIRECT, url = http://cp.cloudflare.com/generate_204, img-url = https://raw.githubusercontent.com/erdongchanyo/icon/main/Policy-Filter/Mainland.png
Advertising = select, REJECT, url = http://cp.cloudflare.com/generate_204, img-url = https://raw.githubusercontent.com/erdongchanyo/icon/main/Policy-Filter/AdBlock.png
FINAL = select, Global, DIRECT, url = http://cp.cloudflare.com/generate_204, img-url = https://raw.githubusercontent.com/erdongchanyo/icon/main/Policy-Filter/Final01.png

[Remote Rule]
# -------------------- 🤖 AI 与流媒体智能分流 --------------------
https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Loon/OpenAI/OpenAI.list, policy=ChatGPT, tag=OpenAI, enabled=true
https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Loon/Claude/Claude.list, policy=Claude, tag=Claude, enabled=true
https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Loon/Anthropic/Anthropic.list, policy=Claude, tag=Anthropic, enabled=true
https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Loon/Gemini/Gemini.list, policy=Gemini, tag=Gemini, enabled=true
https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Loon/Google/Google.list, policy=Gemini, tag=Google, enabled=true
https://rule.kelee.one/Loon/Netflix.lsr, policy=Netflix, tag=Netflix, enabled=true
https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Loon/Disney/Disney.list, policy=Disney+, tag=Disney+, enabled=true
https://rule.kelee.one/Loon/TikTok.lsr, policy=TikTok, tag=TikTok, enabled=true
https://rule.kelee.one/Loon/YouTube.lsr, policy=YouTube, tag=YouTube, enabled=true
https://rule.kelee.one/Loon/Spotify.lsr, policy=YouTube, tag=Spotify, enabled=true
https://rule.kelee.one/Loon/Telegram.lsr, policy=Global, tag=Telegram, enabled=true
https://rule.kelee.one/Loon/Pixiv.lsr, policy=Global, tag=Pixiv, enabled=true
https://kelee.one/Tool/Loon/Lsr/SpeedtestInternational.lsr, policy=Global, tag=Speedtest-INT, enabled=true
https://raw.githubusercontent.com/Xioaruan912/EverythingRules/refs/heads/main/rules/loon/98Tang.list, policy=FINAL, tag=98Tang, enabled=true
https://raw.githubusercontent.com/Xioaruan912/EverythingRules/refs/heads/main/rules/loon/SELF_VPN.list, policy=Global, tag=self_vpn, enabled=true

# -------------------- 🇨🇳 国内直连规则 --------------------
https://rule.kelee.one/Loon/iCloud.lsr, policy=DIRECT, tag=iCloud, enabled=true
https://rule.kelee.one/Loon/Apple.lsr, policy=DIRECT, tag=Apple, enabled=true
https://rule.kelee.one/Loon/AppStore.lsr, policy=DIRECT, tag=AppStore, enabled=true
https://rule.kelee.one/Loon/AppleID.lsr, policy=DIRECT, tag=AppleID, enabled=true
https://rule.kelee.one/Loon/WeChat.lsr, policy=DIRECT, tag=WeChat, enabled=true
https://rule.kelee.one/Loon/Weibo.lsr, policy=DIRECT, tag=Weibo, enabled=true
https://rule.kelee.one/Loon/DingTalk.lsr, policy=DIRECT, tag=DingTalk, enabled=true
https://rule.kelee.one/Loon/DiDi.lsr, policy=DIRECT, tag=DiDi, enabled=true
https://rule.kelee.one/Loon/DouYin.lsr, policy=DIRECT, tag=DouYin, enabled=true
https://rule.kelee.one/Loon/Baidu.lsr, policy=DIRECT, tag=Baidu, enabled=true
https://rule.kelee.one/Loon/AliPay.lsr, policy=DIRECT, tag=AliPay, enabled=true
https://rule.kelee.one/Loon/115.lsr, policy=DIRECT, tag=115, enabled=true
https://rule.kelee.one/Loon/12306.lsr, policy=DIRECT, tag=12306, enabled=true
https://kelee.one/Tool/Loon/Lsr/SpeedtestChina.lsr, policy=DIRECT, tag=Speedtest-China, enabled=true
https://rule.kelee.one/Loon/ChinaMedia.lsr, policy=DIRECT, tag=ChinaMedia, enabled=true
https://rule.kelee.one/Loon/ChinaDNS.lsr, policy=DIRECT, tag=ChinaDNS, enabled=true
https://rule.kelee.one/Loon/ChinaMax.lsr, policy=Mainland, tag=ChinaMax, enabled=true
https://rule.kelee.one/Loon/ChinaIPs.lsr, policy=Mainland, tag=ChinaIPs, enabled=true
https://rule.kelee.one/Loon/BiliBiliIntl.lsr, policy=YouTube, tag=BiliBiliIntl, enabled=true
https://rule.kelee.one/Loon/BiliBili.lsr, policy=DIRECT, tag=BiliBili, enabled=true

# -------------------- 🚫 广告拦截与自定义 --------------------
https://raw.githubusercontent.com/Cats-Team/AdRules/main/adrules.list, policy=Advertising, tag=AdRules, enabled=true
https://raw.githubusercontent.com/Xioaruan912/EverythingRules/refs/heads/main/rules/loon/xiaoyi.list, policy=Advertising, tag=xiaoyiAd, enabled=true
https://raw.githubusercontent.com/Xioaruan912/EverythingRules/refs/heads/main/rules/loon/reject.list, policy=Advertising, tag=reject, enabled=true
https://raw.githubusercontent.com/Xioaruan912/EverythingRules/refs/heads/main/rules/loon/self.list, policy=DIRECT, tag=self, enabled=true

[Proxy Chain]

[Rule]
DOMAIN-SUFFIX,local,DIRECT
GEOIP,CN,DIRECT
FINAL,FINAL

[Host]

[Rewrite]

[Script]

[Plugin]
https://kelee.one/Tool/Loon/Lpx/Prevent_DNS_Leaks.lpx, enabled=true
https://kelee.one/Tool/Loon/Lpx/Block_HTTPDNS.lpx, pin=true, enabled=true
https://kelee.one/Tool/Loon/Lpx/BlockAdvertisers.lpx, pin=true, enabled=true

[Mitm]
hostname =
skip-server-cert-verify = false
`)
	return b.String(), summaries, checkedServers, totalServers
}

// BuildSmartSurgeTemplate 根据双源解锁矩阵实时生成 Surge 智能分流配置（surge-smart.conf）。
func BuildSmartSurgeTemplate() (string, []SmartClashGroupSummary, int, int) {
	_, summaries, checkedServers, totalServers := smartClashBuilderRequest()
	filterMap := map[string]string{}
	for _, spec := range smartClashGroupSpecs {
		f, _ := resolveSmartServiceFilter(spec.Name, spec.FallbackFilter)
		filterMap[spec.Name] = strings.ReplaceAll(f, `"`, `'`)
	}

	var b strings.Builder
	b.WriteString(`[General]
loglevel = notify
bypass-system = true
skip-proxy = 127.0.0.1,192.168.0.0/16,10.0.0.0/8,172.16.0.0/12,100.64.0.0/10,localhost,*.local,e.crashlytics.com,captive.apple.com,::ffff:0:0:0:0/1,::ffff:128:0:0:0/1
bypass-tun = 192.168.0.0/16,10.0.0.0/8,172.16.0.0/12,100.64.0.0/10
dns-server = 119.29.29.29,223.5.5.5,system
internet-test-url = http://cp.cloudflare.com/generate_204
proxy-test-url = http://cp.cloudflare.com/generate_204
test-timeout = 5
exclude-simple-hostnames = true
allow-wifi-access = false

[Proxy]
DIRECT = direct

[Proxy Group]
日常使用 = select
`)
	b.WriteString(fmt.Sprintf("ChatGPT = select, 日常使用, filter(\"%s\")\n", filterMap["ChatGPT"]))
	b.WriteString(fmt.Sprintf("Claude = select, 日常使用, filter(\"%s\")\n", filterMap["Claude"]))
	b.WriteString(fmt.Sprintf("Gemini = select, 日常使用, filter(\"%s\")\n", filterMap["Gemini"]))
	b.WriteString(fmt.Sprintf("Netflix = select, 日常使用, filter(\"%s\")\n", filterMap["Netflix"]))
	b.WriteString(fmt.Sprintf("Disney+ = select, 日常使用, filter(\"%s\")\n", filterMap["Disney+"]))
	b.WriteString(fmt.Sprintf("TikTok = select, 日常使用, filter(\"%s\")\n", filterMap["TikTok"]))
	b.WriteString(`YouTube = select, 日常使用, DIRECT
广告拦截 = select, REJECT, DIRECT
全球直连 = select, DIRECT, 日常使用
漏网之鱼 = select, 日常使用, DIRECT

[Rule]
RULE-SET,https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Surge/Advertising/Advertising.list,广告拦截,update-interval=86400
RULE-SET,https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Surge/Lan/Lan.list,全球直连,update-interval=86400
RULE-SET,https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Surge/OpenAI/OpenAI.list,ChatGPT,update-interval=86400
RULE-SET,https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Surge/Claude/Claude.list,Claude,update-interval=86400
RULE-SET,https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Surge/Anthropic/Anthropic.list,Claude,update-interval=86400
RULE-SET,https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Surge/Gemini/Gemini.list,Gemini,update-interval=86400
RULE-SET,https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Surge/Google/Google.list,Gemini,update-interval=86400
RULE-SET,https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Surge/Netflix/Netflix.list,Netflix,update-interval=86400
RULE-SET,https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Surge/Disney/Disney.list,Disney+,update-interval=86400
RULE-SET,https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Surge/TikTok/TikTok.list,TikTok,update-interval=86400
RULE-SET,https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Surge/YouTube/YouTube.list,YouTube,update-interval=86400
RULE-SET,https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Surge/Telegram/Telegram.list,日常使用,update-interval=86400
RULE-SET,https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Surge/China/China.list,全球直连,update-interval=86400
GEOIP,CN,全球直连
FINAL,漏网之鱼
`)
	return b.String(), summaries, checkedServers, totalServers
}

func syncSmartFile(filename, text string, createIfMissing bool) error {
	fullPath, err := safeFilePath(filename)
	if err != nil {
		return err
	}
	if !createIfMissing {
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			return nil
		}
	}
	if existing, err := os.ReadFile(fullPath); err == nil && string(existing) == text {
		return nil
	}
	if err := atomicWrite(fullPath, text); err != nil {
		return err
	}
	if models.DB != nil {
		_ = models.SaveTemplateVersion(filename, text, "smart_sync")
	}
	return nil
}

// SyncSmartLoonTemplateFile 同步内置智能分流 Loon 模板（loon-smart.conf）。
func SyncSmartLoonTemplateFile(createIfMissing bool) error {
	text, _, _, _ := BuildSmartLoonTemplate()
	return syncSmartFile(SmartLoonDefaultFilename, text, createIfMissing)
}

// SyncSmartSurgeTemplateFile 同步内置智能分流 Surge 模板（surge-smart.conf）。
func SyncSmartSurgeTemplateFile(createIfMissing bool) error {
	text, _, _, _ := BuildSmartSurgeTemplate()
	return syncSmartFile(SmartSurgeDefaultFilename, text, createIfMissing)
}

// SyncAllSmartTemplates 一次性同步 Clash + Loon + Surge 三端智能分流模板。
func SyncAllSmartTemplates(createIfMissing bool) error {
	var firstErr error
	if err := SyncSmartClashTemplateFile(createIfMissing); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := SyncSmartLoonTemplateFile(createIfMissing); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := SyncSmartSurgeTemplateFile(createIfMissing); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}
