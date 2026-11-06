package node

// Probe definitions and regional lists adapted from yt1988/unlock-check,
// commit e941ebde9413ee844ecd34e8ea4ed8ccf7ac0a43 (MIT). See THIRD_PARTY_NOTICES.md.
const anthropicRegions = "AL DZ AD AO AG AR AM AU AT AZ BS BH BD BB BE BZ BJ BT BO BA BW BR BN BG BF BI CV KH CM CA CF TD CL CO KM CG CD CR CI HR CY CZ DK DJ DM DO EC EG SV GQ ER EE SZ ET FJ FI FR GA GM GE DE GH GR GD GT GN GW GY HT HN HU IS IN ID IQ IE IL IT JM JP JO KZ KE KI KW KG LA LV LB LS LR LY LI LT LU MG MW MY MV ML MT MH MR MU MX FM MD MC MN ME MA MZ NA NR NP NL NZ NI NE NG MK NO OM PK PW PS PA PG PY PE PH PL PT QA RO RW KN LC VC WS SM ST SA SN RS SC SL SG SK SI SO SB ZA KR SS ES LK SD SR SE CH TW TJ TZ TH TL TG TO TT TN TR TM TV UG UA AE GB US UY UZ VU VA VN ZM ZW"
const sanctionedRegions = "CU IR KP SY"

var defaultUnlockServices = []UnlockService{
	{Key: "openai", Name: "OpenAI / ChatGPT", Group: "ai", Kind: "openai", URL: "https://api.openai.com/compliance/cookie_requirements", ReachURL: "https://chatgpt.com/cdn-cgi/trace"},
	{Key: "claude", Name: "Claude", Group: "ai", Kind: "trace", URL: "https://claude.ai/cdn-cgi/trace", Allow: anthropicRegions},
	{Key: "google-gemini", Name: "Google Gemini", Group: "ai", Kind: "gemini", URL: "https://gemini.google.com/", ReachURL: "https://gemini.google.com/"},
	{Key: "copilot", Name: "Copilot", Group: "ai", Kind: "trace", URL: "https://copilot.microsoft.com/cdn-cgi/trace", Deny: "CN RU BY " + sanctionedRegions},
	{Key: "grok", Name: "Grok", Group: "ai", Kind: "trace", URL: "https://grok.com/cdn-cgi/trace", Deny: "CN " + sanctionedRegions},
	{Key: "perplexity", Name: "Perplexity", Group: "ai", Kind: "trace", URL: "https://www.perplexity.ai/cdn-cgi/trace", Deny: sanctionedRegions},
	{Key: "mistral", Name: "Le Chat", Group: "ai", Kind: "trace", URL: "https://chat.mistral.ai/cdn-cgi/trace", Deny: sanctionedRegions},
	{Key: "poe", Name: "Poe", Group: "ai", Kind: "trace", URL: "https://poe.com/cdn-cgi/trace", Deny: "CN " + sanctionedRegions},
	{Key: "netflix", Name: "Netflix", Group: "video", Kind: "netflix", URL: "https://www.netflix.com/", Deny: "CN KP RU SY"},
	{Key: "youtube", Name: "YouTube", Group: "video", Kind: "reach", URL: "https://www.youtube.com/generate_204"},
	{Key: "disney", Name: "Disney+", Group: "video", Kind: "disney", URL: "https://www.disneyplus.com/", ReachURL: "https://www.disneyplus.com/robots.txt"},
	{Key: "dazn", Name: "DAZN", Group: "video", Kind: "dazn", URL: "https://startup.core.indazn.com/misl/v5/Startup", Method: "POST", Body: `{"Version":"2","LandingPageKey":"generic","Languages":"en","Platform":"web","Manufacturer":"","PromoCode":"","PlatformAttributes":{}}`},
	{Key: "crunchyroll", Name: "Crunchyroll", Group: "video", Kind: "trace", URL: "https://www.crunchyroll.com/cdn-cgi/trace", Deny: "CN " + sanctionedRegions},
	{Key: "tvb", Name: "TVB Anywhere+", Group: "local", Kind: "tvb", URL: "https://uapisfm.tvbanywhere.com.sg/geoip/check/platform/android"},
	{Key: "google", Name: "Google", Group: "forum", Kind: "reach", URL: "https://www.google.com/generate_204"},
	{Key: "github", Name: "GitHub", Group: "forum", Kind: "reach", URL: "https://github.com/"},
	{Key: "telegram", Name: "Telegram", Group: "forum", Kind: "reach", URL: "https://telegram.org/robots.txt"},
	{Key: "tiktok", Name: "TikTok", Group: "forum", Kind: "reach", URL: "https://www.tiktok.com/robots.txt"},
}
