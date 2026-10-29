package xraycore

import (
	"context"
	"errors"
	"net"
	"strconv"
	"testing"
	"time"

	"ppeelink/relaycore"
)

type relayLoopSource struct {
	specs   []Spec
	fail    map[int]bool
	reports []Report
	calls   int
	cancel  context.CancelFunc
}

func (s *relayLoopSource) Spec(context.Context) (Spec, error) {
	i := s.calls
	s.calls++
	if s.fail[i] {
		return Spec{}, errors.New("panel unavailable")
	}
	return s.specs[i], nil
}
func (s *relayLoopSource) Report(_ context.Context, r Report) error {
	s.reports = append(s.reports, r)
	if len(s.reports) == len(s.specs) {
		s.cancel()
	}
	return nil
}
func relayFreePort(t *testing.T) int {
	t.Helper()
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}
func TestAgentRelaySurvivesFetchFailureAndBadRuleThenDisables(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	runner := NewRunner(t.TempDir(), "relay-loop-test")
	runner.systemd = false
	rule := relaycore.Spec{ID: "line", ListenPort: relayFreePort(t), Network: "both", TargetHost: "8.8.8.8", TargetPort: 443}
	bad := rule
	bad.TargetHost = "127.0.0.1"
	source := &relayLoopSource{cancel: cancel, fail: map[int]bool{1: true}, specs: []Spec{{Enabled: true, Relays: []relaycore.Spec{rule}}, {}, {Enabled: true, Relays: []relaycore.Spec{bad}}, {Enabled: false, Relays: []relaycore.Spec{rule}}}}
	RunLoop(ctx, source, runner, time.Millisecond)
	if len(source.reports) != 4 {
		t.Fatalf("reports=%d", len(source.reports))
	}
	for i, r := range source.reports {
		if r.ProxyFeatures != 5 {
			t.Fatal("relay capability not advertised")
		}
		s := r.Relays[rule.ID]
		if i < 3 && (!s.Running || s.Revision != rule.Rev()) {
			t.Fatalf("tick %d lost last good config: %+v", i, s)
		}
		if i == 2 && s.Error == "" {
			t.Fatal("bad rule did not report per-line error")
		}
		if i == 3 && s.Running {
			t.Fatal("disabled server still forwards")
		}
	}
	if source.reports[0].Rev != source.reports[2].Rev {
		t.Fatal("failed relay edit changed Xray revision")
	}
	if c, e := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(rule.ListenPort)), 50*time.Millisecond); e == nil {
		c.Close()
		t.Fatal("relay listener remains after disable")
	}
}
func TestAgentRestoresRelaysWhenPanelUnavailable(t *testing.T) {
	dir := t.TempDir()
	rule := relaycore.Spec{ID: "restored", ListenPort: relayFreePort(t), Network: "tcp", TargetHost: "8.8.8.8", TargetPort: 443}
	m := relaycore.NewManager(dir)
	if e := m.Apply(context.Background(), []relaycore.Spec{rule}); e != nil {
		t.Fatal(e)
	}
	session := m.Status()[rule.ID].Session
	m.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	runner := NewRunner(dir, "restore-relay-test")
	runner.systemd = false
	source := &relayLoopSource{cancel: cancel, fail: map[int]bool{0: true}, specs: []Spec{{}}}
	RunLoop(ctx, source, runner, time.Millisecond)
	got := source.reports[0].Relays[rule.ID]
	if !got.Running || got.Session == session || got.Revision != rule.Rev() {
		t.Fatalf("failed to restore with new cumulative-counter session: %+v", got)
	}
}

type relayUpdateSource struct {
	relayLoopSource
	atUpdate map[string]relaycore.Status
	updated  bool
}

func (s *relayUpdateSource) Update(context.Context, string) error {
	s.updated = true
	if len(s.reports) > 0 {
		s.atUpdate = s.reports[len(s.reports)-1].Relays
	}
	return errors.New("test update unavailable")
}
func (s *relayUpdateSource) Spec(context.Context) (Spec, error) { return s.specs[0], nil }
func (s *relayUpdateSource) Report(_ context.Context, r Report) error {
	s.reports = append(s.reports, r)
	if s.updated {
		s.cancel()
	}
	return nil
}
func TestAgentSelfUpdateReportsRestoredRelayCountersFirst(t *testing.T) {
	dir := t.TempDir()
	rule := relaycore.Spec{ID: "update", ListenPort: relayFreePort(t), Network: "tcp", TargetHost: "8.8.8.8", TargetPort: 443}
	m := relaycore.NewManager(dir)
	if e := m.Apply(context.Background(), []relaycore.Spec{rule}); e != nil {
		t.Fatal(e)
	}
	m.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	runner := NewRunner(dir, "relay-update-test")
	runner.systemd = false
	source := &relayUpdateSource{relayLoopSource: relayLoopSource{cancel: cancel, specs: []Spec{{Enabled: true, Relays: []relaycore.Spec{rule}, AgentUpdate: true, AgentHash: "replacement"}}}}
	RunLoop(ctx, source, runner, time.Millisecond)
	if !source.updated || !source.atUpdate[rule.ID].Running || source.atUpdate[rule.ID].Session == "" {
		t.Fatal("self-update did not checkpoint live native relay counters")
	}
}
func TestAgentUninstallStopsRestoredRelays(t *testing.T) {
	dir := t.TempDir()
	rule := relaycore.Spec{ID: "uninstall", ListenPort: relayFreePort(t), Network: "both", TargetHost: "8.8.8.8", TargetPort: 443}
	m := relaycore.NewManager(dir)
	if e := m.Apply(context.Background(), []relaycore.Spec{rule}); e != nil {
		t.Fatal(e)
	}
	m.Close()
	runner := NewRunner(dir, "relay-uninstall-test")
	runner.systemd = false
	source := &fakeSource{spec: Spec{Uninstall: true}}
	if !RunLoop(context.Background(), source, runner, time.Second) {
		t.Fatal("uninstall did not complete")
	}
	status := source.reports[len(source.reports)-1].Relays[rule.ID]
	if status.Running || status.Session == "" {
		t.Fatalf("bad final relay status: %+v", status)
	}
	if c, e := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(rule.ListenPort)), 50*time.Millisecond); e == nil {
		c.Close()
		t.Fatal("uninstall left relay listening")
	}
}

func TestAgentDisablesRelaysBeforeSelfUpdate(t *testing.T) {
	dir := t.TempDir()
	rule := relaycore.Spec{ID: "disable-update", ListenPort: relayFreePort(t), Network: "tcp", TargetHost: "8.8.8.8", TargetPort: 443}
	m := relaycore.NewManager(dir)
	if e := m.Apply(context.Background(), []relaycore.Spec{rule}); e != nil {
		t.Fatal(e)
	}
	m.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	runner := NewRunner(dir, "relay-disable-update-test")
	runner.systemd = false
	source := &relayUpdateSource{relayLoopSource: relayLoopSource{cancel: cancel, specs: []Spec{{Enabled: false, Relays: []relaycore.Spec{rule}, AgentUpdate: true, AgentHash: "replacement"}}}}
	RunLoop(ctx, source, runner, time.Millisecond)
	if !source.updated || source.atUpdate[rule.ID].Running {
		t.Fatal("server disable waited until after agent update")
	}
}
