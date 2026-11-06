package main

import (
	"fmt"
	"github.com/Xioaruan912/PPEELink/agentversion"
	"os"
)

var version = agentversion.Current

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: ppeelink-agent agent|monitor|probe [options]; --version")
		return
	}
	switch os.Args[1] {
	case "--version", "-version", "version":
		fmt.Println(version)
	case "agent":
		runAgent(os.Args[2:])
	case "monitor":
		runMonitorClient(os.Args[2:])
	case "probe":
		runProbeClient(os.Args[2:])
	default:
		fmt.Fprintln(os.Stderr, "unknown mode; use agent, monitor or probe")
		os.Exit(2)
	}
}
