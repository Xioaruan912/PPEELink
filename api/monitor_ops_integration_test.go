package api

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"ppeelink/models"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

const fixtureContainerID = "abcdef123456abcdef123456abcdef123456abcdef123456abcdef123456abcd"

// startMonitorOpsSSHFixture serves the actual SSH/SFTP protocols. Exec requests
// are exact allowlisted fixtures; no subprocess or production host is involved.
func startMonitorOpsSSHFixture(t *testing.T) (string, int, <-chan string) {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	config := &ssh.ServerConfig{PasswordCallback: func(metadata ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
		if metadata.User() == "fixture-user" && string(password) == "fixture-password" {
			return nil, nil
		}
		return nil, fmt.Errorf("fixture authentication rejected")
	}}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	commands := make(chan string, 32)
	var connections sync.Map
	var workers sync.WaitGroup
	workers.Add(1)
	go func() {
		defer workers.Done()
		for {
			raw, err := listener.Accept()
			if err != nil {
				return
			}
			connections.Store(raw, true)
			workers.Add(1)
			go func() {
				defer workers.Done()
				defer raw.Close()
				defer connections.Delete(raw)
				connection, channels, requests, err := ssh.NewServerConn(raw, config)
				if err != nil {
					return
				}
				defer connection.Close()
				go ssh.DiscardRequests(requests)
				for channel := range channels {
					if channel.ChannelType() != "session" {
						_ = channel.Reject(ssh.UnknownChannelType, "session only")
						continue
					}
					stream, requests, err := channel.Accept()
					if err != nil {
						continue
					}
					workers.Add(1)
					go func() {
						defer workers.Done()
						defer stream.Close()
						for request := range requests {
							switch request.Type {
							case "subsystem":
								var payload struct{ Name string }
								if ssh.Unmarshal(request.Payload, &payload) != nil || payload.Name != "sftp" {
									_ = request.Reply(false, nil)
									continue
								}
								_ = request.Reply(true, nil)
								server, err := sftp.NewServer(stream)
								if err == nil {
									_ = server.Serve()
									_ = server.Close()
								}
								return
							case "exec":
								var payload struct{ Command string }
								if ssh.Unmarshal(request.Payload, &payload) != nil {
									_ = request.Reply(false, nil)
									return
								}
								commands <- payload.Command
								output, status := monitorOpsDockerFixture(payload.Command)
								_ = request.Reply(true, nil)
								_, _ = io.WriteString(stream, output)
								_, _ = stream.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
								return
							default:
								_ = request.Reply(false, nil)
							}
						}
					}()
				}
			}()
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		connections.Range(func(key, _ any) bool { _ = key.(net.Conn).Close(); return true })
		workers.Wait()
	})
	address := listener.Addr().(*net.TCPAddr)
	return "127.0.0.1", address.Port, commands
}

func monitorOpsDockerFixture(command string) (string, uint32) {
	switch command {
	case "docker ps -a --no-trunc --format '{{json .}}'":
		return `{"ID":"` + fixtureContainerID + `","Names":"fixture-web","Image":"fixture:latest","State":"running","Status":"Up 1 minute","Ports":"8080/tcp"}` + "\n", 0
	case "docker stats --no-stream --no-trunc --format '{{json .}}'":
		return `{"ID":"` + fixtureContainerID + `","Name":"fixture-web","CPUPerc":"1.2%","MemUsage":"1MiB / 1GiB","MemPerc":"0.1%","NetIO":"1kB / 2kB","BlockIO":"0B / 0B","PIDs":"2"}` + "\n", 0
	case "docker logs --tail 500 --timestamps " + fixtureContainerID + " 2>&1":
		return "2026-01-01T00:00:00Z fixture ready <not-html>\n", 0
	case "docker start " + fixtureContainerID, "docker stop --time 10 " + fixtureContainerID, "docker restart --time 10 " + fixtureContainerID:
		return fixtureContainerID + "\n", 0
	default:
		return "fixture rejected non-allowlisted command", 127
	}
}

func TestMonitorOperationsSSHTransport(t *testing.T) {
	// Deliberately not parallel: models.DB is an existing package-global dependency.
	host, port, commands := startMonitorOpsSSHFixture(t)
	oldDB := models.DB
	db, err := gorm.Open(sqlite.Open("file:monitor_ops_transport?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	models.DB = db
	t.Cleanup(func() { models.DB = oldDB; _ = sqlDB.Close() })
	if err := db.AutoMigrate(&models.Monitor{}); err != nil {
		t.Fatal(err)
	}
	// EncryptSecret/DecryptSecret only read the existing configuration (or the
	// documented fallback); this fixture never creates or changes a config file.
	encrypted, err := models.EncryptSecret("fixture-password")
	if err != nil {
		t.Fatal(err)
	}
	monitor := models.Monitor{Name: "transport fixture", TokenHash: models.HashMonitorToken("transport-fixture-token"), SSHHost: host, SSHPort: port, SSHUser: "fixture-user", SSHPasswordEnc: encrypted}
	if err := db.Create(&monitor).Error; err != nil {
		t.Fatal(err)
	}
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("authType", "jwt"); c.Set("isAdmin", true) })
	group := router.Group("/api/v1/monitors", MonitorOperationsAdmin)
	group.GET("/files/list", MonitorFilesList)
	group.GET("/files/read", MonitorFilesRead)
	group.GET("/files/download", MonitorFilesRead)
	group.POST("/files/upload", MonitorFilesWrite)
	group.POST("/files/save", MonitorFilesWrite)
	group.POST("/files/mkdir", MonitorFilesMutate)
	group.POST("/files/rename", MonitorFilesMutate)
	group.POST("/files/delete", MonitorFilesMutate)
	group.GET("/docker/:operation", MonitorDocker)
	group.POST("/docker/:operation", MonitorDocker)
	call := func(method, operation string, params url.Values, body string, want int) *httptest.ResponseRecorder {
		t.Helper()
		if params == nil {
			params = url.Values{}
		}
		params.Set("id", strconv.FormatUint(uint64(monitor.ID), 10))
		request := httptest.NewRequest(method, "/api/v1/monitors/"+operation+"?"+params.Encode(), strings.NewReader(body))
		if strings.HasSuffix(operation, "/rename") {
			request.Header.Set("Content-Type", "application/json")
		} else {
			request.Header.Set("Content-Type", "application/octet-stream")
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != want {
			t.Fatalf("%s %s returned %d, want %d: %s", method, operation, response.Code, want, response.Body.String())
		}
		return response
	}
	fileParams := func(path string) url.Values { return url.Values{"path": {path}} }
	root := t.TempDir()
	directory := filepath.Join(root, "files with spaces;$literal")
	file := filepath.Join(directory, "hello UTF8 你好.txt")
	call(http.MethodPost, "files/mkdir", fileParams(directory), "", 200)
	call(http.MethodPost, "files/upload", fileParams(file), "hello\n", 200)
	if err := os.Chmod(file, 0640); err != nil {
		t.Fatal(err)
	}
	read := call(http.MethodGet, "files/read", fileParams(file), "", 200)
	var text struct {
		Data struct {
			Content string `json:"content"`
		} `json:"data"`
	}
	if err := json.Unmarshal(read.Body.Bytes(), &text); err != nil || text.Data.Content != "hello\n" {
		t.Fatalf("unexpected editor response: %s (%v)", read.Body.String(), err)
	}
	call(http.MethodPost, "files/save", fileParams(file), "updated 你好\n", 200)
	call(http.MethodPost, "files/save", fileParams(file), "invalid UTF-8: \xff", 400)
	call(http.MethodPost, "files/save", fileParams(file), strings.Repeat("x", (1<<20)+1), 413)
	info, err := os.Stat(file)
	if err != nil || info.Mode().Perm() != 0640 {
		t.Fatalf("atomic overwrite did not retain permissions: %v, %v", info, err)
	}
	listing := call(http.MethodGet, "files/list", fileParams(directory), "", 200)
	if !strings.Contains(listing.Body.String(), "hello UTF8") {
		t.Fatalf("listing missing uploaded file: %s", listing.Body.String())
	}
	link := filepath.Join(directory, "link")
	if err := os.Symlink(file, link); err != nil {
		t.Fatal(err)
	}
	listing = call(http.MethodGet, "files/list", fileParams(directory), "", 200)
	if !strings.Contains(listing.Body.String(), `"kind":"symlink"`) {
		t.Fatalf("symbolic link not identified: %s", listing.Body.String())
	}
	call(http.MethodGet, "files/read", fileParams(link), "", 400)
	call(http.MethodPost, "files/save", fileParams(link), "must not replace symlink", 502)
	if content, err := os.ReadFile(file); err != nil || string(content) != "updated 你好\n" {
		t.Fatalf("symlink protection changed original: %q %v", content, err)
	}
	download := call(http.MethodGet, "files/download", fileParams(file), "", 200)
	if download.Body.String() != "updated 你好\n" || download.Header().Get("Content-Type") != "application/octet-stream" || !strings.HasPrefix(download.Header().Get("Content-Disposition"), "attachment;") {
		t.Fatalf("invalid authenticated download: headers=%v body=%q", download.Header(), download.Body.String())
	}
	call(http.MethodPost, "files/delete", fileParams(directory), "", 502) // non-empty must not recurse
	conflictBody, _ := json.Marshal(map[string]string{"destination": link})
	call(http.MethodPost, "files/rename", fileParams(file), string(conflictBody), 409)
	renamed := filepath.Join(directory, "renamed.txt")
	body, _ := json.Marshal(map[string]string{"destination": renamed})
	call(http.MethodPost, "files/rename", fileParams(file), string(body), 200)
	call(http.MethodPost, "files/delete", fileParams(link), "", 200)
	call(http.MethodPost, "files/delete", fileParams(renamed), "", 200)
	call(http.MethodPost, "files/delete", fileParams(directory), "", 200)
	call(http.MethodPost, "files/delete", fileParams("/"), "", 400)
	call(http.MethodGet, "files/list", fileParams(root+"/.."), "", 400)

	for _, operation := range []string{"list", "stats", "logs", "start", "stop", "restart"} {
		method := http.MethodGet
		if operation == "start" || operation == "stop" || operation == "restart" {
			method = http.MethodPost
		}
		response := call(method, "docker/"+operation, url.Values{"containerId": {fixtureContainerID}}, "", 200)
		if operation == "logs" && !strings.Contains(response.Body.String(), "fixture ready") {
			t.Fatalf("missing bounded logs: %s", response.Body.String())
		}
	}
	call(http.MethodGet, "docker/logs", url.Values{"containerId": {fixtureContainerID + ";id"}}, "", 400)
	call(http.MethodGet, "docker/restart", nil, "", 405)
	for i := 0; i < 6; i++ {
		select {
		case command := <-commands:
			if _, status := monitorOpsDockerFixture(command); status != 0 {
				t.Fatalf("unexpected SSH exec: %q", command)
			}
		default:
			t.Fatal("missing expected Docker SSH exec")
		}
	}
	select {
	case command := <-commands:
		t.Fatalf("unsafe command reached SSH server: %q", command)
	default:
	}
}
