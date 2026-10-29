package throughputcore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func decodeOutput(b []byte, v any) error { return json.Unmarshal(b, v) }
func listenerReady(pid, port int) bool {
	entries, e := os.ReadDir(fmt.Sprintf("/proc/%d/fd", pid))
	if e != nil {
		return false
	}
	inodes := map[string]bool{}
	for _, entry := range entries {
		s, e := os.Readlink(filepath.Join(fmt.Sprintf("/proc/%d/fd", pid), entry.Name()))
		if e == nil && strings.HasPrefix(s, "socket:[") {
			inodes[strings.TrimSuffix(strings.TrimPrefix(s, "socket:["), "]")] = true
		}
	}
	for _, name := range []string{"tcp", "tcp6"} {
		b, e := os.ReadFile(fmt.Sprintf("/proc/%d/net/%s", pid, name))
		if e != nil {
			continue
		}
		for _, line := range strings.Split(string(b), "\n") {
			f := strings.Fields(line)
			if len(f) < 10 || f[3] != "0A" || !inodes[f[9]] {
				continue
			}
			parts := strings.Split(f[1], ":")
			n, e := strconv.ParseInt(parts[len(parts)-1], 16, 32)
			if e == nil && int(n) == port {
				return true
			}
		}
	}
	return false
}
