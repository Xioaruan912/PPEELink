//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

func spawnDetached(exe string, args ...string) error {
	cmd := exec.Command(exe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err == nil {
		cmd.Stdin = devNull
		cmd.Stdout = devNull
		cmd.Stderr = devNull
		defer devNull.Close()
	}
	return cmd.Start()
}

func plistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", "com.ppeelink.user-agent.plist"), nil
}

func systemdUnitPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "systemd", "user", "ppeelink-user-agent.service"), nil
}

func installBackground(panelURL, token string, concurrency int) error {
	dir, err := installDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := writeSavedConfig(savedConfig{Panel: panelURL, Token: token, Concurrency: concurrency}); err != nil {
		return err
	}
	targetExe := filepath.Join(dir, binaryFilename())
	selfExe, err := os.Executable()
	if err != nil {
		return err
	}
	if filepath.Clean(selfExe) != filepath.Clean(targetExe) {
		if err := copyFile(selfExe, targetExe); err != nil {
			return err
		}
	}

	if runtime.GOOS == "darwin" {
		plist, err := plistPath()
		if err == nil {
			_ = os.MkdirAll(filepath.Dir(plist), 0755)
			content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>com.ppeelink.user-agent</string>
  <key>ProgramArguments</key>
  <array><string>%s</string><string>-daemon</string></array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
</dict>
</plist>
`, targetExe)
			if err := os.WriteFile(plist, []byte(content), 0644); err == nil {
				_ = exec.Command("launchctl", "unload", plist).Run()
				if exec.Command("launchctl", "load", "-w", plist).Run() == nil {
					if filepath.Clean(selfExe) != filepath.Clean(targetExe) {
						_ = os.Remove(selfExe)
					}
					return nil
				}
			}
		}
	} else {
		if unit, err := systemdUnitPath(); err == nil {
			_ = os.MkdirAll(filepath.Dir(unit), 0755)
			content := fmt.Sprintf(`[Unit]
Description=PPEELink User Agent
After=network-online.target

[Service]
ExecStart=%s -daemon
Restart=always
RestartSec=5

[Install]
WantedBy=default.target
`, targetExe)
			if err := os.WriteFile(unit, []byte(content), 0644); err == nil {
				_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
				if exec.Command("systemctl", "--user", "enable", "--now", "ppeelink-user-agent.service").Run() == nil {
					if filepath.Clean(selfExe) != filepath.Clean(targetExe) {
						_ = os.Remove(selfExe)
					}
					return nil
				}
			}
		}
	}

	// Fallback: detached background daemon without systemd/launchctl.
	if err := spawnDetached(targetExe, "-daemon"); err != nil {
		return err
	}
	if filepath.Clean(selfExe) != filepath.Clean(targetExe) {
		_ = os.Remove(selfExe)
	}
	return nil
}

func performTracelessUninstall() {
	if runtime.GOOS == "darwin" {
		if plist, err := plistPath(); err == nil {
			_ = exec.Command("launchctl", "unload", "-w", plist).Run()
			_ = os.Remove(plist)
		}
	} else {
		_ = exec.Command("systemctl", "--user", "disable", "--now", "ppeelink-user-agent.service").Run()
		if unit, err := systemdUnitPath(); err == nil {
			_ = os.Remove(unit)
		}
		_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	}
	dir, _ := installDir()
	exe, _ := os.Executable()
	var cmds []string
	cmds = append(cmds, "sleep 1")
	if exe != "" {
		cmds = append(cmds, fmt.Sprintf("rm -f %q %q %q", exe, exe+".old", exe+".new"))
	}
	if dir != "" {
		cmds = append(cmds, fmt.Sprintf("rm -rf %q", dir))
	}
	cmd := exec.Command("/bin/sh", "-c", strings.Join(cmds, "; "))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	_ = cmd.Start()
}
