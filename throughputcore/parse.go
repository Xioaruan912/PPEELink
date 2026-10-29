package throughputcore

import (
	"encoding/json"
	"errors"
	"fmt"
)

type iperfSum struct {
	MeanRTT     float64 `json:"mean_rtt"`
	Start       float64 `json:"start"`
	End         float64 `json:"end"`
	Seconds     float64 `json:"seconds"`
	Bytes       uint64  `json:"bytes"`
	BPS         float64 `json:"bits_per_second"`
	Retransmits uint64  `json:"retransmits"`
	Jitter      float64 `json:"jitter_ms"`
	Lost        uint64  `json:"lost_packets"`
	Packets     uint64  `json:"packets"`
	Loss        float64 `json:"lost_percent"`
	Sender      bool    `json:"sender"`
	Omitted     bool    `json:"omitted"`
}
type iperfInterval struct {
	Sum     *iperfSum `json:"sum"`
	Reverse *iperfSum `json:"sum_bidir_reverse"`
}
type iperfJSON struct {
	Error string `json:"error"`
	End   struct {
		Streams []struct {
			Sender *iperfSum `json:"sender"`
		} `json:"streams"`
		Received        *iperfSum `json:"sum_received"`
		Sent            *iperfSum `json:"sum_sent"`
		ReverseReceived *iperfSum `json:"sum_received_bidir_reverse"`
		ReverseSent     *iperfSum `json:"sum_sent_bidir_reverse"`
		Sum             *iperfSum `json:"sum"`
		ReverseSum      *iperfSum `json:"sum_bidir_reverse"`
		CPU             struct {
			Host   float64 `json:"host_total"`
			Remote float64 `json:"remote_total"`
		} `json:"cpu_utilization_percent"`
	} `json:"end"`
	Intervals []iperfInterval `json:"intervals"`
	Server    *iperfJSON      `json:"server_output_json"`
}

func ParseResult(raw []byte, j Job) (Result, error) {
	r := Result{Protocol: j.Protocol, Direction: j.Direction, Duration: float64(j.Duration), Streams: j.Streams, RateMbps: j.RateMbps}
	var d iperfJSON
	if e := json.Unmarshal(raw, &d); e != nil {
		return r, fmt.Errorf("invalid iperf3 JSON: %w", e)
	}
	if d.Error != "" {
		return r, fmt.Errorf("iperf3: %s", d.Error)
	}
	r.CPUHost = d.End.CPU.Host
	r.CPURemote = d.End.CPU.Remote
	read := func(sum, sent *iperfSum, intervals []iperfInterval, reverse bool) *DirectionResult {
		if sum == nil {
			return nil
		}
		x := &DirectionResult{BitsPerSecond: sum.BPS, Bytes: sum.Bytes, JitterMs: sum.Jitter, LostPackets: sum.Lost, Packets: sum.Packets, LossPercent: sum.Loss, Intervals: []Interval{}}
		if sent != nil {
			x.Retransmits = sent.Retransmits
		}
		if sum.Seconds > 0 {
			r.Duration = sum.Seconds
		}
		for _, v := range intervals {
			s := v.Sum
			if reverse {
				s = v.Reverse
			}
			if s != nil && !s.Omitted && !s.Sender {
				x.Intervals = append(x.Intervals, Interval{Start: s.Start, End: s.End, BitsPerSecond: s.BPS, Bytes: s.Bytes, JitterMs: s.Jitter, LossPercent: s.Loss})
			}
		}
		return x
	}
	intervals := d.Intervals
	if j.Direction == "forward" || j.Direction == "bidir" {
		if d.Server != nil {
			intervals = d.Server.Intervals
		}
		sum := d.End.Received
		if sum == nil && j.Protocol == "udp" && d.End.Sum != nil && !d.End.Sum.Sender {
			sum = d.End.Sum
		}
		r.Forward = read(sum, d.End.Sent, intervals, false)
		if r.Forward == nil {
			return r, errors.New("receiver summary missing for forward direction")
		}
		if len(r.Forward.Intervals) == 0 {
			r.Warnings = append(r.Warnings, "Forward receiver interval curve unavailable")
		}
	}
	if j.Direction == "reverse" || j.Direction == "bidir" {
		sum, sent := d.End.Received, d.End.Sent
		reverse := false
		if j.Direction == "bidir" {
			sum, sent = d.End.ReverseReceived, d.End.ReverseSent
			reverse = true
		}
		if sum == nil && j.Protocol == "udp" {
			sum = d.End.Sum
			if reverse {
				sum = d.End.ReverseSum
			}
			if sum != nil && sum.Sender {
				sum = nil
			}
		}
		r.Reverse = read(sum, sent, d.Intervals, reverse)
		if r.Reverse == nil {
			return r, errors.New("receiver summary missing for reverse direction")
		}
		if len(r.Reverse.Intervals) == 0 {
			r.Warnings = append(r.Warnings, "Reverse receiver interval curve unavailable")
		}
	}
	if j.Protocol == "tcp" {
		var forward, reverse float64
		var nf, nr int
		for _, s := range d.End.Streams {
			if s.Sender == nil || s.Sender.MeanRTT <= 0 {
				continue
			}
			if j.Direction == "forward" || (j.Direction == "bidir" && s.Sender.Sender) {
				forward += s.Sender.MeanRTT
				nf++
			} else {
				reverse += s.Sender.MeanRTT
				nr++
			}
		}
		if nf > 0 && r.Forward != nil {
			ms := forward / float64(nf) / 1000
			r.Forward.RTTMs = &ms
		}
		if nr > 0 && r.Reverse != nil {
			ms := reverse / float64(nr) / 1000
			r.Reverse.RTTMs = &ms
		}
	}
	capPerDirection := j.RateMbps * 1e6
	if j.Direction == "bidir" {
		capPerDirection /= 2
	}
	for _, v := range []*DirectionResult{r.Forward, r.Reverse} {
		if v != nil && v.BitsPerSecond >= capPerDirection*.95 {
			r.CapReached = true
		}
	}
	return r, nil
}
