package xraycore

import (
	"ppeelink/relaycore"
	"testing"
)

func TestRuntimeRevisionIgnoresManagementState(t *testing.T) {
	base := Spec{Enabled: true, Version: DefaultVersion, APIPort: 62789, Inbounds: []InboundSpec{{Tag: "in-1", Port: 443}}, Monitor: &MonitorSpec{Enabled: true, IntervalSeconds: 60}}
	changed := base
	changed.Monitor = &MonitorSpec{Enabled: false, IntervalSeconds: 10}
	changed.AgentHash = "new-panel-image"
	changed.AgentUpdate = true
	if base.Rev() != changed.Rev() {
		t.Fatal("management-only changes would restart Xray")
	}
	changed.Inbounds = []InboundSpec{{Tag: "in-1", Port: 8443}}
	if base.Rev() == changed.Rev() {
		t.Fatal("inbound change was ignored")
	}
	changed = base
	changed.WebsiteTraffic = &WebsiteTrafficSpec{Enabled: true, BucketSeconds: 3600}
	if base.Rev() == changed.Rev() {
		t.Fatal("relay configuration change was ignored")
	}
}

func TestRuntimeRevisionIgnoresNativeRelays(t *testing.T) {
	base := Spec{Enabled: true}
	changed := base
	changed.Relays = []relaycore.Spec{{ID: "line", ListenPort: 8443, Network: "both", TargetHost: "1.1.1.1", TargetPort: 443}}
	if base.Rev() != changed.Rev() {
		t.Fatal("native forwarding edit would restart Xray")
	}
}
