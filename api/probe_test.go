package api

import (
	"testing"
	"time"

	"ppeelink/models"
)

func TestApplyProbeResultsPolicies(t *testing.T) {
	setupTestDB(t)
	if err := models.DB.AutoMigrate(&models.Probe{}, &models.ProbeResult{}); err != nil {
		t.Fatal(err)
	}
	probe := models.Probe{Name: "JP", Region: "JP", TokenHash: "x", Enabled: true}
	if err := models.DB.Create(&probe).Error; err != nil {
		t.Fatal(err)
	}
	if err := models.DB.Create(&models.ProbeResult{
		ProbeID: probe.ID, NodeID: 1, Kind: "tcp", Success: true, RTT: 42, CheckedAt: time.Now(),
	}).Error; err != nil {
		t.Fatal(err)
	}

	items := []NodeOverviewItem{
		{ID: 1, Valid: false, rtt: -1, ProbePolicy: "any"},
		{ID: 1, Valid: false, rtt: -1, ProbePolicy: "all"},
		{ID: 1, Valid: false, rtt: -1, ProbePolicy: "manual"},
		{ID: 1, Valid: true, rtt: 10, ProbePolicy: "all"},
		{ID: 1, Valid: false, rtt: -1, ProbePolicy: "panel"},
	}
	applyProbeResults(items)

	want := []bool{true, false, true, true, false}
	for i, item := range items {
		if item.Valid != want[i] {
			t.Fatalf("item %d policy %s: valid=%v want %v", i, item.ProbePolicy, item.Valid, want[i])
		}
	}
	if items[0].rtt != 42 {
		t.Fatalf("probe rtt should replace failed panel rtt, got %d", items[0].rtt)
	}
	if len(items[0].Probes) != 1 || !items[0].Probes[0].Success {
		t.Fatalf("probe status not attached: %+v", items[0].Probes)
	}
}
