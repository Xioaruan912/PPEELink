package throughputcore

import (
	"strings"
	"testing"
)

func TestJSONStreamFinalAndProgress(t *testing.T) {
	j := testJob()
	var progress []Report
	w := newJSONStream(j, func(r Report) { progress = append(progress, r) })
	for _, s := range []string{`{"event":"interval","data":{"sum":{"start":0,"end":1,"bytes":12500000,"bits_per_second":100000000,"sender":true}}}` + "\n", `{"event":"server_output_json","data":{"intervals":[{"sum":{"start":0,"end":1,"bits_per_second":98000000,"sender":false}}]}}` + "\n", `{"event":"end","data":{"sum_received":{"bits_per_second":98000000,"bytes":61250000},"sum_sent":{"bits_per_second":100000000}}}` + "\n"} {
		_, _ = w.Write([]byte(s))
	}
	if len(progress) != 1 || progress[0].Result.ProgressObservation != "sender" || len(progress[0].Result.ProgressIntervals) != 1 {
		t.Fatalf("%+v", progress)
	}
	r, e := ParseResult(w.Bytes(), j)
	if e != nil || r.Forward == nil || len(r.Forward.Intervals) != 1 {
		t.Fatalf("%+v %v %s", r, e, strings.TrimSpace(string(w.Bytes())))
	}
}
