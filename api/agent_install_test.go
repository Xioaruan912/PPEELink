package api

import (
	"os/exec"
	"strings"
	"testing"
)

func TestAgentRolesShareAtomicInstaller(t *testing.T) {
	for _, role := range []string{"agent", "monitor"} {
		t.Run(role, func(t *testing.T) {
			script := agentInstallScript("https://panel.example/base/", "private-token", role)
			for _, required := range []string{"mktemp", "mv -f", "?arch=$ARCH", "EnvironmentFile=", "chmod 600", "systemctl restart", "PPEELINK_AGENT_TOKEN", "ppeelink-" + role + " " + role} {
				if !strings.Contains(script, required) {
					t.Fatalf("installer missing %q", required)
				}
			}
			for _, line := range strings.Split(script, "\n") {
				if strings.HasPrefix(line, "ExecStart=") && strings.Contains(line, "private-token") {
					t.Fatal("credential exposed in process arguments")
				}
			}
			cmd := exec.Command("sh", "-n")
			cmd.Stdin = strings.NewReader(script)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("invalid shell: %v %s", err, out)
			}
		})
	}
}
