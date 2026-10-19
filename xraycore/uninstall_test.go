package xraycore

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRemoveManagedDirOnlyDeletesOurDirs(t *testing.T) {
	if err := removeManagedDir("/"); err == nil {
		t.Fatal("must refuse root")
	}
	if err := removeManagedDir("relative/dir"); err == nil {
		t.Fatal("must refuse relative paths")
	}
	foreign := t.TempDir()
	keep := filepath.Join(foreign, "notes.txt")
	_ = os.WriteFile(keep, []byte("x"), 0o600)
	if err := removeManagedDir(foreign); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatal("directory without xray files must be left alone")
	}
	ours := filepath.Join(t.TempDir(), "xray")
	_ = os.MkdirAll(ours, 0o700)
	_ = os.WriteFile(filepath.Join(ours, "config.json"), []byte("{}"), 0o600)
	if err := removeManagedDir(ours); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ours); !os.IsNotExist(err) {
		t.Fatal("managed directory must be removed")
	}
}

type fakeSource struct {
	spec    Spec
	reports []Report
}

func (f *fakeSource) Spec(context.Context) (Spec, error) { return f.spec, nil }
func (f *fakeSource) Report(_ context.Context, r Report) error {
	f.reports = append(f.reports, r)
	return nil
}

func TestRunLoopUninstallsWhenAsked(t *testing.T) {
	t.Setenv("PPEELINK_XRAY_SYSTEMD", "0")
	dir := filepath.Join(t.TempDir(), "agent")
	_ = os.MkdirAll(dir, 0o700)
	_ = os.WriteFile(filepath.Join(dir, "config.json"), []byte("{}"), 0o600)
	src := &fakeSource{spec: Spec{Uninstall: true}}
	if !RunLoop(context.Background(), src, NewRunner(dir, "test-xray"), time.Hour) {
		t.Fatal("RunLoop must return after uninstalling")
	}
	if len(src.reports) != 1 || !src.reports[0].Uninstalled {
		t.Fatalf("uninstall must be reported: %+v", src.reports)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("xray directory must be removed")
	}
}
