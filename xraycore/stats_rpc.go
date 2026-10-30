package xraycore

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

// The small wire schema follows Xray v26.3.27 app/stats/command/command.proto.
// Keep this read-only API independent of Xray's server implementation/dependencies.
var statsSchema = sync.OnceValues(func() (protoreflect.FileDescriptor, error) {
	field := func(name string, number int32, kind descriptorpb.FieldDescriptorProto_Type, repeated bool, typeName string) *descriptorpb.FieldDescriptorProto {
		label := descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL
		if repeated {
			label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED
		}
		f := &descriptorpb.FieldDescriptorProto{Name: proto.String(name), Number: proto.Int32(number), Type: &kind, Label: &label}
		if typeName != "" {
			f.TypeName = proto.String(typeName)
		}
		return f
	}
	str, boolean, integer, message := descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_TYPE_BOOL, descriptorpb.FieldDescriptorProto_TYPE_INT64, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE
	return protodesc.NewFile(&descriptorpb.FileDescriptorProto{
		Name: proto.String("ppeelink/stats.proto"), Package: proto.String("xray.app.stats.command"), Syntax: proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("QueryStatsRequest"), Field: []*descriptorpb.FieldDescriptorProto{field("pattern", 1, str, false, ""), field("reset", 2, boolean, false, "")}},
			{Name: proto.String("Stat"), Field: []*descriptorpb.FieldDescriptorProto{field("name", 1, str, false, ""), field("value", 2, integer, false, "")}},
			{Name: proto.String("QueryStatsResponse"), Field: []*descriptorpb.FieldDescriptorProto{field("stat", 1, message, true, ".xray.app.stats.command.Stat")}},
			{Name: proto.String("GetStatsRequest"), Field: []*descriptorpb.FieldDescriptorProto{field("name", 1, str, false, ""), field("reset", 2, boolean, false, "")}},
			{Name: proto.String("GetAllOnlineUsersRequest")},
			{Name: proto.String("GetAllOnlineUsersResponse"), Field: []*descriptorpb.FieldDescriptorProto{field("users", 1, str, true, "")}},
			{Name: proto.String("GetStatsOnlineIpListResponse"), Field: []*descriptorpb.FieldDescriptorProto{field("name", 1, str, false, ""), field("ips", 2, message, true, ".xray.app.stats.command.GetStatsOnlineIpListResponse.IpsEntry")},
				NestedType: []*descriptorpb.DescriptorProto{{Name: proto.String("IpsEntry"), Options: &descriptorpb.MessageOptions{MapEntry: proto.Bool(true)}, Field: []*descriptorpb.FieldDescriptorProto{field("key", 1, str, false, ""), field("value", 2, integer, false, "")}}}},
		},
	}, nil)
})

func statsMessage(name string) (*dynamicpb.Message, error) {
	schema, err := statsSchema()
	if err != nil {
		return nil, err
	}
	return dynamicpb.NewMessage(schema.Messages().ByName(protoreflect.Name(name))), nil
}
func statsField(m *dynamicpb.Message, name string) protoreflect.FieldDescriptor {
	return m.Descriptor().Fields().ByName(protoreflect.Name(name))
}
func statsInvoke(ctx context.Context, conn *grpc.ClientConn, method string, request *dynamicpb.Message, response string) (*dynamicpb.Message, error) {
	reply, err := statsMessage(response)
	if err != nil {
		return nil, err
	}
	callCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	err = conn.Invoke(callCtx, "/xray.app.stats.command.StatsService/"+method, request, reply)
	return reply, err
}
func (r *Runner) closeStatsLocked() {
	if r.statsConn != nil {
		_ = r.statsConn.Close()
		r.statsConn = nil
	}
	r.statsPort = 0
}
func (r *Runner) statsConnectionLocked(port int) (*grpc.ClientConn, error) {
	if r.statsConn != nil && r.statsPort == port {
		return r.statsConn, nil
	}
	r.closeStatsLocked()
	conn, err := grpc.NewClient("passthrough:///127.0.0.1:"+strconv.Itoa(port), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDisableRetry())
	if err != nil {
		return nil, err
	}
	r.statsConn, r.statsPort = conn, port
	return conn, nil
}

// Collect serializes destructive counter reads. Online lookups share the same
// loopback connection with at most four concurrent calls and a bounded budget.
func (r *Runner) Collect(ctx context.Context) (map[string]Counter, map[string]int, error) {
	r.collectMu.Lock()
	defer r.collectMu.Unlock()
	traffic, online := map[string]Counter{}, map[string]int{}
	r.mu.Lock()
	port, running := r.apiPort, r.runningLocked()
	var conn *grpc.ClientConn
	var err error
	if port != 0 && running {
		conn, err = r.statsConnectionLocked(port)
	}
	r.mu.Unlock()
	if err != nil {
		return traffic, online, err
	}
	if conn == nil {
		return traffic, online, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := statsMessage("QueryStatsRequest")
	if err != nil {
		return traffic, online, err
	}
	req.Set(statsField(req, "pattern"), protoreflect.ValueOfString("user>>>"))
	req.Set(statsField(req, "reset"), protoreflect.ValueOfBool(true))
	// Never retry reset calls: the server may have consumed counters before a lost response.
	reply, err := statsInvoke(ctx, conn, "QueryStats", req, "QueryStatsResponse")
	if err != nil {
		return traffic, online, fmt.Errorf("读取流量失败: %w", err)
	}
	stats := reply.Get(statsField(reply, "stat")).List()
	for i := 0; i < stats.Len(); i++ {
		s := stats.Get(i).Message()
		fields := s.Descriptor().Fields()
		parts := strings.Split(s.Get(fields.ByName("name")).String(), ">>>")
		value := s.Get(fields.ByName("value")).Int()
		if len(parts) != 4 || parts[2] != "traffic" || value < 0 {
			continue
		}
		c := traffic[parts[1]]
		switch parts[3] {
		case "uplink":
			c.Up += value
		case "downlink":
			c.Down += value
		default:
			continue
		}
		traffic[parts[1]] = c
	}
	empty, _ := statsMessage("GetAllOnlineUsersRequest")
	users, err := statsInvoke(ctx, conn, "GetAllOnlineUsers", empty, "GetAllOnlineUsersResponse")
	if err != nil {
		return traffic, online, nil
	}
	list := users.Get(statsField(users, "users")).List()
	emails := make(chan string)
	var workers sync.WaitGroup
	var resultMu sync.Mutex
	for i := 0; i < 4; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for email := range emails {
				if ctx.Err() != nil {
					continue
				}
				request, _ := statsMessage("GetStatsRequest")
				request.Set(statsField(request, "name"), protoreflect.ValueOfString("user>>>"+email+">>>online"))
				ips, e := statsInvoke(ctx, conn, "GetStatsOnlineIpList", request, "GetStatsOnlineIpListResponse")
				if e == nil {
					resultMu.Lock()
					online[email] = ips.Get(statsField(ips, "ips")).Map().Len()
					resultMu.Unlock()
				}
			}
		}()
	}
	seen := map[string]bool{}
sendLoop:
	for i := 0; i < list.Len(); i++ {
		email := list.Get(i).String()
		if parts := strings.Split(email, ">>>"); len(parts) >= 2 {
			email = parts[1]
		}
		if email == "" || seen[email] {
			continue
		}
		seen[email] = true
		select {
		case emails <- email:
		case <-ctx.Done():
			break sendLoop
		}
	}
	close(emails)
	workers.Wait()
	return traffic, online, nil
}
