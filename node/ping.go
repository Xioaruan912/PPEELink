package node

import (
	"net"
	"time"
)

// TCPPing 对指定地址做 TCP 连接测速（DialTimeout），返回毫秒级延迟；失败返回 -1。
func TCPPing(addr string, timeout time.Duration) int {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return -1
	}
	conn.Close()
	ms := time.Since(start).Milliseconds()
	if ms < 1 {
		ms = 1
	}
	return int(ms)
}
