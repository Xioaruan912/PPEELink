//go:build windows

package main

import (
	"encoding/binary"
	"fmt"
	"os/exec"
	"runtime"
	"sync/atomic"
	"syscall"
	"unsafe"
)

var (
	modUser32              = syscall.NewLazyDLL("user32.dll")
	modShell32             = syscall.NewLazyDLL("shell32.dll")
	modKernel32            = syscall.NewLazyDLL("kernel32.dll")
	procGetModuleHandleW   = modKernel32.NewProc("GetModuleHandleW")
	procRegisterClassExW   = modUser32.NewProc("RegisterClassExW")
	procCreateWindowExW    = modUser32.NewProc("CreateWindowExW")
	procDefWindowProcW     = modUser32.NewProc("DefWindowProcW")
	procGetMessageW        = modUser32.NewProc("GetMessageW")
	procTranslateMessage   = modUser32.NewProc("TranslateMessage")
	procDispatchMessageW   = modUser32.NewProc("DispatchMessageW")
	procPostQuitMessage    = modUser32.NewProc("PostQuitMessage")
	procPostMessageW       = modUser32.NewProc("PostMessageW")
	procDestroyWindow      = modUser32.NewProc("DestroyWindow")
	procCreatePopupMenu    = modUser32.NewProc("CreatePopupMenu")
	procAppendMenuW        = modUser32.NewProc("AppendMenuW")
	procTrackPopupMenu     = modUser32.NewProc("TrackPopupMenu")
	procDestroyMenu        = modUser32.NewProc("DestroyMenu")
	procGetCursorPos       = modUser32.NewProc("GetCursorPos")
	procSetForegroundWin   = modUser32.NewProc("SetForegroundWindow")
	procCreateIconFromRes  = modUser32.NewProc("CreateIconFromResourceEx")
	procDestroyIcon        = modUser32.NewProc("DestroyIcon")
	procShellNotifyIconW   = modShell32.NewProc("Shell_NotifyIconW")
)

const (
	wmDestroy        = 0x0002
	wmCommand        = 0x0111
	wmApp            = 0x8000
	wmTrayCallback   = wmApp + 1
	wmLButtonDblClk  = 0x0203
	wmRButtonUp      = 0x0205
	nimAdd           = 0x00000000
	nimDelete        = 0x00000002
	nifMessage       = 0x00000001
	nifIcon          = 0x00000002
	nifTip           = 0x00000004
	mfString         = 0x00000000
	mfSeparator      = 0x00000800
	mfGrayed         = 0x00000001
	tpmRightButton   = 0x0002
	tpmBottomAlign   = 0x0020
	cmdOpenPanel     = 1001
	cmdTogglePause   = 1002
	cmdExitAgent     = 1003
)

type wndClassExW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

type winPoint struct {
	X int32
	Y int32
}

type winMsg struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      winPoint
}

type notifyIconDataW struct {
	CbSize           uint32
	HWnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	SzTip            [128]uint16
}

type trayController struct {
	panelURL string
	paused   *atomic.Bool
	onQuit   func()
	hwnd     uintptr
	hIcon    uintptr
}

var activeTray *trayController

func startSystemTray(panelURL string, paused *atomic.Bool, onQuit func()) func() {
	tc := &trayController{
		panelURL: panelURL,
		paused:   paused,
		onQuit:   onQuit,
	}
	activeTray = tc

	ready := make(chan struct{})
	go tc.run(ready)
	<-ready

	return func() {
		if tc.hwnd != 0 {
			procPostMessageW.Call(tc.hwnd, wmDestroy, 0, 0)
		}
	}
}

func (tc *trayController) run(ready chan struct{}) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hInst, _, _ := procGetModuleHandleW.Call(0)
	className, _ := syscall.UTF16PtrFromString("PPEELinkUserAgentTrayClass")

	wc := wndClassExW{
		CbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		LpfnWndProc:   syscall.NewCallback(trayWndProc),
		HInstance:     hInst,
		LpszClassName: className,
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(className)),
		0, 0, 0, 0, 0,
		0, 0, hInst, 0,
	)
	tc.hwnd = hwnd
	close(ready)
	if hwnd == 0 {
		return
	}

	tc.hIcon = buildEmeraldTrayIcon()
	nid := notifyIconDataW{
		CbSize:           uint32(unsafe.Sizeof(notifyIconDataW{})),
		HWnd:             hwnd,
		UID:              1,
		UFlags:           nifMessage | nifIcon | nifTip,
		UCallbackMessage: wmTrayCallback,
		HIcon:            tc.hIcon,
	}
	tip, _ := syscall.UTF16FromString(fmt.Sprintf("PPEELink 用户 Agent v%s（在线待命中）", agentVersion))
	copy(nid.SzTip[:], tip)
	procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&nid)))

	var m winMsg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}

	procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
	if tc.hIcon != 0 {
		procDestroyIcon.Call(tc.hIcon)
	}
}

func trayWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmTrayCallback:
		switch uint32(lParam) {
		case wmLButtonDblClk:
			if activeTray != nil {
				openBrowserURL(activeTray.panelURL + "/#/monitor/user-agent")
			}
		case wmRButtonUp:
			if activeTray != nil {
				activeTray.showContextMenu(hwnd)
			}
		}
		return 0
	case wmCommand:
		if activeTray != nil {
			switch uint32(wParam & 0xffff) {
			case cmdOpenPanel:
				openBrowserURL(activeTray.panelURL + "/#/monitor/user-agent")
			case cmdTogglePause:
				activeTray.paused.Store(!activeTray.paused.Load())
			case cmdExitAgent:
				if activeTray.onQuit != nil {
					activeTray.onQuit()
				}
				procDestroyWindow.Call(hwnd)
			}
		}
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return ret
}

func (tc *trayController) showContextMenu(hwnd uintptr) {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)

	statusLabel := fmt.Sprintf("PPEELink 用户 Agent v%s · 运行中", agentVersion)
	pauseLabel := "暂停定时采样"
	if tc.paused.Load() {
		statusLabel = fmt.Sprintf("PPEELink 用户 Agent v%s · 已暂停", agentVersion)
		pauseLabel = "恢复运行"
	}

	appendMenuText(menu, mfString|mfGrayed, 0, statusLabel)
	procAppendMenuW.Call(menu, mfSeparator, 0, 0)
	appendMenuText(menu, mfString, cmdOpenPanel, "打开面板")
	appendMenuText(menu, mfString, cmdTogglePause, pauseLabel)
	procAppendMenuW.Call(menu, mfSeparator, 0, 0)
	appendMenuText(menu, mfString, cmdExitAgent, "退出")

	var pt winPoint
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	procSetForegroundWin.Call(hwnd)
	procTrackPopupMenu.Call(menu, tpmRightButton|tpmBottomAlign, uintptr(pt.X), uintptr(pt.Y), 0, hwnd, 0)
}

func appendMenuText(menu uintptr, flags uint32, id uintptr, text string) {
	p, _ := syscall.UTF16PtrFromString(text)
	procAppendMenuW.Call(menu, uintptr(flags), id, uintptr(unsafe.Pointer(p)))
}

func openBrowserURL(u string) {
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	hideWindow(cmd)
	_ = cmd.Start()
}

// buildEmeraldTrayIcon constructs a crisp 16x16 32-bit BGRA circular emerald icon in memory.
func buildEmeraldTrayIcon() uintptr {
	const size = 16
	headerSize := 40
	pixelBytes := size * size * 4
	maskRowBytes := ((size + 31) / 32) * 4
	maskBytes := maskRowBytes * size
	buf := make([]byte, headerSize+pixelBytes+maskBytes)

	// BITMAPINFOHEADER
	binary.LittleEndian.PutUint32(buf[0:4], uint32(headerSize))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(size))
	binary.LittleEndian.PutUint32(buf[8:12], uint32(size*2)) // height * 2 for XOR+AND masks
	binary.LittleEndian.PutUint16(buf[12:14], 1)             // planes
	binary.LittleEndian.PutUint16(buf[14:16], 32)            // 32bpp BGRA
	binary.LittleEndian.PutUint32(buf[20:24], uint32(pixelBytes))

	// Draw crisp anti-aliased emerald ring with bright center (Apple status dot aesthetic).
	cx, cy := 7.5, 7.5
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			dist := dx*dx + dy*dy
			off := headerSize + (y*size+x)*4
			if dist <= 36.0 { // radius 6px
				// Outer emerald (#10b981 -> B=0x81, G=0xb9, R=0x10), inner highlight
				if dist <= 9.0 {
					buf[off+0] = 0xbb // B
					buf[off+1] = 0xf7 // G
					buf[off+2] = 0x6e // R
					buf[off+3] = 0xff // A
				} else {
					buf[off+0] = 0x81 // B
					buf[off+1] = 0xb9 // G
					buf[off+2] = 0x10 // R
					buf[off+3] = 0xff // A
				}
			}
		}
	}

	hIcon, _, _ := procCreateIconFromRes.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		1, // fIcon = TRUE
		0x00030000,
		size,
		size,
		0,
	)
	return hIcon
}
