package api

import (
	"bytes"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMonitorOperationsPathValidation(t *testing.T) {
	for _, tt := range []struct {
		path     string
		mutation bool
		want     string
		valid    bool
	}{
		{"", false, "/", true}, {"/", false, "/", true}, {"/etc//./config", false, "/etc/config", true}, {"/你好/file name;$(id)", true, "/你好/file name;$(id)", true},
		{"relative", false, "", false}, {"/a/../b", false, "", false}, {"/../", true, "", false}, {"/", true, "", false}, {"///.", true, "", false}, {"/a\x00b", false, "", false}, {"/a\nb", false, "", false}, {"/\xff", false, "", false}, {"/" + strings.Repeat("a", 4096), false, "", false},
	} {
		got, err := validateRemotePath(tt.path, tt.mutation)
		if (err == nil) != tt.valid || got != tt.want {
			t.Errorf("validate(%q,%v)=%q,%v", tt.path, tt.mutation, got, err)
		}
	}
}
func TestMonitorOperationsDockerCommands(t *testing.T) {
	for _, id := range []string{"abcdef123456", "ABCDEF123456", strings.Repeat("a", 64)} {
		if !validContainerID(id) {
			t.Errorf("valid id rejected: %q", id)
		}
		for _, op := range []string{"logs", "start", "stop", "restart"} {
			cmd, err := dockerCommand(op, id)
			if err != nil || !strings.Contains(cmd, id) {
				t.Errorf("%s: %q %v", op, cmd, err)
			}
		}
	}
	for _, id := range []string{"", "abc", strings.Repeat("a", 65), "abcdef123456;id", "$(id)", "--help", "abcdef123456\n"} {
		if validContainerID(id) {
			t.Errorf("unsafe id accepted: %q", id)
		}
		if _, err := dockerCommand("logs", id); err == nil {
			t.Errorf("unsafe logs id accepted: %q", id)
		}
	}
	if _, err := dockerCommand("exec", "abcdef123456"); err == nil {
		t.Fatal("arbitrary command accepted")
	}
	for _, op := range []string{"list", "stats"} {
		cmd, err := dockerCommand(op, "; rm -rf /")
		if err != nil || strings.Contains(cmd, "rm -rf") {
			t.Fatalf("fixed command interpolated input: %q", cmd)
		}
	}
}
func TestMonitorOperationsDockerParser(t *testing.T) {
	items, err := parseDockerLines("{\"ID\":\"abcdef123456\",\"Names\":\"web\"}\n\n{\"ID\":\"123456abcdef\",\"CPUPerc\":\"1.2%\"}\n")
	if err != nil || len(items) != 2 || items[0]["Names"] != "web" {
		t.Fatalf("parse: %#v %v", items, err)
	}
	for _, bad := range []string{"not JSON", "[]", "null", "{\"ID\":", strings.Repeat("{}\n", 2001)} {
		if _, err := parseDockerLines(bad); err == nil {
			t.Errorf("accepted invalid output: %.30q", bad)
		}
	}
	empty, err := parseDockerLines("\n")
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatal("empty output must return an empty list")
	}
}
func TestMonitorOperationsBounds(t *testing.T) {
	if _, err := readBounded(strings.NewReader("1234"), 3); err == nil {
		t.Fatal("oversized read accepted")
	}
	if data, err := readBounded(strings.NewReader("123"), 3); err != nil || string(data) != "123" {
		t.Fatalf("exact limit: %q %v", data, err)
	}
	w := &cappedWriter{limit: 4}
	if n, err := w.Write([]byte("abcdef")); n != 6 || err != nil || !w.truncated || w.buffer.String() != "abcd" {
		t.Fatal("output not bounded")
	}
	_, _ = w.Write(bytes.Repeat([]byte("x"), 10))
	if w.buffer.Len() != 4 {
		t.Fatal("output grew past cap")
	}
}
func TestMonitorOperationsAdminGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tt := range []struct {
		auth   string
		admin  bool
		scopes string
		want   int
	}{{"", false, "", 403}, {"jwt", false, "", 403}, {"jwt", true, "", 200}, {"api-token", false, "read,write", 403}, {"api-token", false, "admin", 200}} {
		r := gin.New()
		r.Use(func(c *gin.Context) {
			c.Set("authType", tt.auth)
			c.Set("isAdmin", tt.admin)
			c.Set("apiScopes", tt.scopes)
		})
		r.GET("/", MonitorOperationsAdmin, func(c *gin.Context) { c.Status(200) })
		recorder := httptest.NewRecorder()
		r.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
		if recorder.Code != tt.want {
			t.Errorf("guard(%+v)=%d", tt, recorder.Code)
		}
	}
}
