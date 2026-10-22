package api

import (
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"ppeelink/models"
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
			return nil, errors.New("容器数量超过 2000")
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

// dockerCommand 保留为协议校验辅助函数；实际执行已经迁移到 agent 本地。
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

func monitorProxyServer(c *gin.Context) (*models.ProxyServer, bool) {
	id, _ := strconv.Atoi(c.Query("id"))
	var monitor models.Monitor
	if err := models.DB.First(&monitor, id).Error; err != nil || monitor.ProxyServerID == nil {
		opsError(c, 404, "服务器未关联 agent")
		return nil, false
	}
	var server models.ProxyServer
	if err := models.DB.First(&server, *monitor.ProxyServerID).Error; err != nil || server.Kind != models.ServerKindAgent {
		opsError(c, 404, "服务器 agent 不存在")
		return nil, false
	}
	return &server, true
}

type agentDockerReply struct {
	Output    string `json:"output"`
	Truncated bool   `json:"truncated"`
}

func MonitorDocker(c *gin.Context) {
	operation := c.Param("operation")
	if (c.Request.Method == "GET" && operation != "list" && operation != "stats" && operation != "logs") || (c.Request.Method == "POST" && operation != "start" && operation != "stop" && operation != "restart") {
		opsError(c, 405, "不支持的操作或请求方法")
		return
	}
	containerID := c.Query("containerId")
	if operation != "list" && operation != "stats" && !validContainerID(containerID) {
		opsError(c, 400, "容器 ID 必须为 12–64 位十六进制字符")
		return
	}
	server, ok := monitorProxyServer(c)
	if !ok {
		return
	}
	raw, err := agentRequest(server.ID, "docker", gin.H{"operation": operation, "containerId": containerID}, 50*time.Second)
	if err != nil {
		opsError(c, 502, err.Error())
		return
	}
	var reply agentDockerReply
	if err := json.Unmarshal(raw, &reply); err != nil {
		opsError(c, 502, "agent 返回的 Docker 结果无法解析")
		return
	}
	if operation == "list" || operation == "stats" {
		if reply.Truncated {
			opsError(c, 502, "Docker 输出超过 2 MiB，请减少容器数量")
			return
		}
		items, err := parseDockerLines(reply.Output)
		if err != nil {
			opsError(c, 502, err.Error())
			return
		}
		opsOK(c, gin.H{"items": items})
		return
	}
	if operation == "logs" {
		opsOK(c, gin.H{"content": reply.Output, "truncated": reply.Truncated, "lines": 500})
		return
	}
	opsOK(c, gin.H{"operation": operation, "containerId": containerID})
}
