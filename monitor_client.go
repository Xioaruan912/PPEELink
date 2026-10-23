package main

// runMonitorClient is the backwards-compatible entry point for installations
// using a Monitor token. The panel binds that token to a proxy-disabled agent;
// all configuration, heartbeat, upgrades, terminal and jobs use the same agent
// lifecycle. Enabling proxy service later requires only a Web configuration change.
func runMonitorClient(args []string) {
	runAgent(args)
}
