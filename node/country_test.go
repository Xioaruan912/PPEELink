package node

import "testing"

func TestCountryFromNameUsesExplicitTokens(t *testing.T) {
	cases := map[string]string{
		"[备用]01_HK":         "HK",
		"[备用]Nosla-JP":      "JP",
		"lighelayer_USA":    "US",
		"AKKO_USA":          "US",
		"Test Reality":      "", // reality 里的 it 不能识别成意大利
		"my server":         "", // 小写 my 不是马来西亚
		"🇯🇵 东京 01":          "JP",
		"中国香港 IPLC":         "HK",
		"印度尼西亚 01":          "ID",
		"印度 01":             "IN",
		"HK CN2 GIA":        "HK",
		"CN2 GIA 美国":        "US",
		"Tokyo-IIJ":         "JP",
		"UK London":         "GB",
		"新加坡 x2":            "SG",
		"未知":                "",
		"SG 01 / via HK":    "SG", // 取最靠前的线索
		"Singapore Premium": "SG",
	}
	for name, want := range cases {
		if got := CountryFromName(name); got != want {
			t.Errorf("CountryFromName(%q)=%q want %q", name, got, want)
		}
	}
}

func TestWithFlagReplacesExistingFlag(t *testing.T) {
	if got := WithFlag("🇭🇰 Nosla-JP", "JP"); got != "🇯🇵 Nosla-JP" {
		t.Fatalf("got %q", got)
	}
	if got := WithFlag("🇭🇰 Nosla-JP", ""); got != "Nosla-JP" {
		t.Fatalf("empty code must strip: %q", got)
	}
	if CountryName("SC") != "塞舌尔" || len(Countries()) < 100 || Countries()[0].Flag == "" {
		t.Fatal("country table incomplete")
	}
}
