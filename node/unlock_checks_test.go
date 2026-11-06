package node

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"
)

// External smoke checks are opt-in; deterministic evidence tests use fake transports.
func TestCheckFunctionsDirect(t *testing.T) {
	if os.Getenv("PPEELINK_NETWORK_TESTS") != "1" {
		t.Skip("set PPEELINK_NETWORK_TESTS=1 for external smoke checks")
	}
	c := &http.Client{Timeout: 8 * time.Second}
	for _, s := range ListUnlockServices() {
		t.Run(s.Key, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			r := probeUnlock(ctx, c, s)
			t.Logf("%s: %s/%s %s", s.Name, r.Status, r.Evidence, r.Note)
		})
	}
}
