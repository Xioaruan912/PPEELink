package api

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"sync"
	"testing"

	"ppeelink/models"
)

func TestWebDAVBackupClientAndRetention(t *testing.T) {
	setupTestDB(t)
	_ = models.DB.AutoMigrate(&models.WebDAVBackupSetting{})
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	if err := os.MkdirAll("template", 0755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile("template/demo.yaml", []byte("port: 7890\n"), 0644)

	var mu sync.Mutex
	store := map[string][]byte{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "test@example.com" || pass != "app-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case "MKCOL":
			w.WriteHeader(http.StatusCreated)
		case "PUT":
			body, _ := io.ReadAll(r.Body)
			store[path.Base(r.URL.Path)] = body
			w.WriteHeader(http.StatusCreated)
		case "GET":
			name := path.Base(r.URL.Path)
			data, exists := store[name]
			if !exists {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data)
		case "DELETE":
			delete(store, path.Base(r.URL.Path))
			w.WriteHeader(http.StatusNoContent)
		case "PROPFIND":
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusMultiStatus)
			var b strings.Builder
			b.WriteString(`<?xml version="1.0" encoding="utf-8"?><d:multistatus xmlns:d="DAV:">`)
			for name, data := range store {
				b.WriteString(fmt.Sprintf(`<d:response><d:href>/dav/ppeelink-backups/%s</d:href><d:propstat><d:prop><d:getcontentlength>%d</d:getcontentlength><d:getlastmodified>Thu, 27 Nov 2026 04:00:00 GMT</d:getlastmodified></d:prop></d:propstat></d:response>`, name, len(data)))
			}
			b.WriteString(`</d:multistatus>`)
			_, _ = w.Write([]byte(b.String()))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer srv.Close()

	cfg, err := models.GetOrCreateWebDAVBackupSetting()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Enabled = true
	cfg.URL = srv.URL + "/dav/"
	cfg.Username = "test@example.com"
	cfg.Password = "app-secret"
	cfg.RemoteDir = "ppeelink-backups"
	cfg.KeepCount = 2
	models.DB.Save(cfg)

	client, err := newWebDAVClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.testConnection(); err != nil {
		t.Fatalf("testConnection failed: %v", err)
	}

	// Pre-populate 2 older backups so retention prunes down to KeepCount=2
	store["ppeelink-backup-20260101-000001.tar.gz"] = []byte("old1")
	store["ppeelink-backup-20260102-000001.tar.gz"] = []byte("old2")

	name, size, err := runWebDAVBackupWithConfig(cfg)
	if err != nil {
		t.Fatalf("runWebDAVBackupWithConfig failed: %v", err)
	}
	if size == 0 || !strings.HasSuffix(name, ".tar.gz") {
		t.Fatalf("unexpected backup result: %s (%d)", name, size)
	}

	files, err := client.listBackups()
	if err != nil {
		t.Fatalf("listBackups failed: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected retention KeepCount=2, got %d files: %+v", len(files), files)
	}
	if files[0].Name != name {
		t.Fatalf("expected newest file %s first, got %s", name, files[0].Name)
	}
}
