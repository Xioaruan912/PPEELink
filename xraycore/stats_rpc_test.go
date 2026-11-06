package xraycore

import (
	"context"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func statsTestRunner(t *testing.T, port int) *Runner {
	t.Helper()
	// Stub process liveness only; collection uses a real loopback gRPC connection.
	r := &Runner{apiPort: port, cmd: &exec.Cmd{}, exited: make(chan struct{})}
	t.Cleanup(func() { r.closeStatsLocked() })
	return r
}

func statsTestServer(t *testing.T, calls *atomic.Int32, loseResponse bool) *grpc.Server {
	t.Helper()
	return grpc.NewServer(grpc.UnknownServiceHandler(func(_ any, stream grpc.ServerStream) error {
		method, _ := grpc.MethodFromServerStream(stream)
		if strings.HasSuffix(method, "/QueryStats") {
			req, _ := statsMessage("QueryStatsRequest")
			if err := stream.RecvMsg(req); err != nil {
				return err
			}
			if !req.Get(statsField(req, "reset")).Bool() {
				return status.Error(codes.InvalidArgument, "reset was not enabled")
			}
			calls.Add(1)
			if loseResponse {
				return status.Error(codes.Unavailable, "response lost after reset")
			}
			reply, _ := statsMessage("QueryStatsResponse")
			stat, _ := statsMessage("Stat")
			stat.Set(statsField(stat, "name"), protoreflect.ValueOfString("user>>>u7>>>traffic>>>uplink"))
			stat.Set(statsField(stat, "value"), protoreflect.ValueOfInt64(123))
			reply.Mutable(statsField(reply, "stat")).List().Append(protoreflect.ValueOfMessage(stat))
			return stream.SendMsg(reply)
		}
		if strings.HasSuffix(method, "/GetAllOnlineUsers") {
			req, _ := statsMessage("GetAllOnlineUsersRequest")
			if err := stream.RecvMsg(req); err != nil {
				return err
			}
			reply, _ := statsMessage("GetAllOnlineUsersResponse")
			return stream.SendMsg(reply)
		}
		return status.Error(codes.Unimplemented, "unexpected method")
	}))
}

func TestCollectWaitsForStatsListenerAfterXrayStarts(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	r := statsTestRunner(t, port)
	conn, err := r.statsConnectionLocked(port)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn.Connect()
	for conn.GetState() != connectivity.TransientFailure {
		s := conn.GetState()
		if s == connectivity.TransientFailure {
			break
		}
		if !conn.WaitForStateChange(ctx, s) {
			t.Fatal("did not reproduce initial connection refusal")
		}
	}
	var calls atomic.Int32
	server := statsTestServer(t, &calls, false)
	defer server.Stop()
	started := make(chan error, 1)
	timer := time.AfterFunc(100*time.Millisecond, func() {
		listener, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
		if err != nil {
			started <- err
			return
		}
		started <- nil
		_ = server.Serve(listener)
	})
	defer timer.Stop()
	traffic, _, err := r.Collect(ctx)
	if err != nil {
		t.Fatalf("temporary startup refusal must not fail collection: %v", err)
	}
	if err := <-started; err != nil {
		t.Fatal(err)
	}
	if traffic["u7"].Up != 123 || calls.Load() != 1 {
		t.Fatal("counter was lost or reset more than once")
	}
}

func TestCollectDoesNotReplayAResetAfterLostResponse(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	server := statsTestServer(t, &calls, true)
	defer server.Stop()
	go server.Serve(l)
	r := statsTestRunner(t, l.Addr().(*net.TCPAddr).Port)
	_, _, err = r.Collect(context.Background())
	if err == nil || !strings.Contains(err.Error(), "response lost after reset") {
		t.Fatalf("expected lost-response failure, got %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("destructive reset was replayed %d times", calls.Load())
	}
}

func TestCollectReadinessWaitHonorsCancellation(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	r := statsTestRunner(t, port)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err = r.Collect(ctx)
	if err == nil || status.Code(err) != codes.Canceled {
		t.Fatalf("collection did not honor cancellation: %v", err)
	}
}
