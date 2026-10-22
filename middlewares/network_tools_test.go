package middlewares

import "testing"

func TestNetworkToolsRequireAdminScopes(t *testing.T) {
	for _, method := range []string{"GET", "POST", "DELETE"} {
		for _, path := range []string{"/api/v1/network/topology", "/api/v1/network/traces/create", "/api/v1/monitors/files/list", "/api/v1/monitors/files/download", "/api/v1/monitors/docker/list", "/api/v1/monitors/docker/action", "/api/v1/monitors/ssh/ticket"} {
			if got := requiredScope(method, path); got != "admin" {
				t.Errorf("%s %s got %s", method, path, got)
			}
		}
	}
	if got := requiredScope("GET", "/api/v1/monitors/list"); got != "read" {
		t.Fatal("monitor metrics read unnecessarily privileged")
	}
}
