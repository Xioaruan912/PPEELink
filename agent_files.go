package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const agentFileLimit = 64 << 20

type agentFileRequest struct {
	Operation   string `json:"operation"`
	Path        string `json:"path"`
	Destination string `json:"destination"`
	Data        string `json:"data"`
}

type agentFileItem struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Mode     string `json:"mode"`
	Modified int64  `json:"modified"`
	Kind     string `json:"kind"`
	Target   string `json:"target,omitempty"`
}

func cleanAgentPath(value string, mutation bool) (string, error) {
	if value == "" && !mutation {
		value = "/"
	}
	if !strings.HasPrefix(value, "/") || len(value) > 4096 {
		return "", errors.New("路径必须是绝对路径")
	}
	for _, part := range strings.Split(value, "/") {
		if part == ".." {
			return "", errors.New("路径不能包含 ..")
		}
	}
	clean := path.Clean(value)
	if mutation && clean == "/" {
		return "", errors.New("不允许修改根目录")
	}
	return clean, nil
}

func handleAgentFiles(req agentFileRequest) (json.RawMessage, error) {
	switch req.Operation {
	case "list":
		return agentFileList(req.Path)
	case "read":
		return agentFileRead(req.Path)
	case "write":
		return agentFileWrite(req.Path, req.Data)
	case "mkdir", "rename", "delete":
		return agentFileMutate(req)
	default:
		return nil, errors.New("不支持的文件操作")
	}
}

func agentFileList(raw string) (json.RawMessage, error) {
	p, err := cleanAgentPath(raw, false)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(p)
	if err != nil {
		return nil, fileAgentError(err)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return entries[i].Name() < entries[j].Name()
	})
	truncated := len(entries) > 2000
	if truncated {
		entries = entries[:2000]
	}
	items := make([]agentFileItem, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		full := filepath.Join(p, name)
		info, err := os.Lstat(full)
		if err != nil {
			continue
		}
		kind, target := "file", ""
		if info.IsDir() {
			kind = "directory"
		} else if info.Mode()&os.ModeSymlink != 0 {
			kind = "symlink"
			target, _ = os.Readlink(full)
		} else if !info.Mode().IsRegular() {
			kind = "special"
		}
		items = append(items, agentFileItem{Name: name, Path: full, Size: info.Size(), Mode: info.Mode().String(), Modified: info.ModTime().Unix(), Kind: kind, Target: target})
	}
	return json.Marshal(map[string]any{"path": p, "parent": path.Dir(p), "items": items, "truncated": truncated})
}

func agentFileRead(raw string) (json.RawMessage, error) {
	p, err := cleanAgentPath(raw, false)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(p)
	if err != nil {
		return nil, fileAgentError(err)
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("仅支持普通文件，不跟随符号链接")
	}
	if info.Size() > agentFileLimit {
		return nil, errors.New("文件超过 64 MiB 限制")
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, fileAgentError(err)
	}
	return json.Marshal(map[string]any{"path": p, "data": base64.StdEncoding.EncodeToString(data), "size": len(data)})
}

func agentFileWrite(rawPath, encoded string) (json.RawMessage, error) {
	p, err := cleanAgentPath(rawPath, true)
	if err != nil {
		return nil, err
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errors.New("文件内容编码无效")
	}
	if len(data) > agentFileLimit {
		return nil, errors.New("文件超过 64 MiB 限制")
	}
	mode := os.FileMode(0644)
	if info, err := os.Lstat(p); err == nil {
		if !info.Mode().IsRegular() {
			return nil, errors.New("目标不是普通文件，不允许覆盖")
		}
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return nil, fileAgentError(err)
	}
	tmp := filepath.Join(filepath.Dir(p), ".ppeelink-"+time.Now().Format("20060102150405.000000000")+".tmp")
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return nil, fileAgentError(err)
	}
	if err := os.Chmod(tmp, mode); err != nil {
		_ = os.Remove(tmp)
		return nil, fileAgentError(err)
	}
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		return nil, fileAgentError(err)
	}
	return json.Marshal(map[string]any{"path": p, "size": len(data)})
}

func agentFileMutate(req agentFileRequest) (json.RawMessage, error) {
	p, err := cleanAgentPath(req.Path, true)
	if err != nil {
		return nil, err
	}
	switch req.Operation {
	case "mkdir":
		if err := os.Mkdir(p, 0755); err != nil {
			return nil, fileAgentError(err)
		}
	case "rename":
		destination, err := cleanAgentPath(req.Destination, true)
		if err != nil {
			return nil, err
		}
		if _, err := os.Lstat(destination); err == nil {
			return nil, errors.New("目标已存在，不允许覆盖")
		} else if !os.IsNotExist(err) {
			return nil, fileAgentError(err)
		}
		if err := os.Rename(p, destination); err != nil {
			return nil, fileAgentError(err)
		}
	case "delete":
		info, err := os.Lstat(p)
		if err != nil {
			return nil, fileAgentError(err)
		}
		if info.IsDir() {
			err = os.Remove(p)
		} else if info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			err = os.Remove(p)
		} else {
			return nil, errors.New("不允许删除特殊设备文件")
		}
		if err != nil {
			return nil, fileAgentError(err)
		}
	}
	return json.Marshal(map[string]any{"path": p})
}

func fileAgentError(err error) error {
	if os.IsNotExist(err) {
		return errors.New("文件或目录不存在")
	}
	if os.IsPermission(err) {
		return errors.New("agent 没有访问该路径的权限")
	}
	if errors.Is(err, os.ErrExist) {
		return errors.New("目标已存在")
	}
	return errors.New("文件操作失败：" + err.Error())
}
