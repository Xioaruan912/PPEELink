package api

import (
	"ppeelink/models"
	"strconv"
	"strings"
	"testing"
)

func TestDiagnosticPath(t *testing.T) {
	setupTestDB(t)
	a := models.Node{Name: "entry", Link: testLinkA}
	models.DB.Create(&a)
	b := models.Node{Name: "exit", Link: testLinkB, Flags: encodeNodeFlags(decodeNodeFlags("{}"))}
	models.DB.Create(&b)
	models.DB.Model(&b).Update("flags", `{"dialerProxyId":`+strconv.Itoa(a.ID)+`}`)
	path, e := diagnosticPath(b.ID)
	if e != nil || len(path) != 2 || path[0].Name != "entry" || path[1].Name != "exit" {
		t.Fatalf("path=%+v error=%v", path, e)
	}
	models.DB.Model(&a).Update("flags", `{"dialerProxyId":`+strconv.Itoa(b.ID)+`}`)
	if _, e = diagnosticPath(b.ID); e == nil || !strings.Contains(e.Error(), "循环") {
		t.Fatalf("cycle accepted: %v", e)
	}
	models.DB.Model(&a).Update("flags", `{"dialerProxyId":999999}`)
	if _, e = diagnosticPath(b.ID); e == nil {
		t.Fatal("missing relay accepted")
	}
	models.DB.Model(&a).Update("flags", `broken`)
	if _, e = diagnosticPath(b.ID); e == nil {
		t.Fatal("malformed flags accepted")
	}
}

func TestChainUpdateRejectsCycles(t *testing.T) {
	setupTestDB(t)
	a := models.Node{Name: "a", Link: testLinkA}
	models.DB.Create(&a)
	b := models.Node{Name: "b", Link: testLinkB}
	models.DB.Create(&b)
	models.DB.Model(&a).Update("flags", `{"dialerProxyId":`+strconv.Itoa(b.ID)+`}`)
	res := jsonRequest(t, NodeBatch, nodeBatchRequest{IDs: []int{b.ID}, Action: "chain", RelayID: a.ID}, nil)
	if res.Code != 400 {
		t.Fatalf("cycle saved: %d", res.Code)
	}
	var saved models.Node
	models.DB.First(&saved, b.ID)
	if decodeNodeFlags(saved.Flags).DialerProxyID != 0 {
		t.Fatal("invalid update was partially saved")
	}
}

func TestRelayExportIncludesAncestors(t *testing.T) {
	setupTestDB(t)
	a := models.Node{Name: "database entry", Link: testLinkA}
	models.DB.Create(&a)
	b := models.Node{Name: "database middle", Link: testLinkB, Flags: `{"dialerProxyId":` + strconv.Itoa(a.ID) + `}`}
	models.DB.Create(&b)
	exit := models.Node{Name: "exit", Link: "trojan://secret@exit.example.com:443#Exit", Flags: `{"dialerProxyId":` + strconv.Itoa(b.ID) + `}`}
	models.DB.Create(&exit)
	urls, flags := collectNodeInputs([]models.Node{exit})
	if len(urls) != 3 || flags[b.Link].DialerProxy != "Alpha" || flags[exit.Link].DialerProxy != "Bravo" {
		t.Fatalf("ancestor flags lost: urls %d flags %+v", len(urls), flags)
	}
}
