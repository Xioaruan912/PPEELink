package throughputcore

import (
	"bytes"
	"encoding/json"
	"time"
)

type jsonStream struct {
	job      Job
	report   func(Report)
	pending  []byte
	doc      iperfJSON
	total    int
	overflow bool
	last     time.Time
	progress []Interval
}

func newJSONStream(j Job, report func(Report)) *jsonStream {
	return &jsonStream{job: j, report: report}
}
func (w *jsonStream) Write(p []byte) (int, error) {
	n := len(p)
	w.total += n
	if w.total > 2*1024*1024 {
		w.overflow = true
		return n, nil
	}
	w.pending = append(w.pending, p...)
	for {
		i := bytes.IndexByte(w.pending, '\n')
		if i < 0 {
			break
		}
		line := w.pending[:i]
		w.consume(line)
		w.pending = w.pending[i+1:]
	}
	return n, nil
}
func (w *jsonStream) consume(b []byte) {
	var event struct {
		Event string          `json:"event"`
		Data  json.RawMessage `json:"data"`
	}
	if json.Unmarshal(b, &event) != nil {
		return
	}
	switch event.Event {
	case "interval":
		var v iperfInterval
		if json.Unmarshal(event.Data, &v) != nil {
			return
		}
		w.doc.Intervals = append(w.doc.Intervals, v)
		sum := v.Sum
		direction := "forward"
		if w.job.Direction == "reverse" {
			direction = "reverse"
		}
		if w.job.Direction == "bidir" && v.Reverse != nil {
			sum = v.Reverse
			direction = "reverse"
		}
		if sum == nil || sum.Omitted {
			return
		}
		w.progress = append(w.progress, Interval{Start: sum.Start, End: sum.End, BitsPerSecond: sum.BPS, Bytes: sum.Bytes, JitterMs: sum.Jitter, LossPercent: sum.Loss})
		if !w.last.IsZero() && time.Since(w.last) < 800*time.Millisecond {
			return
		}
		w.last = time.Now()
		observation := "receiver"
		if sum.Sender {
			observation = "sender"
		}
		r := Result{Protocol: w.job.Protocol, Direction: w.job.Direction, Duration: float64(w.job.Duration), Streams: w.job.Streams, RateMbps: w.job.RateMbps, ProgressIntervals: append([]Interval(nil), w.progress...), ProgressDirection: direction, ProgressObservation: observation}
		w.report(Report{ID: w.job.ID, Role: w.job.Role, Status: "running", Result: &r})
	case "end":
		_ = json.Unmarshal(event.Data, &w.doc.End)
	case "server_output_json":
		var s iperfJSON
		if json.Unmarshal(event.Data, &s) == nil {
			w.doc.Server = &s
		}
	case "error":
		_ = json.Unmarshal(event.Data, &w.doc.Error)
	}
}
func (w *jsonStream) Bytes() []byte {
	if len(w.pending) > 0 {
		w.consume(w.pending)
		w.pending = nil
	}
	b, _ := json.Marshal(w.doc)
	return b
}
func (w *jsonStream) Overflow() bool { return w.overflow }
