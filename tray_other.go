//go:build !windows

package main

import "sync/atomic"

func startSystemTray(_ string, _ *atomic.Bool, _ func()) func() {
	return func() {}
}

func updateTrayTopNodes(_ []probeNode, _ []probeResult) {}

func updateTrayClashState(_, _ string, _ []string) {}
