//go:build linux

package throughputcore

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCommandDiesWithAgentParent(t *testing.T) {
	if marker := os.Getenv("SUBL_PDEATH_PIDFILE"); marker != "" {
		cmd := exec.CommandContext(context.Background(), "sleep", "30")
		configureCommand(cmd)
		if cmd.Start() != nil {
			os.Exit(2)
		}
		_ = os.WriteFile(marker, []byte(strconv.Itoa(cmd.Process.Pid)), 0600)
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
	if _, e := exec.LookPath("sleep"); e != nil {
		t.Skip("sleep unavailable")
	}
	marker := filepath.Join(t.TempDir(), "child.pid")
	helper := exec.Command(os.Args[0], "-test.run=^TestCommandDiesWithAgentParent$")
	helper.Env = append(os.Environ(), "SUBL_PDEATH_PIDFILE="+marker)
	if e := helper.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() {
		if helper.ProcessState == nil {
			helper.Process.Kill()
			helper.Wait()
		}
	}()
	var pid int
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		b, e := os.ReadFile(marker)
		if e == nil {
			pid, _ = strconv.Atoi(string(b))
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if pid == 0 {
		t.Fatal("helper did not start child")
	}
	defer syscall.Kill(pid, syscall.SIGKILL)
	helper.Process.Kill()
	helper.Wait()
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		b, e := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
		if os.IsNotExist(e) || strings.Contains(string(b), ") Z ") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("listener survived Agent parent death")
}
