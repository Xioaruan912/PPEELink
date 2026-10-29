package api

import (
	"encoding/json"
	"ppeelink/models"
	"testing"
)

func TestDiagnosticUsesSavedEntryWithoutLegacyClientChain(t *testing.T) {
	setupTestDB(t)
	n := models.Node{Name: "entry", Link: testLinkA, Flags: `{"dialerProxyId":999999}`}
	models.DB.Create(&n)
	path, err := diagnosticPath(n.ID)
	if err != nil || len(path) != 1 || path[0].Link != n.Link {
		t.Fatalf("diagnostic followed retired client relation: %+v %v", path, err)
	}
	models.DB.Model(&n).Update("flags", `broken`)
	if _, err := diagnosticPath(n.ID); err == nil {
		t.Fatal("malformed connection flags accepted")
	}
}

func TestRetiringClientChainsPreservesOtherFlagsAndNodeIdentity(t *testing.T) {
	setupTestDB(t)
	n := models.Node{Name: "existing", Link: testLinkA, Flags: `{"dialerProxyId":42,"udp":true,"custom":"keep"}`}
	models.DB.Create(&n)
	for i := 0; i < 2; i++ {
		if err := models.RemoveClientProxyChains(); err != nil {
			t.Fatal(err)
		}
	}
	var saved models.Node
	models.DB.First(&saved, n.ID)
	var flags map[string]any
	json.Unmarshal([]byte(saved.Flags), &flags)
	if _, exists := flags["dialerProxyId"]; exists || flags["udp"] != true || flags["custom"] != "keep" || saved.Name != n.Name || saved.Link != n.Link {
		t.Fatal("legacy cleanup changed unrelated node settings")
	}
}
