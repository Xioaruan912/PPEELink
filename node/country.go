package node

import (
	"regexp"
	"strings"
	"unicode"
)

// Country 节点国家/地区：用于国旗、名称识别与国家选择。
type Country struct {
	Code    string   `json:"code"`
	Name    string   `json:"name"`
	Flag    string   `json:"flag"`
	Aliases []string `json:"aliases"`
}

// countries 常用在前：选择框顺序与名称识别的优先级都按这个顺序。
var countries = []Country{
	{Code: "HK", Name: "香港", Aliases: []string{"香港", "Hong Kong", "HongKong", "HKG"}},
	{Code: "TW", Name: "台湾", Aliases: []string{"台湾", "臺灣", "台灣", "Taiwan", "Taipei", "TWN"}},
	{Code: "JP", Name: "日本", Aliases: []string{"日本", "东京", "東京", "大阪", "Japan", "Tokyo", "Osaka", "JPN"}},
	{Code: "SG", Name: "新加坡", Aliases: []string{"新加坡", "狮城", "獅城", "Singapore", "SGP"}},
	{Code: "US", Name: "美国", Aliases: []string{"美国", "美國", "洛杉矶", "圣何塞", "硅谷", "西雅图", "纽约", "芝加哥", "达拉斯", "United States", "America", "Los Angeles", "San Jose", "Seattle", "New York", "Chicago", "Dallas", "USA"}},
	{Code: "KR", Name: "韩国", Aliases: []string{"韩国", "韓國", "首尔", "首爾", "Korea", "Seoul", "KOR"}},
	{Code: "GB", Name: "英国", Aliases: []string{"英国", "英國", "伦敦", "United Kingdom", "Britain", "London", "GBR", "UK"}},
	{Code: "DE", Name: "德国", Aliases: []string{"德国", "德國", "法兰克福", "Germany", "Frankfurt", "DEU"}},
	{Code: "FR", Name: "法国", Aliases: []string{"法国", "法國", "巴黎", "France", "Paris", "FRA"}},
	{Code: "NL", Name: "荷兰", Aliases: []string{"荷兰", "荷蘭", "阿姆斯特丹", "Netherlands", "Amsterdam", "NLD"}},
	{Code: "CA", Name: "加拿大", Aliases: []string{"加拿大", "Canada", "Toronto", "Vancouver", "Montreal"}},
	{Code: "AU", Name: "澳大利亚", Aliases: []string{"澳大利亚", "澳洲", "悉尼", "Australia", "Sydney", "Melbourne", "AUS"}},
	{Code: "RU", Name: "俄罗斯", Aliases: []string{"俄罗斯", "俄羅斯", "莫斯科", "Russia", "Moscow", "RUS"}},
	{Code: "IN", Name: "印度", Aliases: []string{"印度", "孟买", "India", "Mumbai", "IND"}},
	{Code: "TR", Name: "土耳其", Aliases: []string{"土耳其", "Turkey", "Turkiye", "Istanbul", "TUR"}},
	{Code: "MO", Name: "澳门", Aliases: []string{"澳门", "澳門", "Macao", "Macau", "MAC"}},
	{Code: "CN", Name: "中国大陆", Aliases: []string{"中国大陆", "大陆", "大陸", "China", "CHN"}},
	{Code: "MY", Name: "马来西亚", Aliases: []string{"马来西亚", "馬來西亞", "Malaysia", "Kuala Lumpur", "MYS"}},
	{Code: "TH", Name: "泰国", Aliases: []string{"泰国", "泰國", "曼谷", "Thailand", "Bangkok", "THA"}},
	{Code: "VN", Name: "越南", Aliases: []string{"越南", "Vietnam", "VNM"}},
	{Code: "PH", Name: "菲律宾", Aliases: []string{"菲律宾", "菲律賓", "Philippines", "Manila", "PHL"}},
	{Code: "ID", Name: "印尼", Aliases: []string{"印尼", "印度尼西亚", "雅加达", "Indonesia", "Jakarta", "IDN"}},
	{Code: "AE", Name: "阿联酋", Aliases: []string{"阿联酋", "迪拜", "Dubai", "Emirates", "ARE"}},
	{Code: "BR", Name: "巴西", Aliases: []string{"巴西", "Brazil", "Sao Paulo", "BRA"}},
	{Code: "IT", Name: "意大利", Aliases: []string{"意大利", "米兰", "Italy", "Milan", "ITA"}},
	{Code: "ES", Name: "西班牙", Aliases: []string{"西班牙", "Spain", "Madrid", "ESP"}},
	{Code: "CH", Name: "瑞士", Aliases: []string{"瑞士", "Switzerland", "Zurich", "CHE"}},
	{Code: "SE", Name: "瑞典", Aliases: []string{"瑞典", "Sweden", "Stockholm", "SWE"}},
	{Code: "PL", Name: "波兰", Aliases: []string{"波兰", "Poland", "Warsaw", "POL"}},
	{Code: "AR", Name: "阿根廷", Aliases: []string{"阿根廷", "Argentina", "ARG"}},
	{Code: "ZA", Name: "南非", Aliases: []string{"南非", "South Africa", "Johannesburg", "ZAF"}},
	{Code: "IL", Name: "以色列", Aliases: []string{"以色列", "Israel", "ISR"}},
	{Code: "UA", Name: "乌克兰", Aliases: []string{"乌克兰", "Ukraine", "Kyiv", "UKR"}},
	{Code: "MX", Name: "墨西哥", Aliases: []string{"墨西哥", "Mexico", "MEX"}},
	{Code: "NZ", Name: "新西兰", Aliases: []string{"新西兰", "New Zealand", "Auckland", "NZL"}},
	{Code: "FI", Name: "芬兰", Aliases: []string{"芬兰", "Finland", "Helsinki", "FIN"}},
	{Code: "NO", Name: "挪威", Aliases: []string{"挪威"}},
	{Code: "DK", Name: "丹麦", Aliases: []string{"丹麦"}},
	{Code: "AT", Name: "奥地利", Aliases: []string{"奥地利"}},
	{Code: "BE", Name: "比利时", Aliases: []string{"比利时"}},
	{Code: "IE", Name: "爱尔兰", Aliases: []string{"爱尔兰", "Ireland", "Dublin", "IRL"}},
	{Code: "CZ", Name: "捷克", Aliases: []string{"捷克"}},
	{Code: "EG", Name: "埃及", Aliases: []string{"埃及"}},
	{Code: "BG", Name: "保加利亚", Aliases: []string{"保加利亚"}},
	{Code: "GR", Name: "希腊", Aliases: []string{"希腊"}},
	{Code: "PT", Name: "葡萄牙", Aliases: []string{"葡萄牙"}},
	{Code: "SA", Name: "沙特", Aliases: []string{"沙特"}},
	{Code: "KZ", Name: "哈萨克", Aliases: []string{"哈萨克", "哈萨克斯坦", "Kazakhstan", "KAZ"}},
	{Code: "UZ", Name: "乌兹别克", Aliases: []string{"乌兹别克"}},
	{Code: "CO", Name: "哥伦比亚", Aliases: []string{"哥伦比亚"}},
	{Code: "CL", Name: "智利", Aliases: []string{"智利"}},
	{Code: "PE", Name: "秘鲁", Aliases: []string{"秘鲁"}},
	{Code: "RO", Name: "罗马尼亚", Aliases: []string{"罗马尼亚"}},
	{Code: "HU", Name: "匈牙利", Aliases: []string{"匈牙利"}},
	{Code: "LT", Name: "立陶宛", Aliases: []string{"立陶宛"}},
	{Code: "EE", Name: "爱沙尼亚", Aliases: []string{"爱沙尼亚"}},
	{Code: "LV", Name: "拉脱维亚", Aliases: []string{"拉脱维亚"}},
	{Code: "IS", Name: "冰岛", Aliases: []string{"冰岛"}},
	{Code: "LU", Name: "卢森堡", Aliases: []string{"卢森堡"}},
	{Code: "CY", Name: "塞浦路斯", Aliases: []string{"塞浦路斯"}},
	{Code: "MT", Name: "马耳他", Aliases: []string{"马耳他"}},
	{Code: "AF", Name: "阿富汗", Aliases: []string{"阿富汗"}},
	{Code: "AM", Name: "亚美尼亚", Aliases: []string{"亚美尼亚"}},
	{Code: "AZ", Name: "阿塞拜疆", Aliases: []string{"阿塞拜疆"}},
	{Code: "BD", Name: "孟加拉", Aliases: []string{"孟加拉"}},
	{Code: "BH", Name: "巴林", Aliases: []string{"巴林"}},
	{Code: "BO", Name: "玻利维亚", Aliases: []string{"玻利维亚"}},
	{Code: "BY", Name: "白俄罗斯", Aliases: []string{"白俄罗斯"}},
	{Code: "CI", Name: "科特迪瓦", Aliases: []string{"科特迪瓦"}},
	{Code: "CR", Name: "哥斯达黎加", Aliases: []string{"哥斯达黎加"}},
	{Code: "CU", Name: "古巴", Aliases: []string{"古巴"}},
	{Code: "DO", Name: "多米尼加", Aliases: []string{"多米尼加"}},
	{Code: "DZ", Name: "阿尔及利亚", Aliases: []string{"阿尔及利亚"}},
	{Code: "EC", Name: "厄瓜多尔", Aliases: []string{"厄瓜多尔"}},
	{Code: "ET", Name: "埃塞俄比亚", Aliases: []string{"埃塞俄比亚"}},
	{Code: "GE", Name: "格鲁吉亚", Aliases: []string{"格鲁吉亚"}},
	{Code: "GH", Name: "加纳", Aliases: []string{"加纳"}},
	{Code: "GT", Name: "危地马拉", Aliases: []string{"危地马拉"}},
	{Code: "HR", Name: "克罗地亚", Aliases: []string{"克罗地亚"}},
	{Code: "IQ", Name: "伊拉克", Aliases: []string{"伊拉克"}},
	{Code: "IR", Name: "伊朗", Aliases: []string{"伊朗"}},
	{Code: "JM", Name: "牙买加", Aliases: []string{"牙买加"}},
	{Code: "JO", Name: "约旦", Aliases: []string{"约旦"}},
	{Code: "KE", Name: "肯尼亚", Aliases: []string{"肯尼亚"}},
	{Code: "KH", Name: "柬埔寨", Aliases: []string{"柬埔寨"}},
	{Code: "KW", Name: "科威特", Aliases: []string{"科威特"}},
	{Code: "LA", Name: "老挝", Aliases: []string{"老挝"}},
	{Code: "LB", Name: "黎巴嫩", Aliases: []string{"黎巴嫩"}},
	{Code: "LI", Name: "列支敦士登", Aliases: []string{"列支敦士登"}},
	{Code: "LK", Name: "斯里兰卡", Aliases: []string{"斯里兰卡"}},
	{Code: "LY", Name: "利比亚", Aliases: []string{"利比亚"}},
	{Code: "MA", Name: "摩洛哥", Aliases: []string{"摩洛哥"}},
	{Code: "MD", Name: "摩尔多瓦", Aliases: []string{"摩尔多瓦"}},
	{Code: "MM", Name: "缅甸", Aliases: []string{"缅甸"}},
	{Code: "NG", Name: "尼日利亚", Aliases: []string{"尼日利亚"}},
	{Code: "NP", Name: "尼泊尔", Aliases: []string{"尼泊尔"}},
	{Code: "OM", Name: "阿曼", Aliases: []string{"阿曼"}},
	{Code: "PA", Name: "巴拿马", Aliases: []string{"巴拿马"}},
	{Code: "PK", Name: "巴基斯坦", Aliases: []string{"巴基斯坦"}},
	{Code: "PR", Name: "波多黎各", Aliases: []string{"波多黎各"}},
	{Code: "PY", Name: "巴拉圭", Aliases: []string{"巴拉圭"}},
	{Code: "QA", Name: "卡塔尔", Aliases: []string{"卡塔尔"}},
	{Code: "RS", Name: "塞尔维亚", Aliases: []string{"塞尔维亚"}},
	{Code: "SC", Name: "塞舌尔", Aliases: []string{"塞舌尔", "Seychelles", "SYC"}},
	{Code: "SD", Name: "苏丹", Aliases: []string{"苏丹"}},
	{Code: "SI", Name: "斯洛文尼亚", Aliases: []string{"斯洛文尼亚"}},
	{Code: "SK", Name: "斯洛伐克", Aliases: []string{"斯洛伐克"}},
	{Code: "SN", Name: "塞内加尔", Aliases: []string{"塞内加尔"}},
	{Code: "TN", Name: "突尼斯", Aliases: []string{"突尼斯"}},
	{Code: "TZ", Name: "坦桑尼亚", Aliases: []string{"坦桑尼亚"}},
	{Code: "UG", Name: "乌干达", Aliases: []string{"乌干达"}},
	{Code: "UY", Name: "乌拉圭", Aliases: []string{"乌拉圭"}},
	{Code: "VE", Name: "委内瑞拉", Aliases: []string{"委内瑞拉"}},
	{Code: "YE", Name: "也门", Aliases: []string{"也门"}},
}

var countryByCode = func() map[string]Country {
	m := make(map[string]Country, len(countries))
	for i := range countries {
		countries[i].Flag = FlagEmoji(countries[i].Code)
		m[countries[i].Code] = countries[i]
	}
	return m
}()

// Countries 全部可选国家/地区。
func Countries() []Country { return countries }

// CountryName 国家代码对应的中文名，未知时返回代码本身。
func CountryName(code string) string {
	if c, ok := countryByCode[strings.ToUpper(code)]; ok {
		return c.Name
	}
	return code
}

var (
	flagPair    = regexp.MustCompile(`[\x{1F1E6}-\x{1F1FF}]{2}`)
	letterRun   = regexp.MustCompile(`[A-Za-z]+`)
	cn2Route    = regexp.MustCompile(`^CN2`)
	codeAliases = func() map[string]string {
		m := map[string]string{}
		for _, c := range countries {
			for _, a := range c.Aliases {
				if len(a) == 3 && strings.ToUpper(a) == a && isASCIILetters(a) {
					m[a] = c.Code
				}
			}
		}
		m["UK"] = "GB"
		return m
	}()
)

func isASCIILetters(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII || !unicode.IsLetter(r) {
			return false
		}
	}
	return s != ""
}

// CountryFromName 从节点名识别国家，取名称中最靠前的一处线索：
//   - 国旗 emoji，如 🇯🇵；
//   - 中文名或城市，如 香港、东京；
//   - 英文名或城市（不区分大小写），如 Japan、Tokyo；
//   - 全大写的 2/3 位代码作为独立片段，如 JP、USA（Nosla-JP、lighelayer_USA）。
//
// 小写或嵌在单词里的字母（如 Reality 里的 it）不会被当作代码。
func CountryFromName(name string) string {
	if strings.TrimSpace(name) == "" {
		return ""
	}
	// 位置最靠前者优先；同一位置取更长的匹配（印度尼西亚 优先于 印度）
	best, bestAt, bestLen := "", len(name)+1, 0
	consider := func(code string, at, length int) {
		if at >= 0 && (at < bestAt || at == bestAt && length > bestLen) {
			best, bestAt, bestLen = code, at, length
		}
	}
	if loc := flagPair.FindStringIndex(name); loc != nil {
		runes := []rune(name[loc[0]:loc[1]])
		code := string([]byte{byte(runes[0] - 0x1F1E6 + 'A'), byte(runes[1] - 0x1F1E6 + 'A')})
		if _, ok := countryByCode[code]; ok {
			consider(code, loc[0], loc[1]-loc[0])
		}
	}
	lower := strings.ToLower(name)
	for _, c := range countries {
		for _, a := range c.Aliases {
			if isASCIILetters(strings.ReplaceAll(a, " ", "")) && len(a) <= 3 {
				continue // 代码类别名按片段匹配
			}
			consider(c.Code, strings.Index(lower, strings.ToLower(a)), len(a))
		}
	}
	for _, loc := range letterRun.FindAllStringIndex(name, -1) {
		token := name[loc[0]:loc[1]]
		if strings.ToUpper(token) != token {
			continue
		}
		switch len(token) {
		case 2:
			if token == "CN" && cn2Route.MatchString(name[loc[0]:]) {
				continue // CN2 是线路名，不是国家
			}
			if alias, ok := codeAliases[token]; ok {
				consider(alias, loc[0], 2)
			} else if _, ok := countryByCode[token]; ok {
				consider(token, loc[0], 2)
			}
		case 3:
			if code, ok := codeAliases[token]; ok {
				consider(code, loc[0], 3)
			}
		}
	}
	return best
}

// StripFlags 去掉名称里的国旗 emoji 与随后的空白。
func StripFlags(name string) string {
	return strings.TrimSpace(flagPair.ReplaceAllString(name, ""))
}

// WithFlag 把名称改成 “国旗 名称”；code 为空时只去掉旧国旗。
func WithFlag(name, code string) string {
	bare := StripFlags(name)
	if flag := FlagEmoji(code); flag != "" && bare != "" {
		return flag + " " + bare
	}
	return bare
}

// NodeCountry 节点所属国家：优先用名称里的线索（用户命名的意图），识别不到再按服务器 IP 查 GeoIP。
func NodeCountry(name, host string) string {
	if code := CountryFromName(name); code != "" {
		return code
	}
	if host == "" {
		return ""
	}
	return LookupCountry(host)
}
