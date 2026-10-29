package api

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNetworkMenuIncludesPathsAndThroughput(t *testing.T) {
	rec := jsonRequest(t, GetMenus, nil, nil)
	if rec.Code != 200 {
		t.Fatalf("menus status %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(strings.ToLower(rec.Body.String()), "topology") {
		t.Fatal("removed topology is still advertised")
	}
	var payload struct {
		Data []Menu `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	for _, menu := range payload.Data {
		if menu.Path != "/network" {
			continue
		}
		if menu.Redirect != "/network/traceroute" || len(menu.Children) != 2 || menu.Children[0].Component != "network/Traceroute" || menu.Children[1].Component != "network/Throughput" {
			t.Fatalf("network menu lost its surviving destination: %+v", menu)
		}
		return
	}
	t.Fatal("network tools menu missing")
}
