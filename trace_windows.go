//go:build windows

package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
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

// icmpEchoReply matches Windows ICMP_ECHO_REPLY (x64 layout).
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
	ipSuccess         = 0
	ipTTLExpiredTrans = 11013
	ipTTLExpiredReasm = 11014
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

type winProbeResult struct {
	hop     traceHop
	reached bool
}

func probeSingleWinTTL(srcAddr, dstAddr uint32, dstStr string, ttl int) winProbeResult {
	handle, _, _ := procIcmpCreate.Call()
	if handle == 0 || handle == ^uintptr(0) {
		return winProbeResult{hop: traceHop{TTL: ttl, Timeout: true, RTT: -1}}
	}
	defer procIcmpClose.Call(handle)

	payload := []byte("ppeelink-trace-probe")
	replyBuf := make([]byte, unsafe.Sizeof(icmpEchoReply{})+uintptr(len(payload))+64)

	// Try up to 2 attempts per TTL so occasional ICMP rate-limiting doesn't turn a real hop into '*'.
	for attempt := 0; attempt < 2; attempt++ {
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
			continue
		}
		reply := (*icmpEchoReply)(unsafe.Pointer(&replyBuf[0]))
		hopIP := uint32LEToIPv4(reply.Address)
		rtt := int(reply.RoundTripTime)
		if rtt <= 0 && elapsed > 0 {
			rtt = elapsed
		}
		if rtt <= 0 {
			rtt = 1
		}
		reached := reply.Status == ipSuccess || hopIP == dstStr
		return winProbeResult{
			hop: traceHop{
				TTL:     ttl,
				IP:      hopIP,
				RTT:     rtt,
				Timeout: false,
			},
			reached: reached,
		}
	}
	return winProbeResult{hop: traceHop{TTL: ttl, Timeout: true, RTT: -1}}
}

// platformTrace uses Windows built-in IcmpSendEcho2Ex with SourceAddress bound
// to the physical NIC IPv4, probing TTL 1..24 concurrently so MPLS / CERNET
// silent intermediate hops never cause premature termination at hop 10.
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
	dstStr := dstV4.String()

	const maxHops = 24
	results := make([]winProbeResult, maxHops)

	// Probe TTL 1..24 concurrently (bounded concurrency 8) -> completes in ~2-3s total!
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for ttl := 1; ttl <= maxHops; ttl++ {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		go func(t int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if ctx.Err() != nil {
				results[t-1] = winProbeResult{hop: traceHop{TTL: t, Timeout: true, RTT: -1}}
				return
			}
			results[t-1] = probeSingleWinTTL(srcAddr, dstAddr, dstStr, t)
		}(ttl)
	}
	wg.Wait()

	// Find first TTL that reached destination, or last responding hop.
	cutoff := maxHops
	for i := 0; i < maxHops; i++ {
		if results[i].reached {
			cutoff = i + 1
			break
		}
	}
	if cutoff == maxHops {
		lastResp := 0
		for i := maxHops - 1; i >= 0; i-- {
			if !results[i].hop.Timeout && results[i].hop.IP != "" {
				lastResp = i + 1
				break
			}
		}
		if lastResp > 0 {
			cutoff = lastResp + 3
			if cutoff > maxHops {
				cutoff = maxHops
			}
		} else {
			cutoff = 6
		}
	}

	hops := make([]traceHop, 0, cutoff)
	for i := 0; i < cutoff; i++ {
		hops = append(hops, results[i].hop)
	}
	return hops, nil
}
