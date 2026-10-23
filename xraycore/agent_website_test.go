package xraycore

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

type websiteRetrySource struct {
	runner  *Runner
	reports []Report
	cancel  context.CancelFunc
}

func (s *websiteRetrySource) Spec(context.Context) (Spec, error) {
	return Spec{Enabled: false, APIPort: 62789}, nil
}
func (s *websiteRetrySource) Report(_ context.Context, r Report) error {
	s.reports = append(s.reports, r)
	if len(s.reports) == 1 {
		k, _ := s.runner.websiteStats.reserve("in-1", "second.example", 3600)
		s.runner.websiteStats.add(k, 17, true)
		return errors.New("response lost after remote commit")
	}
	if len(s.reports) == 3 {
		s.cancel()
	}
	return nil
}
func TestWebsiteRetryRetainsExactBatch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r := NewRunner(t.TempDir(), "website-loop-test")
	r.systemd = false
	r.websiteStats = newWebsiteCounters()
	k, err := r.websiteStats.reserve("in-1", "first.example", 3600)
	if err != nil {
		t.Fatal(err)
	}
	r.websiteStats.add(k, 23, false)
	source := &websiteRetrySource{runner: r, cancel: cancel}
	if RunLoop(ctx, source, r, time.Millisecond) {
		t.Fatal("not an uninstall")
	}
	if len(source.reports) != 3 {
		t.Fatalf("reports %d", len(source.reports))
	}
	a, b, c := source.reports[0], source.reports[1], source.reports[2]
	if a.WebsiteBatchID == "" || a.WebsiteBatchID != b.WebsiteBatchID || !reflect.DeepEqual(a.WebsiteTraffic, b.WebsiteTraffic) {
		t.Fatal("retry changed batch ID/payload")
	}
	if c.WebsiteBatchID == a.WebsiteBatchID || len(c.WebsiteTraffic) != 1 || c.WebsiteTraffic[0].Domain != "second.example" || c.WebsiteTraffic[0].Up != 17 {
		t.Fatalf("new flow merged with acknowledged batch: %+v", c)
	}
	if a.Running || b.Running || a.Rev == "" {
		t.Fatal("disabled proxy must still apply configuration and report")
	}
}

type websiteRestoreSource struct {
	cancel context.CancelFunc
	report Report
	fail   bool
}

func (s *websiteRestoreSource) Spec(context.Context) (Spec, error) { return Spec{}, nil }
func (s *websiteRestoreSource) Report(_ context.Context, r Report) error {
	s.report = r
	s.cancel()
	if s.fail {
		return errors.New("offline")
	}
	return nil
}
func TestWebsiteLoopRestoresPendingBatch(t *testing.T) {
	dir := t.TempDir()
	id := "0123456789abcdef0123456789abcdef"
	buckets := []WebsiteTrafficBucket{{BucketStart: time.Now().UTC().Truncate(time.Hour).Unix(), InboundTag: "in-1", Domain: "restore.example", Up: 19, Down: 31}}
	if err := saveWebsiteBatch(dir, id, buckets); err != nil {
		t.Fatal(err)
	}
	for _, fail := range []bool{true, false} {
		ctx, cancel := context.WithCancel(context.Background())
		r := NewRunner(dir, "restore-test")
		r.systemd = false
		s := &websiteRestoreSource{cancel: cancel, fail: fail}
		RunLoop(ctx, s, r, time.Millisecond)
		cancel()
		if s.report.WebsiteBatchID != id || !reflect.DeepEqual(s.report.WebsiteTraffic, buckets) {
			t.Fatalf("restored batch changed: %+v", s.report)
		}
		gotID, got, err := loadWebsiteBatch(dir)
		if err != nil {
			t.Fatal(err)
		}
		if fail && (gotID != id || !reflect.DeepEqual(got, buckets)) {
			t.Fatal("failed report removed pending spool")
		}
		if !fail && (gotID != "" || len(got) != 0) {
			t.Fatal("acknowledged batch not removed")
		}
	}
}

type intervalSource struct{ cancel context.CancelFunc }

func (s intervalSource) Spec(context.Context) (Spec, error)   { return Spec{}, nil }
func (s intervalSource) Report(context.Context, Report) error { s.cancel(); return nil }
func TestAgentNonPositiveInterval(t *testing.T) {
	for _, d := range []time.Duration{0, -time.Second} {
		ctx, cancel := context.WithCancel(context.Background())
		r := NewRunner(t.TempDir(), "interval-test")
		r.systemd = false
		RunLoop(ctx, intervalSource{cancel}, r, d)
		cancel()
	}
}
