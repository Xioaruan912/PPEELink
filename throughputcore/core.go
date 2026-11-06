// Package throughputcore defines the panel/Agent throughput wire contract.
// Execution and tool preparation live exclusively on the agent branch.
package throughputcore

import "time"

type Job struct {
	ID         uint      `json:"id"`
	Role       string    `json:"role"`
	Host       string    `json:"host"`
	Port       int       `json:"port"`
	Protocol   string    `json:"protocol"`
	Direction  string    `json:"direction"`
	Family     int       `json:"family"`
	Duration   int       `json:"duration"`
	Streams    int       `json:"streams"`
	RateMbps   float64   `json:"rateMbps"`
	Username   string    `json:"username"`
	Password   string    `json:"password"`
	PublicKey  string    `json:"publicKey,omitempty"`
	PrivateKey string    `json:"privateKey,omitempty"`
	ExpiresAt  time.Time `json:"expiresAt"`
}
type Report struct {
	ID     uint    `json:"id"`
	Role   string  `json:"role"`
	Status string  `json:"status"`
	Error  string  `json:"error,omitempty"`
	Result *Result `json:"result,omitempty"`
}
type Capability struct {
	Preparing           bool   `json:"preparing"`
	Available           bool   `json:"available"`
	Path                string `json:"path"`
	Version             string `json:"version"`
	AuthSupported       bool   `json:"authSupported"`
	BidirSupported      bool   `json:"bidirSupported"`
	JSONStreamSupported bool   `json:"jsonStreamSupported"`
	Error               string `json:"error,omitempty"`
}
type Result struct {
	ProgressIntervals   []Interval       `json:"progressIntervals,omitempty"`
	ProgressDirection   string           `json:"progressDirection,omitempty"`
	ProgressObservation string           `json:"progressObservation,omitempty"`
	Protocol            string           `json:"protocol"`
	Direction           string           `json:"direction"`
	Duration            float64          `json:"duration"`
	Streams             int              `json:"streams"`
	RateMbps            float64          `json:"rateMbps"`
	CapReached          bool             `json:"capReached"`
	Forward             *DirectionResult `json:"forward,omitempty"`
	Reverse             *DirectionResult `json:"reverse,omitempty"`
	CPUHost             float64          `json:"cpuHost"`
	CPURemote           float64          `json:"cpuRemote"`
	Warnings            []string         `json:"warnings,omitempty"`
}
type DirectionResult struct {
	RTTMs         *float64   `json:"rttMs,omitempty"`
	BitsPerSecond float64    `json:"bitsPerSecond"`
	Bytes         uint64     `json:"bytes"`
	Retransmits   uint64     `json:"retransmits"`
	JitterMs      float64    `json:"jitterMs"`
	LostPackets   uint64     `json:"lostPackets"`
	Packets       uint64     `json:"packets"`
	LossPercent   float64    `json:"lossPercent"`
	Intervals     []Interval `json:"intervals"`
}
type Interval struct {
	Start         float64 `json:"start"`
	End           float64 `json:"end"`
	BitsPerSecond float64 `json:"bitsPerSecond"`
	Bytes         uint64  `json:"bytes"`
	JitterMs      float64 `json:"jitterMs"`
	LossPercent   float64 `json:"lossPercent"`
}
