package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Xioaruan912/PPEELink/terminalcore"
	"time"
)

type terminalRequest struct {
	Operation string `json:"operation"`
	Owner     int    `json:"owner"`
	SessionID string `json:"sessionId"`
	Name      string `json:"name"`
}

func handleTerminalOp(parent context.Context, payload json.RawMessage) (json.RawMessage, error) {
	var req terminalRequest
	if err := json.Unmarshal(payload, &req); err != nil || req.Owner <= 0 {
		return nil, errors.New("无效的会话请求")
	}
	timeout := 8 * time.Second
	if req.Operation == "prepare" {
		timeout = 4 * time.Minute
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	switch req.Operation {
	case "list":
		sessions, err := terminalcore.Default.List(ctx, req.Owner)
		if err != nil {
			return nil, err
		}
		return json.Marshal(sessions)
	case "create":
		s, err := terminalcore.Default.Create(ctx, req.Owner, req.Name)
		if err != nil {
			return nil, err
		}
		return json.Marshal(s)
	case "terminate":
		if err := terminalcore.Default.Terminate(ctx, req.Owner, req.SessionID); err != nil {
			return nil, err
		}
		return json.Marshal(map[string]bool{"ended": true})
	case "check":
		if err := terminalcore.Default.Exists(ctx, req.Owner, req.SessionID); err != nil {
			return nil, err
		}
		return json.Marshal(map[string]bool{"exists": true})
	case "prepare":
		if err := terminalcore.Default.Prepare(ctx, true); err != nil {
			return nil, err
		}
		return json.Marshal(map[string]bool{"ready": true})
	default:
		return nil, errors.New("不支持的会话操作")
	}
}
