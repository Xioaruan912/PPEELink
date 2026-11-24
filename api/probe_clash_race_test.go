package api

import (
	"testing"
)

func TestDispatchJobPreservesPreassignedID(t *testing.T) {
	id := newProbeJobID()
	ch := probeHub.register(999123)
	defer probeHub.unregister(999123, ch)

	gotID, _, ok := probeHub.dispatchJob(999123, queuedProbeJob{
		ID:          id,
		Action:      "clash-status",
		ClashAPI:    "http://127.0.0.1:9090",
		ClashSecret: "123",
	}, true)
	if !ok {
		t.Fatal("expected dispatchJob to succeed")
	}
	defer probeHub.cleanupDone(gotID)
	if gotID != id {
		t.Fatalf("expected preassigned ID %q, got %q", id, gotID)
	}
	job := <-ch
	if job.ID != id || job.ClashSecret != "123" || job.ClashAPI != "http://127.0.0.1:9090" {
		t.Fatalf("unexpected dispatched job: %+v", job)
	}
}

func TestServerUnlocksServiceAndGroupKey(t *testing.T) {
	if k := groupUnlockServiceKey("ChatGPT 专线"); k != "chatgpt" {
		t.Fatalf("unexpected group key: %q", k)
	}
	if k := groupUnlockServiceKey("Netflix 流媒体"); k != "netflix" {
		t.Fatalf("unexpected group key: %q", k)
	}
	rep := &serverOpsUnlockReport{
		Items: []serverOpsUnlockItem{
			{Key: "chatgpt", Status: "available"},
			{Key: "claude", Status: "blocked"},
			{Key: "netflix", Status: "originals"},
		},
	}
	if ok, known := serverUnlocksService(rep, "chatgpt"); !ok || !known {
		t.Fatalf("expected chatgpt unlocked, got ok=%v known=%v", ok, known)
	}
	if ok, known := serverUnlocksService(rep, "claude"); ok || !known {
		t.Fatalf("expected claude blocked, got ok=%v known=%v", ok, known)
	}
	if ok, known := serverUnlocksService(rep, "netflix"); !ok || !known {
		t.Fatalf("expected netflix originals allowed, got ok=%v known=%v", ok, known)
	}
	if ok, known := serverUnlocksService(rep, "ai"); !ok || !known {
		t.Fatalf("expected ai group unlocked when chatgpt is available, got ok=%v known=%v", ok, known)
	}
}
