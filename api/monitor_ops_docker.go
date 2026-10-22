package api

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

var dockerContainerID = regexp.MustCompile(`^[a-fA-F0-9]{12,64}$`)

func validContainerID(id string) bool { return dockerContainerID.MatchString(id) }

// Docker's JSON-lines output is decoded, never treated as commands or HTML.
func parseDockerLines(output string) ([]map[string]any, error) {
	items := make([]map[string]any, 0)
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if len(items) >= 2000 {
			return nil, errors.New("容器数量超过 2000，请使用 SSH 管理")
		}
		var item map[string]any
		if json.Unmarshal([]byte(line), &item) != nil || item == nil {
			return nil, errors.New("Docker 返回了无法解析的 JSON，请检查 Docker 版本")
		}
		id, ok := item["ID"].(string)
		if !ok || !validContainerID(id) {
			return nil, errors.New("Docker 返回了无效的容器 ID，请检查 Docker 版本")
		}
		items = append(items, item)
	}
	return items, nil
}

func dockerCommand(operation, id string) (string, error) {
	switch operation {
	case "list":
		return "docker ps -a --no-trunc --format '{{json .}}'", nil
	case "stats":
		return "docker stats --no-stream --no-trunc --format '{{json .}}'", nil
	case "logs", "start", "stop", "restart":
		if !validContainerID(id) {
			return "", errors.New("容器 ID 必须为 12–64 位十六进制字符")
		}
		switch operation {
		case "logs":
			return "docker logs --tail 500 --timestamps " + id + " 2>&1", nil
		case "start":
			return "docker start " + id, nil
		case "stop":
			return "docker stop --time 10 " + id, nil
		case "restart":
			return "docker restart --time 10 " + id, nil
		}
	}
	return "", errors.New("不支持的 Docker 操作")
}

func MonitorDocker(c *gin.Context) {
	operation := c.Param("operation")
	if (c.Request.Method == "GET" && operation != "list" && operation != "stats" && operation != "logs") || (c.Request.Method == "POST" && operation != "start" && operation != "stop" && operation != "restart") {
		opsError(c, 405, "不支持的操作或请求方法")
		return
	}
	command, err := dockerCommand(operation, c.Query("containerId"))
	if err != nil {
		opsError(c, 400, err.Error())
		return
	}
	client, close, err := opsSSH(c)
	if err != nil {
		opsError(c, 400, err.Error())
		return
	}
	defer close()
	output, truncated, err := opsCommand(client, command)
	if err != nil {
		opsError(c, 502, err.Error())
		return
	}
	if operation == "list" || operation == "stats" {
		if truncated {
			opsError(c, 502, "Docker 输出超过 2 MiB，请减少容器数量或通过 SSH 管理")
			return
		}
		items, err := parseDockerLines(output)
		if err != nil {
			opsError(c, 502, err.Error())
			return
		}
		opsOK(c, gin.H{"items": items})
		return
	}
	if operation == "logs" {
		opsOK(c, gin.H{"content": output, "truncated": truncated, "lines": 500})
		return
	}
	opsOK(c, gin.H{"operation": operation, "containerId": c.Query("containerId")})
}
