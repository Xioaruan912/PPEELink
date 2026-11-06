package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
	"github.com/Xioaruan912/PPEELink/terminalcore"
)

type agentTermMessage struct {
	Type       string          `json:"type"`
	ID         string          `json:"id,omitempty"`
	Data       string          `json:"data,omitempty"`
	Cols       int             `json:"cols,omitempty"`
	Rows       int             `json:"rows,omitempty"`
	Op         string          `json:"op,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
	OK         bool            `json:"ok,omitempty"`
	Error      string          `json:"error,omitempty"`
	Code       int             `json:"code,omitempty"`
	Owner      int             `json:"owner,omitempty"`
	SessionID  string          `json:"sessionId,omitempty"`
	Persistent bool            `json:"persistent,omitempty"`
}

type agentPTY struct {
	file     *os.File
	cmd      *exec.Cmd
	input    chan []byte
	done     chan struct{}
	closeOne sync.Once
}

func (s *agentPTY) stop() {
	s.closeOne.Do(func() {
		if s.done != nil {
			close(s.done)
		}
		_ = s.cmd.Process.Kill()
		_ = s.file.Close()
	})
}
func (s *agentPTY) readInput() {
	for {
		select {
		case data := <-s.input:
			_ = s.file.SetWriteDeadline(time.Now().Add(2 * time.Second))
			if _, err := s.file.Write(data); err != nil {
				s.stop()
				return
			}
		case <-s.done:
			return
		}
	}
}

// runAgentTerminal 维持 agent 到面板的反向 WebSocket；终端完全在 agent 本地执行。
func runAgentTerminal(ctx context.Context, panel, token string) {
	go func() {
		prepareCtx, cancel := context.WithTimeout(ctx, 4*time.Minute)
		defer cancel()
		if err := terminalcore.Default.Prepare(prepareCtx, true); err != nil {
			log.Printf("[terminal] 任务保护未就绪: %v", err)
		}
	}()
	url := strings.TrimRight(panel, "/")
	if strings.HasPrefix(url, "https://") {
		url = "wss://" + strings.TrimPrefix(url, "https://")
	} else {
		url = "ws://" + strings.TrimPrefix(url, "http://")
	}
	url += "/api/v1/agent/ws"
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		header := http.Header{}
		header.Set("Authorization", "Agent "+token)
		conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, header)
		if err != nil {
			if !sleepCtx(ctx, 5*time.Second) {
				return
			}
			continue
		}
		if err := serveAgentTerminal(ctx, conn); err != nil && ctx.Err() == nil {
			log.Printf("[agent] 终端通道断开，将重连: %v", err)
		}
		_ = conn.Close()
		if !sleepCtx(ctx, 3*time.Second) {
			return
		}
	}
}

func serveAgentTerminal(ctx context.Context, conn *websocket.Conn) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stopClose := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stopClose()
	conn.SetReadLimit(1 << 20)
	var writeMu sync.Mutex
	var sessionsMu sync.Mutex
	sessions := map[string]*agentPTY{}
	write := func(msg agentTermMessage) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		return conn.WriteJSON(msg)
	}
	if err := write(agentTermMessage{Type: "hello", Persistent: true}); err != nil {
		return err
	}
	defer func() {
		sessionsMu.Lock()
		for _, session := range sessions {
			session.stop()
		}
		sessionsMu.Unlock()
	}()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		var msg agentTermMessage
		if err := conn.ReadJSON(&msg); err != nil {
			return err
		}
		switch msg.Type {
		case "term.open":
			var session *agentPTY
			var err error
			if msg.SessionID != "" || msg.Owner != 0 {
				var attached *terminalcore.Attachment
				attached, err = terminalcore.Default.Attach(ctx, msg.Owner, msg.SessionID, msg.Cols, msg.Rows)
				if err == nil {
					session = &agentPTY{file: attached.File, cmd: attached.Cmd}
				}
			} else {
				session, err = startAgentPTY(msg.Cols, msg.Rows)
			}
			if err != nil {
				_ = write(agentTermMessage{Type: "term.exit", ID: msg.ID, Code: 1, Error: err.Error()})
				continue
			}
			session.input = make(chan []byte, 16)
			session.done = make(chan struct{})
			go session.readInput()
			sessionsMu.Lock()
			if old := sessions[msg.ID]; old != nil {
				old.stop()
			}
			sessions[msg.ID] = session
			sessionsMu.Unlock()
			if msg.SessionID != "" {
				_ = write(agentTermMessage{Type: "term.ready", ID: msg.ID, SessionID: msg.SessionID, Persistent: true})
			}
			go func(id, persistentID string, owner int, s *agentPTY) {
				defer s.file.Close()
				buf := make([]byte, 32*1024)
				for {
					n, err := s.file.Read(buf)
					if n > 0 {
						if write(agentTermMessage{Type: "term.output", ID: id, Data: base64.StdEncoding.EncodeToString(buf[:n])}) != nil {
							s.stop()
							break
						}
					}
					if err != nil {
						break
					}
				}
				// Reap the shell before reporting exit. Without Wait every
				// session leaks a zombie and reports a fabricated zero code.
				_ = s.cmd.Wait()
				code := 0
				if s.cmd.ProcessState != nil {
					code = s.cmd.ProcessState.ExitCode()
				}
				kind := "term.exit"
				if persistentID != "" && terminalcore.Default.Exists(ctx, owner, persistentID) == nil {
					kind = "term.detached"
				}
				s.stop()
				_ = write(agentTermMessage{Type: kind, ID: id, Code: code})
				sessionsMu.Lock()
				if sessions[id] == s {
					delete(sessions, id)
				}
				sessionsMu.Unlock()
			}(msg.ID, msg.SessionID, msg.Owner, session)
		case "term.input":
			sessionsMu.Lock()
			session := sessions[msg.ID]
			sessionsMu.Unlock()
			if session != nil {
				data, err := base64.StdEncoding.DecodeString(msg.Data)
				if err == nil && len(data) <= 8192 {
					select {
					case session.input <- data:
					case <-session.done:
					default:
						session.stop()
					}
				}
			}
		case "term.resize":
			sessionsMu.Lock()
			session := sessions[msg.ID]
			sessionsMu.Unlock()
			if session != nil && msg.Cols > 0 && msg.Rows > 0 {
				_ = pty.Setsize(session.file, &pty.Winsize{Cols: uint16(msg.Cols), Rows: uint16(msg.Rows)})
			}
		case "term.close":
			sessionsMu.Lock()
			session := sessions[msg.ID]
			delete(sessions, msg.ID)
			sessionsMu.Unlock()
			if session != nil {
				session.stop()
			}
		case "op":
			go func(id string, op string, payload json.RawMessage) {
				raw, err := handleAgentOp(ctx, op, payload)
				reply := agentTermMessage{Type: "op.result", ID: id, OK: err == nil, Payload: raw}
				if err != nil {
					reply.Error = err.Error()
				}
				_ = write(reply)
			}(msg.ID, msg.Op, msg.Payload)
		}
	}
}

func startAgentPTY(cols, rows int) (*agentPTY, error) {
	shell := "/bin/bash"
	if _, err := os.Stat(shell); err != nil {
		shell = "/bin/sh"
	}
	cmd := exec.Command(shell)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor")
	if _, err := os.Stat("/root"); err == nil {
		cmd.Dir = "/root"
	}
	if cols <= 0 {
		cols = 100
	}
	if rows <= 0 {
		rows = 30
	}
	file, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	if err != nil {
		return nil, err
	}
	return &agentPTY{file: file, cmd: cmd}, nil
}

var agentContainerID = regexp.MustCompile(`^[a-fA-F0-9]{12,64}$`)

type agentDockerRequest struct {
	Operation   string `json:"operation"`
	ContainerID string `json:"containerId"`
}

func handleAgentOp(ctx context.Context, op string, payload json.RawMessage) (json.RawMessage, error) {
	switch op {
	case "terminal":
		return handleTerminalOp(ctx, payload)
	case "docker":
		var req agentDockerRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			return nil, errors.New("Docker 请求格式无效")
		}
		output, truncated, err := runAgentDocker(ctx, req)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{"output": output, "truncated": truncated})
	case "files":
		var req agentFileRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			return nil, errors.New("文件请求格式无效")
		}
		return handleAgentFiles(req)
	default:
		return nil, errors.New("不支持的 agent 操作")
	}
}

func runAgentDocker(parent context.Context, req agentDockerRequest) (string, bool, error) {
	var args []string
	switch req.Operation {
	case "list":
		args = []string{"ps", "-a", "--no-trunc", "--format", "{{json .}}"}
	case "stats":
		args = []string{"stats", "--no-stream", "--no-trunc", "--format", "{{json .}}"}
	case "logs", "start", "stop", "restart":
		if !agentContainerID.MatchString(req.ContainerID) {
			return "", false, errors.New("容器 ID 必须为 12–64 位十六进制字符")
		}
		switch req.Operation {
		case "logs":
			args = []string{"logs", "--tail", "500", "--timestamps", req.ContainerID}
		case "start":
			args = []string{"start", req.ContainerID}
		case "stop":
			args = []string{"stop", "--time", "10", req.ContainerID}
		case "restart":
			args = []string{"restart", "--time", "10", req.ContainerID}
		}
	default:
		return "", false, errors.New("不支持的 Docker 操作")
	}
	ctx, cancel := context.WithTimeout(parent, 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", args...)
	var output bytes.Buffer
	cmd.Stdout = &cappedAgentBuffer{buf: &output, limit: 2 << 20}
	cmd.Stderr = cmd.Stdout
	err := cmd.Run()
	if ctx.Err() != nil {
		return output.String(), false, errors.New("Docker 操作超时")
	}
	if err != nil {
		return output.String(), false, errors.New("Docker 操作失败：请确认 Docker 已安装、服务运行且 agent 用户有权限")
	}
	return output.String(), output.Len() >= 2<<20, nil
}

type cappedAgentBuffer struct {
	buf   *bytes.Buffer
	limit int
}

func (b *cappedAgentBuffer) Write(p []byte) (int, error) {
	remaining := b.limit - b.buf.Len()
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		_, _ = b.buf.Write(p)
	}
	return len(p), nil
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
