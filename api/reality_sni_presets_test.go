package api

import (
	"slices"
	"testing"
)

func TestRegionalRealitySNIPresetsAndInference(t *testing.T) {
	cases := map[string]string{
		"NoSLA_JP":     "JP",
		"AsiaCLoud_HK": "HK",
		"DMIT_USA":     "US",
		"DE":           "EU",
		"UK":           "GB",
		"新加坡":          "SG",
		"台湾":           "TW",
		"韩国":           "KR",
		"加拿大":          "CA",
	}
	for in, want := range cases {
		if got := normalizeSNIRegionCode(in); got != want {
			t.Fatalf("normalizeSNIRegionCode(%q) = %q, want %q", in, got, want)
		}
	}

	jpTargets := buildRegionalSNITargets("JP", []string{"www.amd.com:443"})
	if len(jpTargets) > 12 || jpTargets[0] != "www.amd.com" {
		t.Fatalf("unexpected JP targets: %v", jpTargets)
	}
	if !slices.Contains(jpTargets, "www.sony.jp") || !slices.Contains(jpTargets, "www.debian.org") {
		t.Fatalf("expected JP local + global fallback targets, got %v", jpTargets)
	}

	euTargets := buildRegionalSNITargets("EU", nil)
	if slices.Contains(euTargets, "www.nokia.com") || !slices.Contains(euTargets, "www.siemens.com") {
		t.Fatalf("expected EU targets to exclude CF-fronted nokia.com and include siemens.com: %v", euTargets)
	}
}
