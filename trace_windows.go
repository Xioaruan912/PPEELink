//go:build windows

package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"syscall"
	"time"
	"unsafe"
)

var (
	modIphlpapi       = syscall.NewLazyDLL("iphlpapi.dll")
	procIcmpCreate    = modIphlpapi.NewProc("IcmpCreateFile")
	procIcmpClose     = modIphlpapi.NewProc("IcmpCloseHandle")
	procIcmpSendEcho2 = modIphlpapi.NewProc("IcmpSendEcho2Ex")
)

type ipOptionInformation struct {
	Ttl         byte
	Tos         byte
	Flags       byte
	OptionsSize byte
	OptionsData uintptr
}

// icmpEchoReply32 matches Windows ICMP_ECHO_REPLY (x64 layout).
type icmpEchoReply struct {
	Address       uint32
	Status        uint32
	RoundTripTime uint32
	DataSize      uint16
	Reserved      uint16
	Data          uintptr
	Options       ipOptionInformation
}

const (
	ipSuccess          = 0
	ipTTLExpiredTrans  = 11013
	ipTTLExpiredReasm  = 11014
)

func ipv4ToUint32LE(ip net.IP) uint32 {
	v4 := ip.To4()
	if v4 == nil {
		return 0
	}
	return binary.LittleEndian.Uint32(v4)
}

func uint32LEToIPv4(v uint32) string {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	return net.IPv4(b[0], b[1], b[2], b[3]).String()
}

// platformTrace uses Windows built-in IcmpSendEcho2Ex with SourceAddress bound
// to the physical NIC IPv4, completely bypassing Clash/Mihomo TUN with zero
// admin privileges and zero console windows.
func platformTrace(ctx context.Context, host string) ([]traceHop, error) {
	dstIP, err := resolveTraceTargetIP(host)
	if err != nil {
		return nil, fmt.Errorf("解析目标失败: %w", err)
	}
	dstV4 := dstIP.To4()
	if dstV4 == nil {
		return nil, fmt.Errorf("当前 Windows 去程路由追踪仅支持 IPv4 目标")
	}

	iface := findPhysicalInterface(false)
	var srcAddr uint32
	if iface != nil && iface.IPv4 != nil {
		srcAddr = ipv4ToUint32LE(iface.IPv4)
	}
	dstAddr := ipv4ToUint32LE(dstV4)

	handle, _, err := procIcmpCreate.Call()
	if handle == 0 || handle == ^uintptr(0) {
		return nil, fmt.Errorf("初始化 ICMP 失败: %v", err)
	}
	defer procIcmpClose.Call(handle)

	payload := []byte("ppeelink-trace-probe")
	replyBuf := make([]byte, unsafe.Sizeof(icmpEchoReply{})+uintptr(len(payload))+64)

	const maxHops = 24
	hops := make([]traceHop, 0, maxHops)
	consecutiveTimeouts := 0

	for ttl := 1; ttl <= maxHops; ttl++ {
		if ctx.Err() != nil {
			break
		}
		opt := ipOptionInformation{Ttl: byte(ttl)}
		start := time.Now()
		ret, _, _ := procIcmpSendEcho2.Call(
			handle,
			0, // Event
			0, // ApcRoutine
			0, // ApcContext
			uintptr(srcAddr),
			uintptr(dstAddr),
			uintptr(unsafe.Pointer(&payload[0])),
			uintptr(len(payload)),
			uintptr(unsafe.Pointer(&opt)),
			uintptr(unsafe.Pointer(&replyBuf[0])),
			uintptr(len(replyBuf)),
			1100, // Timeout ms
		)
		elapsed := int(time.Since(start).Milliseconds())
		if ret == 0 {
			hops = append(hops, traceHop{TTL: ttl, Timeout: true, RTT: -1})
			consecutiveTimeouts++
			if consecutiveTimeouts >= 5 {
				break
			}
			continue
		}
		consecutiveTimeouts = 0
		reply := (*icmpEchoReply)(unsafe.Pointer(&replyBuf[0]))
		hopIP := uint32LEToIPv4(reply.Address)
		rtt := int(reply.RoundTripTime)
		if rtt <= 0 && elapsed > 0 {
			rtt = elapsed
		}
		if rtt <= 0 {
			rtt = 1
		}
		hops = append(hops, traceHop{
			TTL:     ttl,
			IP:      hopIP,
			RTT:     rtt,
			Timeout: false,
		})
		if reply.Status == ipSuccess || hopIP == dstV4.String() {
			break
		}
		if reply.Status != ipTTLExpiredTrans && reply.Status != ipTTLExpiredReasm {
			// Destination unreachable or filtered; stop after recording the responder.
			break
		}
	}
	return hops, nil
}
