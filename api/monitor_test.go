package api

import (
	"testing"
	"time"

	"ppeelink/models"
)

func TestDownsampleMonitorMetrics(t *testing.T) {
	rows := make([]models.MonitorMetric, 1000)
	start := time.Now().Add(-time.Hour)
	for i := range rows {
		rows[i] = models.MonitorMetric{
			MonitorID: 1, CollectedAt: start.Add(time.Duration(i) * time.Second),
			CPUPercent: float64(i % 100), MemUsed: uint64(i * 100), MemTotal: 100000,
			DiskUsed: 500, DiskTotal: 1000,
		}
	}
	out := downsampleMonitorMetrics(rows, 100)
	if len(out) == 0 || len(out) > 100 {
		t.Fatalf("unexpected bucket count: %d", len(out))
	}
	if out[0].MemTotal != 100000 {
		t.Fatalf("average lost total memory: %+v", out[0])
	}
	if out[len(out)-1].CollectedAt.Before(out[0].CollectedAt) {
		t.Fatalf("timestamps out of order")
	}
}
