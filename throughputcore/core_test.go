package throughputcore

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func testJob() Job {
	return Job{ID: 1, Role: "client", Host: "127.0.0.1", Port: 35201, Protocol: "tcp", Direction: "forward", Family: 4, Duration: 5, Streams: 4, RateMbps: 100, Username: "mario", Password: "rossi", PublicKey: "test", ExpiresAt: time.Now().Add(time.Minute)}
}
func TestClientBoundsAndAggregateRate(t *testing.T) {
	j := testJob()
	a, err := ClientArgs(j, "public.pem")
	if err != nil {
		t.Fatal(err)
	}
	s := strings.Join(a, " ")
	if !strings.Contains(s, "-b 25000000") || !strings.Contains(s, "-4") {
		t.Fatal(s)
	}
	for _, change := range []func(*Job){func(j *Job) { j.Streams = 9 }, func(j *Job) { j.Duration = 31 }, func(j *Job) { j.RateMbps = 0 }, func(j *Job) { j.Family = 0 }, func(j *Job) { j.ExpiresAt = time.Now().Add(-time.Second) }, func(j *Job) { j.Host = "--help" }} {
		bad := j
		change(&bad)
		if _, err := ClientArgs(bad, "p"); err == nil {
			t.Fatalf("accepted unsafe job %+v", bad)
		}
	}
}
func TestAuthFiles(t *testing.T) {
	j := testJob()
	j.Role = "server"
	j.PrivateKey = "private"
	f, err := NewAuthFiles(j)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	b, _ := os.ReadFile(f.Users)
	if string(b) != "mario,bf7a49a846d44b454a5d11e7acfaf13d138bbe0b7483aa3e050879700572709b\n" {
		t.Fatalf("bad credential hash %s", b)
	}
	for _, p := range []string{f.Users, f.Key} {
		st, _ := os.Stat(p)
		if st.Mode().Perm() != 0600 {
			t.Fatal(st.Mode())
		}
	}
	dir := f.Dir
	f.Close()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("secret dir retained")
	}
}
func TestParseReceiverAndCap(t *testing.T) {
	j := testJob()
	raw := []byte(`{"end":{"sum_sent":{"bits_per_second":100000000,"retransmits":9},"sum_received":{"bits_per_second":98000000,"bytes":61250000,"seconds":5},"cpu_utilization_percent":{"host_total":3,"remote_total":4}},"intervals":[{"sum":{"start":0,"end":1,"bits_per_second":99000000,"bytes":12375000}}]}`)
	r, e := ParseResult(raw, j)
	if e != nil {
		t.Fatal(e)
	}
	if r.Forward == nil || r.Forward.BitsPerSecond != 98000000 || r.Forward.Retransmits != 9 || !r.CapReached || r.CPUHost != 3 {
		t.Fatalf("%+v %+v", r, r.Forward)
	}
}
func TestParseReverseUDPAndBidir(t *testing.T) {
	j := testJob()
	j.Protocol = "udp"
	j.Direction = "reverse"
	r, e := ParseResult([]byte(`{"end":{"sum_received":{"bits_per_second":50000000,"bytes":100,"jitter_ms":1.5,"lost_packets":2,"packets":100,"lost_percent":2,"seconds":5}}}`), j)
	if e != nil || r.Reverse == nil || r.Reverse.LostPackets != 2 || r.CapReached {
		t.Fatalf("%+v %v", r, e)
	}
	j.Direction = "bidir"
	r, e = ParseResult([]byte(`{"end":{"sum_received":{"bits_per_second":50000000,"bytes":100},"sum_received_bidir_reverse":{"bits_per_second":40000000,"bytes":80}}}`), j)
	if e != nil || r.Forward.BitsPerSecond != 50000000 || r.Reverse.BitsPerSecond != 40000000 {
		t.Fatalf("%+v %v", r, e)
	}
}
func TestParseRejectMissingReceiver(t *testing.T) {
	if _, err := ParseResult([]byte(`{"end":{"sum_sent":{"bits_per_second":100}}}`), testJob()); err == nil {
		t.Fatal("sender-only reported as receiver")
	}
}
func TestCapabilitiesRejectOldAuth(t *testing.T) {
	for _, version := range []string{"iperf 3.16", "iperf 3.9", "iperf 3.21", "bad"} {
		c := ParseCapability("/bin/iperf3", version, "--rsa-private-key-path --rsa-public-key-path --authorized-users-path --bidir --json-stream")
		if c.AuthSupported {
			t.Fatal(version)
		}
	}
	c := ParseCapability("/bin/iperf3", "iperf 3.22", "--rsa-private-key-path --rsa-public-key-path --authorized-users-path --bidir --json-stream")
	if !c.AuthSupported || !c.BidirSupported || !c.JSONStreamSupported {
		t.Fatal(c)
	}
}
func TestTCPMeanRTTOptionalAndBidirDirection(t *testing.T) {
	j := testJob()
	j.Direction = "bidir"
	r, e := ParseResult([]byte(`{"end":{"sum_received":{"bits_per_second":10},"sum_received_bidir_reverse":{"bits_per_second":20},"streams":[{"sender":{"mean_rtt":1500,"sender":true}},{"sender":{"mean_rtt":2500,"sender":false}}]}}`), j)
	if e != nil || r.Forward.RTTMs == nil || *r.Forward.RTTMs != 1.5 || r.Reverse.RTTMs == nil || *r.Reverse.RTTMs != 2.5 {
		t.Fatalf("%+v %v", r, e)
	}
}
func TestStaleSecretsRecoveryIsScoped(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	old, e := NewAuthFiles(testJob())
	if e != nil {
		t.Fatal(e)
	}
	defer old.Close()
	recent, e := NewAuthFiles(testJob())
	if e != nil {
		t.Fatal(e)
	}
	defer recent.Close()
	then := time.Now().Add(-4 * time.Minute)
	if e = os.Chtimes(old.Dir, then, then); e != nil {
		t.Fatal(e)
	}
	unrelated := tmp + "/keep-user-files"
	if e = os.Mkdir(unrelated, 0700); e != nil {
		t.Fatal(e)
	}
	os.Chtimes(unrelated, then, then)
	CleanupStaleAuthDirs(time.Now())
	if _, e = os.Stat(old.Dir); !os.IsNotExist(e) {
		t.Fatal("stale credentials retained")
	}
	for _, p := range []string{recent.Dir, unrelated} {
		if _, e = os.Stat(p); e != nil {
			t.Fatal("unrelated or live directory removed", p)
		}
	}
}
func TestPrepareReportsMissingDependencies(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	c := Prepare(context.Background())
	if c.AuthSupported || !strings.Contains(c.Error, "requires cc") {
		t.Fatal(c)
	}
}
