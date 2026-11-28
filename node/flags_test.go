package node

import (
	"os"
	"strings"
	"testing"
)

func TestEncodeClashWithPerNodeFlags(t *testing.T) {
	tmpl := `
proxies:
proxy-groups:
  - name: 全部
    type: select
    proxies:
      - DIRECT
`
	path := writeTempYaml(t, tmpl)
	defer os.Remove(path)

	flagged := "ss://YWVzLTI1Ni1nY206cGFzc3dvcmQxMjM0NTY3ODk@1.2.3.4:8388#FLAGGED"
	plain := "ss://YWVzLTI1Ni1nY206cGFzc3dvcmQxMjM0NTY3ODk@5.6.7.8:8388#PLAIN"
	out, err := EncodeClashWithFlags([]string{flagged, plain}, map[string]NodeFlags{
		flagged: {UDP: true, TFO: true, SkipCertVerify: true},
	}, SqlConfig{Clash: path})
	if err != nil {
		t.Fatalf("EncodeClashWithFlags error: %v", err)
	}
	s := string(out)
	for _, want := range []string{"udp: true", "tfo: true", "skip-cert-verify: true"} {
		if !strings.Contains(s, want) {
			t.Fatalf("per-node flag %q missing from clash output:\n%s", want, s)
		}
	}
	if !strings.Contains(s, "FLAGGED") || !strings.Contains(s, "PLAIN") {
		t.Fatalf("nodes missing from clash output:\n%s", s)
	}
}

func TestEncodeClashVLESSGrpcOptsOnlyForGRPC(t *testing.T) {
	tcpLink := "vless://62cf0629-888f-416d-9afc-4b919bea2129@179.255.148.213:48230?encryption=none&flow=xtls-rprx-vision&fp=chrome&pbk=6s94RIUIg4ypjXBlXPBSYsl76tkw9Rzi9OYe664_MjY&security=reality&sid=639b4713&sni=www.harvard.edu&type=tcp#TCP_NODE"
	grpcLink := "vless://62cf0629-888f-416d-9afc-4b919bea2129@179.255.148.213:48231?encryption=none&fp=chrome&pbk=6s94RIUIg4ypjXBlXPBSYsl76tkw9Rzi9OYe664_MjY&security=reality&serviceName=mygrpc&sid=639b4713&sni=www.harvard.edu&type=grpc#GRPC_NODE"

	proxies := BuildClashProxies([]string{tcpLink, grpcLink}, nil, SqlConfig{Udp: true})
	if len(proxies) != 2 {
		t.Fatalf("expected 2 proxies, got %d", len(proxies))
	}
	if len(proxies[0].Grpc_opts) != 0 {
		t.Fatalf("tcp vless should not emit grpc-opts, got %#v", proxies[0].Grpc_opts)
	}
	if !proxies[0].Udp {
		t.Fatalf("expected udp=true on tcp vless")
	}
	if proxies[1].Grpc_opts["grpc-service-name"] != "mygrpc" || proxies[1].Grpc_opts["grpc-mode"] != "gun" {
		t.Fatalf("grpc vless missing grpc-opts, got %#v", proxies[1].Grpc_opts)
	}
}
