package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type savedConfig struct {
	Panel          string `json:"panel"`
	Token          string `json:"token"`
	Concurrency    int    `json:"concurrency,omitempty"`
	ClashAPI       string `json:"clashApi,omitempty"`
	ClashSecret    string `json:"clashSecret,omitempty"`
	ReportPublicIP *bool  `json:"reportPublicIp,omitempty"`
	AllowClash     *bool  `json:"allowClash,omitempty"`
}

func installDir() (string, error) {
	switch runtime.GOOS {
	case "windows":
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(base, "PPEELinkUserAgent"), nil
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", "PPEELinkUserAgent"), nil
	default:
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "share", "ppeelink-user-agent"), nil
	}
}

func binaryFilename() string {
	if runtime.GOOS == "windows" {
		return "ppeelink-user-agent.exe"
	}
	return "ppeelink-user-agent"
}

func configPath() (string, error) {
	dir, err := installDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func loadSavedConfig() (*savedConfig, error) {
	p, err := configPath()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var cfg savedConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func writeSavedConfig(cfg savedConfig) error {
	dir, err := installDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "config.json"), raw, 0600)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if runtime.GOOS == "windows" {
		_ = os.Remove(dst + ".old")
		_ = os.Rename(dst, dst+".old")
	}
	return os.Rename(tmp, dst)
}

func cleanupOldBinary() {
	if exe, err := os.Executable(); err == nil {
		_ = os.Remove(exe + ".old")
		_ = os.Remove(exe + ".new")
		_ = os.Remove(exe + ".tmp")
	}
}

func performSelfUpdate(client *http.Client, panelURL, token string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	url := fmt.Sprintf("%s/api/v1/probe/download?os=%s&arch=%s", strings.TrimRight(panelURL, "/"), runtime.GOOS, runtime.GOARCH)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Probe "+token)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载更新失败: HTTP %d", resp.StatusCode)
	}
	newPath := exe + ".new"
	f, err := os.OpenFile(newPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		_ = os.Remove(newPath)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(newPath)
		return err
	}
	if runtime.GOOS == "windows" {
		oldPath := exe + ".old"
		_ = os.Remove(oldPath)
		if err := os.Rename(exe, oldPath); err != nil {
			_ = os.Remove(newPath)
			return err
		}
		if err := os.Rename(newPath, exe); err != nil {
			_ = os.Rename(oldPath, exe)
			return err
		}
	} else {
		if err := os.Rename(newPath, exe); err != nil {
			_ = os.Remove(newPath)
			return err
		}
	}
	return spawnDetached(exe, "-daemon")
}
