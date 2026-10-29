package throughputcore

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CleanupStaleAuthDirs recovers private files left by a hard Agent restart.
// Active jobs live at most two minutes. Only private, owned directories older
// than three minutes under the OS temporary directory are eligible.
func CleanupStaleAuthDirs(now time.Time) {
	root := os.TempDir()
	entries, e := os.ReadDir(root)
	if e != nil {
		return
	}
	for _, v := range entries {
		if !strings.HasPrefix(v.Name(), "subl-throughput-") || !v.IsDir() || v.Type()&os.ModeSymlink != 0 {
			continue
		}
		p := filepath.Join(root, v.Name())
		st, e := os.Lstat(p)
		if e != nil || !st.IsDir() || st.Mode().Perm() != 0700 || !ownedDirectory(st) || now.Sub(st.ModTime()) < 3*time.Minute {
			continue
		}
		_ = os.RemoveAll(p)
	}
}
