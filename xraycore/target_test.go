package xraycore

import (
	"os"
	"testing"
	"time"
)

func TestCheckTarget(t *testing.T) {
	if os.Getenv("XRAY_E2E_DIR") == "" {
		t.Skip()
	}
	for _, h := range []string{"www.microsoft.com", "www.apple.com", "dl.google.com"} {
		t.Logf("%s %+v", h, CheckRealityTarget(h+":443", h, 5*time.Second))
	}
}
