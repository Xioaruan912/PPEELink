//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

const (
	detachedProcess       = 0x00000008
	createNewProcessGroup = 0x00000200
	createNoWindow        = 0x08000000
)

func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}

func spawnDetached(exe string, args ...string) error {
	cmd := exec.Command(exe, args...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow | createNewProcessGroup | detachedProcess,
	}
	return cmd.Start()
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
	// Stop any previous background instance before replacing binary.
	killCmd := exec.Command("taskkill", "/F", "/IM", binaryFilename(), "/FI", fmt.Sprintf("PID ne %d", os.Getpid()))
	hideWindow(killCmd)
	_ = killCmd.Run()

	if !strings.EqualFold(filepath.Clean(selfExe), filepath.Clean(targetExe)) {
		if err := copyFile(selfExe, targetExe); err != nil {
			return err
		}
	}
	regCmd := exec.Command("reg", "add",
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`,
		"/v", "PPEELinkUserAgent",
		"/t", "REG_SZ",
		"/d", fmt.Sprintf(`"%s" -daemon`, targetExe),
		"/f",
	)
	hideWindow(regCmd)
	if out, err := regCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("注册开机自启失败: %s", strings.TrimSpace(string(out)))
	}
	if err := spawnDetached(targetExe, "-daemon"); err != nil {
		return err
	}
	// Clean up temporary installer exe if launched from outside installDir.
	if !strings.EqualFold(filepath.Clean(selfExe), filepath.Clean(targetExe)) {
		cleanCmd := exec.Command("cmd", "/C", fmt.Sprintf(`ping 127.0.0.1 -n 2 >NUL & del /F /Q "%s"`, selfExe))
		hideWindow(cleanCmd)
		_ = cleanCmd.Start()
	}
	return nil
}

func performTracelessUninstall() {
	regCmd := exec.Command("reg", "delete",
		`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`,
		"/v", "PPEELinkUserAgent",
		"/f",
	)
	hideWindow(regCmd)
	_ = regCmd.Run()

	dir, _ := installDir()
	exe, _ := os.Executable()

	// Schedule hidden self-removal after this process exits so zero files remain.
	var parts []string
	parts = append(parts, "ping 127.0.0.1 -n 3 >NUL")
	if exe != "" {
		parts = append(parts, fmt.Sprintf(`del /F /Q "%s" "%s.old" "%s.new" 2>NUL`, exe, exe, exe))
	}
	if dir != "" {
		parts = append(parts, fmt.Sprintf(`rmdir /S /Q "%s" 2>NUL`, dir))
	}
	cmd := exec.Command("cmd", "/C", strings.Join(parts, " & "))
	hideWindow(cmd)
	_ = cmd.Start()
}
