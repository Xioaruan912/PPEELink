package api

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"ppeelink/models"
)

func TestNodeAddRecognizesMierusAndKeepsItsParameters(t *testing.T) {
	setupTestDB(t)
	t.Cleanup(InvalidateOverview)
	link := "mierus://primary:example-secret@192.0.2.1?handshake-mode=HANDSHAKE_NO_WAIT&mtu=1400&multiplexing=MULTIPLEXING_OFF&port=6901&profile=default&protocol=TCP"
	form := url.Values{"link": {link}, "group": {"Mieru"}}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/nodes/add", strings.NewReader(form.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	NodeAdd(c)
	if w.Code != 200 {
		t.Fatalf("import failed: %d %s", w.Code, w.Body.String())
	}
	nodes, err := models.GetNodeList()
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].Name != "192.0.2.1:6901" || nodes[0].Link != link || len(nodes[0].GroupNodes) != 1 || nodes[0].GroupNodes[0].Name != "Mieru" {
		t.Fatal("imported node is missing, has the wrong name/group, or lost its parameters")
	}
}
