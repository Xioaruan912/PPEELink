package api

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"ppeelink/models"

	"github.com/gin-gonic/gin"
)

func TestNodeForceDeliverEndpoint(t *testing.T) {
	setupTestDB(t)
	n := models.Node{Name: "A", Link: testLinkA}
	models.DB.Create(&n)

	rec := jsonRequest(t, NodeForceDeliver, map[string]any{"id": n.ID, "forceDeliver": true}, nil)
	if rec.Code != 200 {
		t.Fatalf("force deliver failed: %d %s", rec.Code, rec.Body.String())
	}
	var got models.Node
	if err := models.DB.First(&got, n.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !got.ForceDeliver {
		t.Fatal("force_deliver must be persisted")
	}

	rec = jsonRequest(t, NodeForceDeliver, map[string]any{"id": n.ID, "forceDeliver": false}, nil)
	if rec.Code != 200 {
		t.Fatalf("clear force deliver failed: %d", rec.Code)
	}
	models.DB.First(&got, n.ID)
	if got.ForceDeliver {
		t.Fatal("force_deliver must be cleared")
	}
}

func TestUpdateAlertSettingPartialKeepsWebhook(t *testing.T) {
	setupTestDB(t)
	if err := models.DB.AutoMigrate(&models.AlertSetting{}); err != nil {
		t.Fatal(err)
	}
	setting := models.AlertSetting{
		Enabled: true, WebhookURL: "https://example.com/hook", FailureThreshold: 3,
	}
	models.DB.Create(&setting)

	form := url.Values{"autoBlockInvalid": {"false"}}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("POST", "/api/v1/nodes/alerts", strings.NewReader(form.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	UpdateAlertSetting(c)
	if rec.Code != 200 {
		t.Fatalf("update failed: %d %s", rec.Code, rec.Body.String())
	}

	var got models.AlertSetting
	models.DB.First(&got)
	if got.AutoBlockInvalid {
		t.Fatal("autoBlockInvalid must be turned off")
	}
	if !got.Enabled || got.WebhookURL != "https://example.com/hook" {
		t.Fatalf("partial update wiped webhook settings: %+v", got)
	}
}

func TestReleaseStaleHealthBlocksAddsRecovery(t *testing.T) {
	setupTestDB(t)
	if err := models.DB.AutoMigrate(&models.NodeHealthEvent{}, &models.AlertSetting{}); err != nil {
		t.Fatal(err)
	}
	models.DB.Create(&models.NodeHealthEvent{NodeID: 7, NodeName: "A", Type: "down", CreatedAt: time.Now()})
	releaseStaleHealthBlocks()
	var last models.NodeHealthEvent
	models.DB.Order("id desc").First(&last)
	if last.NodeID != 7 || last.Type != "recovery" {
		t.Fatalf("expected recovery event, got %+v", last)
	}
}
