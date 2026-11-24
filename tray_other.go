//go:build !windows

package main

import "sync/atomic"

func startSystemTray(_ string, _ *atomic.Bool, _ func()) func() {
	return func() {}
}
