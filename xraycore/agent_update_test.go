package xraycore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExecutableHashUsesRunningImage(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux executable inode regression")
	}
	if os.Getenv("SUBL_HASH_TEST_CHILD") == "1" {
		exe, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		original, err := os.ReadFile(exe)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(original)
		// Only the disposable copy running in this child is replaced.
		if err := os.WriteFile(exe+".next", []byte("not the running image"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(exe+".next", exe); err != nil {
			t.Fatal(err)
		}
		if got := ExecutableHash(); got != hex.EncodeToString(sum[:]) {
			t.Fatalf("confirmed replaced file rather than running image: %s", got)
		}
		return
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	copy := filepath.Join(t.TempDir(), "agent-hash-test")
	if err := os.WriteFile(copy, raw, 0700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(copy, "-test.run=^TestExecutableHashUsesRunningImage$")
	cmd.Env = append(os.Environ(), "SUBL_HASH_TEST_CHILD=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("hash subprocess failed: %v\n%s", err, out)
	}
}

func TestUpdateExecutableRejectsBadChecksum(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Monitor test" {
			t.Error("update lost role credential")
		}
		w.Write([]byte("not the expected binary"))
	}))
	defer server.Close()
	err := UpdateExecutable(context.Background(), server.Client(), server.URL, "Monitor test", strings.Repeat("0", 64))
	if err == nil || !strings.Contains(err.Error(), "校验值不匹配") {
		t.Fatalf("unexpected result: %v", err)
	}
}

func TestReplaceExecutableRollsBackFailedRestart(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "agent")
	if err := os.WriteFile(exe, []byte("old"), 0751); err != nil {
		t.Fatal(err)
	}
	err := replaceExecutable(exe, strings.NewReader("new"), updateTestHash("new"), func(path string) error {
		raw, e := os.ReadFile(path)
		if e != nil || string(raw) != "new" {
			t.Fatalf("replacement not installed: %s %v", raw, e)
		}
		return errors.New("exec failed")
	})
	if err == nil {
		t.Fatal("failed restart was accepted")
	}
	raw, err := os.ReadFile(exe)
	if err != nil || string(raw) != "old" {
		t.Fatalf("old executable not restored: %s %v", raw, err)
	}
	info, _ := os.Stat(exe)
	if info.Mode().Perm() != 0751 {
		t.Fatal("original permissions changed")
	}
	files, _ := os.ReadDir(filepath.Dir(exe))
	if len(files) != 1 {
		t.Fatalf("temporary updates leaked: %v", files)
	}
}

func BenchmarkExecutableHash(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if ExecutableHash() == "" {
			b.Fatal("empty hash")
		}
	}
}

func updateTestHash(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

type failedUpdateReader struct{}

func (failedUpdateReader) Read(p []byte) (int, error) { return 0, io.ErrUnexpectedEOF }

type endlessUpdateReader struct{}

func (endlessUpdateReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }

func TestReplaceExecutableRejectsIncompleteAndOversizedStreams(t *testing.T) {
	for _, tt := range []struct {
		name   string
		reader io.Reader
		hash   string
	}{
		{"bad checksum", strings.NewReader("new"), updateTestHash("different")},
		{"read error", io.MultiReader(strings.NewReader("partial"), failedUpdateReader{}), updateTestHash("partial")},
		{"oversized", endlessUpdateReader{}, updateTestHash("new")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			exe := filepath.Join(t.TempDir(), "agent")
			if err := os.WriteFile(exe, []byte("old"), 0751); err != nil {
				t.Fatal(err)
			}
			restarted := false
			err := replaceExecutable(exe, tt.reader, tt.hash, func(string) error { restarted = true; return nil })
			if err == nil || restarted {
				t.Fatalf("invalid update accepted: %v", err)
			}
			data, _ := os.ReadFile(exe)
			if string(data) != "old" {
				t.Fatal("old executable changed")
			}
			entries, _ := os.ReadDir(filepath.Dir(exe))
			if len(entries) != 1 {
				t.Fatalf("temporary files leaked: %v", entries)
			}
		})
	}
}

func TestReplaceExecutableStreamsVerifiedImage(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "agent")
	if err := os.WriteFile(exe, []byte("old"), 0751); err != nil {
		t.Fatal(err)
	}
	err := replaceExecutable(exe, strings.NewReader("new"), updateTestHash("new"), func(path string) error {
		data, e := os.ReadFile(path)
		if e != nil || string(data) != "new" {
			t.Fatalf("bad installed image: %s %v", data, e)
		}
		info, e := os.Stat(path)
		if e != nil || info.Mode().Perm() != 0751 {
			t.Fatal("mode not preserved")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Dir(exe))
	if len(entries) != 1 {
		t.Fatalf("temporary files leaked: %v", entries)
	}
}
