package api

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// agentWireMessage is the small JSON protocol shared by the agent and panel.
// Terminal data is base64 so arbitrary PTY bytes survive JSON.
type agentWireMessage struct {
	Type    string          `json:"type"`
	ID      string          `json:"id,omitempty"`
	Data    string          `json:"data,omitempty"`
	Cols    int             `json:"cols,omitempty"`
	Rows    int             `json:"rows,omitempty"`
	Op      string          `json:"op,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
	OK      bool            `json:"ok,omitempty"`
	Error   string          `json:"error,omitempty"`
	Code    int             `json:"code,omitempty"`
}

type agentReply struct {
	OK    bool
	Data  json.RawMessage
	Error string
}

type agentTerminal struct {
	out    func([]byte)
	closed func()
}

type agentConn struct {
	serverID uint
	conn     *websocket.Conn
	send     chan []byte
	done     chan struct{}
	closeOne sync.Once
	mu       sync.Mutex
	terms    map[string]*agentTerminal
	pending  map[string]chan agentReply
}

var (
	agentConnMu sync.RWMutex
	agentConns  = map[uint]*agentConn{}
)

var agentWSUpgrader = websocket.Upgrader{
	ReadBufferSize:  32 * 1024,
	WriteBufferSize: 32 * 1024,
	CheckOrigin:     func(*http.Request) bool { return true },
}

func newAgentConn(serverID uint, conn *websocket.Conn) *agentConn {
	return &agentConn{
		serverID: serverID, conn: conn, send: make(chan []byte, 128), done: make(chan struct{}),
		terms: map[string]*agentTerminal{}, pending: map[string]chan agentReply{},
	}
}

func (ac *agentConn) writeLoop() {
	for {
		select {
		case raw := <-ac.send:
			if err := ac.conn.WriteMessage(websocket.TextMessage, raw); err != nil {
				ac.shutdown()
				return
			}
		case <-ac.done:
			return
		}
	}
}

func (ac *agentConn) sendMessage(msg agentWireMessage) bool {
	raw, err := json.Marshal(msg)
	if err != nil {
		return false
	}
	select {
	case ac.send <- raw:
		return true
	case <-ac.done:
		return false
	default:
		ac.shutdown()
		return false
	}
}

func (ac *agentConn) readLoop() {
	defer ac.shutdown()
	for {
		_, raw, err := ac.conn.ReadMessage()
		if err != nil {
			return
		}
		var msg agentWireMessage
		if json.Unmarshal(raw, &msg) != nil {
			continue
		}
		switch msg.Type {
		case "term.output":
			data, err := decodeBase64(msg.Data)
			if err != nil {
				continue
			}
			ac.mu.Lock()
			session := ac.terms[msg.ID]
			ac.mu.Unlock()
			if session != nil {
				session.out(data)
			}
		case "term.exit":
			ac.mu.Lock()
			session := ac.terms[msg.ID]
			delete(ac.terms, msg.ID)
			ac.mu.Unlock()
			if session != nil {
				session.closed()
			}
		case "op.result":
			ac.mu.Lock()
			reply := ac.pending[msg.ID]
			delete(ac.pending, msg.ID)
			ac.mu.Unlock()
			if reply != nil {
				reply <- agentReply{OK: msg.OK, Data: msg.Payload, Error: msg.Error}
			}
		}
	}
}

func (ac *agentConn) shutdown() {
	ac.closeOne.Do(func() {
		close(ac.done)
		_ = ac.conn.Close()
		ac.mu.Lock()
		for id, session := range ac.terms {
			delete(ac.terms, id)
			if session.closed != nil {
				session.closed()
			}
		}
		for id, reply := range ac.pending {
			delete(ac.pending, id)
			reply <- agentReply{Error: "agent 已断开"}
		}
		ac.mu.Unlock()
		agentConnMu.Lock()
		if agentConns[ac.serverID] == ac {
			delete(agentConns, ac.serverID)
		}
		agentConnMu.Unlock()
	})
}

func agentConnForServer(serverID uint) *agentConn {
	agentConnMu.RLock()
	defer agentConnMu.RUnlock()
	return agentConns[serverID]
}

func (ac *agentConn) openTerminal(id string, cols, rows int, out func([]byte), closed func()) bool {
	ac.mu.Lock()
	ac.terms[id] = &agentTerminal{out: out, closed: closed}
	ac.mu.Unlock()
	return ac.sendMessage(agentWireMessage{Type: "term.open", ID: id, Cols: cols, Rows: rows})
}

func (ac *agentConn) closeTerminal(id string) {
	ac.mu.Lock()
	delete(ac.terms, id)
	ac.mu.Unlock()
	ac.sendMessage(agentWireMessage{Type: "term.close", ID: id})
}

func agentRequest(serverID uint, op string, payload any, timeout time.Duration) (json.RawMessage, error) {
	ac := agentConnForServer(serverID)
	if ac == nil {
		return nil, errAgentOffline
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	id := randomWireID()
	ch := make(chan agentReply, 1)
	ac.mu.Lock()
	ac.pending[id] = ch
	ac.mu.Unlock()
	defer func() {
		ac.mu.Lock()
		delete(ac.pending, id)
		ac.mu.Unlock()
	}()
	if !ac.sendMessage(agentWireMessage{Type: "op", ID: id, Op: op, Payload: raw}) {
		return nil, errAgentOffline
	}
	select {
	case reply := <-ch:
		if reply.Error != "" {
			return nil, errAgentReply(reply.Error)
		}
		return reply.Data, nil
	case <-time.After(timeout):
		return nil, errAgentTimeout
	}
}

func randomWireID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// AgentWebSocket 是 agent 主动连回面板的长期通道，终端和运维操作都复用它。
func AgentWebSocket(c *gin.Context) {
	server, ok := agentServer(c)
	if !ok {
		return
	}
	conn, err := agentWSUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	ac := newAgentConn(server.ID, conn)
	agentConnMu.Lock()
	if old := agentConns[server.ID]; old != nil {
		old.shutdown()
	}
	agentConns[server.ID] = ac
	agentConnMu.Unlock()
	go ac.writeLoop()
	ac.readLoop()
}

func decodeBase64(value string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(value)
}

func encodeBase64(value []byte) string {
	return base64.StdEncoding.EncodeToString(value)
}

type agentError string

func (e agentError) Error() string { return string(e) }

const (
	errAgentOffline = agentError("agent 未连接，请等待心跳恢复")
	errAgentTimeout = agentError("agent 操作超时")
)

func errAgentReply(value string) error { return agentError(value) }
