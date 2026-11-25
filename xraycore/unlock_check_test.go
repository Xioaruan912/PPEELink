package xraycore

import "testing"

func TestParseGeminiAvailability(t *testing.T) {
	if st, reg := parseGeminiAvailability(200, `...2,1,200,"HKG"...`); st != "blocked" || reg != "HK" {
		t.Fatalf("expected HKG blocked/HK, got %s/%s", st, reg)
	}
	if st, reg := parseGeminiAvailability(200, `...2,1,200,"CHN"...`); st != "blocked" || reg != "CN" {
		t.Fatalf("expected CHN blocked/CN, got %s/%s", st, reg)
	}
	if st, reg := parseGeminiAvailability(200, `...2,1,200,"JPN"...`); st != "available" || reg != "JP" {
		t.Fatalf("expected JPN available/JP, got %s/%s", st, reg)
	}
	if st, reg := parseGeminiAvailability(200, `...2,1,200,"USA"...`); st != "available" || reg != "US" {
		t.Fatalf("expected USA available/US, got %s/%s", st, reg)
	}
	if st, reg := parseGeminiAvailability(200, `...2,1,200,"NLD"...`); st != "available" || reg != "NL" {
		t.Fatalf("expected NLD available/NL, got %s/%s", st, reg)
	}
	// Plain HTTP 200 without marker must NOT produce a false positive
	if st, _ := parseGeminiAvailability(200, `<html><body>gemini</body></html>`); st != "blocked" {
		t.Fatalf("expected markerless HTTP 200 to be blocked, got %s", st)
	}
}
