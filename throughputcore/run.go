package throughputcore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type limitedBuffer struct {
	mu       sync.Mutex
	b        bytes.Buffer
	max      int
	overflow bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	remaining := b.max - b.b.Len()
	if len(p) > remaining {
		p = p[:remaining]
		b.overflow = true
	}
	_, _ = b.b.Write(p)
	return n, nil
}
func (b *limitedBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.b.Bytes()...)
}
func (b *limitedBuffer) Overflow() bool { b.mu.Lock(); defer b.mu.Unlock(); return b.overflow }

// Run never logs command output or credentials. A parent cancellation kills and
// reaps the child before removing its private directory.
func Run(parent context.Context, j Job, report func(Report)) {
	callback := report
	var reportMu sync.Mutex
	report = func(r Report) { reportMu.Lock(); defer reportMu.Unlock(); callback(r) }
	fail := func(e error) {
		message := e.Error()
		if j.Password != "" {
			message = strings.ReplaceAll(message, j.Password, "[redacted]")
		}
		if len(message) > 512 {
			message = message[:512]
		}
		report(Report{ID: j.ID, Role: j.Role, Status: "failed", Error: message})
	}
	if e := Validate(j); e != nil {
		fail(e)
		return
	}
	c := Inspect(parent)
	if !c.AuthSupported {
		fail(errors.New(c.Error))
		return
	}
	if j.Direction == "bidir" && !c.BidirSupported {
		fail(errors.New("iperf3 does not support simultaneous bidirectional tests"))
		return
	}
	f, e := NewAuthFiles(j)
	if e != nil {
		fail(e)
		return
	}
	defer f.Close()
	ctx, cancel := jobContext(parent, j)
	defer cancel()
	var args []string
	if j.Role == "server" {
		args, e = ServerArgs(j, f)
	} else {
		args, e = ClientArgs(j, f.Key)
	}
	if e != nil {
		fail(e)
		return
	}

	var out interface {
		io.Writer
		Bytes() []byte
		Overflow() bool
	} = &limitedBuffer{max: 2 * 1024 * 1024}
	if j.Role == "client" && c.JSONStreamSupported {
		args = append(args, "--json-stream")
		out = newJSONStream(j, report)
	}
	cmd := exec.CommandContext(ctx, c.Path, args...)
	configureCommand(cmd)
	stderr := &limitedBuffer{max: 16 * 1024}
	cmd.Stdout = out
	cmd.Stderr = stderr
	for _, v := range os.Environ() {
		if !strings.HasPrefix(v, "IPERF3_PASSWORD=") {
			cmd.Env = append(cmd.Env, v)
		}
	}
	if j.Role == "client" {
		cmd.Env = append(cmd.Env, "IPERF3_PASSWORD="+j.Password)
	}
	if e = cmd.Start(); e != nil {
		fail(fmt.Errorf("start iperf3: %w", e))
		return
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	if j.Role == "server" {
		ready := time.NewTimer(350 * time.Millisecond)
		defer ready.Stop()
		select {
		case e = <-done:
			fail(fmt.Errorf("temporary listener failed to start: %v", e))
			return
		case <-ctx.Done():
			<-done
			fail(ctx.Err())
			return
		case <-ready.C:
		}
		// A control probe would consume iperf3's one-off server. Validate ownership
		// of the listener through the OS instead of opening a client connection.
		if !listenerReady(cmd.Process.Pid, j.Port) {
			cancel()
			<-done
			fail(errors.New("temporary listener did not bind the requested port"))
			return
		}
		report(Report{ID: j.ID, Role: j.Role, Status: "ready"})
	} else {
		report(Report{ID: j.ID, Role: j.Role, Status: "running"})
	}
	select {
	case e = <-done:
	case <-ctx.Done():
		e = ctx.Err()
		<-done
	}
	if ctx.Err() != nil {
		fail(ctx.Err())
		return
	}
	if out.Overflow() {
		fail(errors.New("iperf3 output exceeded safe limit"))
		return
	}
	if e != nil { // Only parse structured error; arbitrary output can contain sensitive values.
		var parsed iperfJSON
		if unmarshalErr := decodeOutput(out.Bytes(), &parsed); unmarshalErr == nil && parsed.Error != "" {
			fail(fmt.Errorf("iperf3: %s", parsed.Error))
		} else {
			fail(fmt.Errorf("iperf3 execution failed: %w", e))
		}
		return
	}
	if j.Role == "server" {
		report(Report{ID: j.ID, Role: j.Role, Status: "completed"})
		return
	}
	r, e := ParseResult(out.Bytes(), j)
	if e != nil {
		fail(e)
		return
	}
	report(Report{ID: j.ID, Role: j.Role, Status: "completed", Result: &r})
}

var _ io.Writer = (*limitedBuffer)(nil)
