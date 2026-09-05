package app

import (
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Win32 bits Burnt needs that x/sys/windows doesn't already wrap. The DLLs are loaded
// with NewLazySystemDLL (always from %SystemRoot%\System32) so a stray user32.dll or
// dwmapi.dll next to burnt.exe can never be picked up instead.
var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	dwmapi   = windows.NewLazySystemDLL("dwmapi.dll")

	procGetCursorPos          = user32.NewProc("GetCursorPos")
	procSystemParametersInfoW = user32.NewProc("SystemParametersInfoW")
	procGetDpiForWindow       = user32.NewProc("GetDpiForWindow")
	procSetWindowPos          = user32.NewProc("SetWindowPos")
	procShowWindow            = user32.NewProc("ShowWindow")
	procSetForegroundWindow   = user32.NewProc("SetForegroundWindow")
	procGetForegroundWindow   = user32.NewProc("GetForegroundWindow")
	procGetAncestor           = user32.NewProc("GetAncestor")
	procSetWindowLongPtrW     = user32.NewProc("SetWindowLongPtrW")
	procOpenClipboard         = user32.NewProc("OpenClipboard")
	procCloseClipboard        = user32.NewProc("CloseClipboard")
	procEmptyClipboard        = user32.NewProc("EmptyClipboard")
	procSetClipboardData      = user32.NewProc("SetClipboardData")

	procGlobalAlloc  = kernel32.NewProc("GlobalAlloc")
	procGlobalLock   = kernel32.NewProc("GlobalLock")
	procGlobalUnlock = kernel32.NewProc("GlobalUnlock")
	procGlobalFree   = kernel32.NewProc("GlobalFree")
	procRtlMoveMem   = kernel32.NewProc("RtlMoveMemory")

	procDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")
)

const (
	gwlStyle   = ^uintptr(15) // -16
	gwlExStyle = ^uintptr(19) // -20

	wsPopup        = 0x80000000
	wsClipChildren = 0x02000000
	wsExToolWindow = 0x00000080
	wsExTopmost    = 0x00000008

	swpNoSize       = 0x0001
	swpNoMove       = 0x0002
	swpNoActivate   = 0x0010
	swpFrameChanged = 0x0020
	hwndTopmost     = ^uintptr(0) // -1

	swHide = 0
	swShow = 5

	spiGetWorkArea = 0x0030
	gaRoot         = 2

	// Windows 11 rounded corners. Both are no-ops (an HRESULT failure we ignore) on
	// Windows 10, which has no corner preference.
	dwmwaWindowCornerPreference = 33
	dwmwcpRound                 = 2

	cfUnicodeText = 13
	gmemMoveable  = 0x0002
)

type point struct{ X, Y int32 }

// cursorPos is where the user just clicked the tray icon.
func cursorPos() point {
	var p point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	return p
}

// workArea is the monitor area excluding the taskbar. SPI_GETWORKAREA reports the
// primary monitor, which is where the tray lives on every normal setup.
func workArea() Rect {
	var r Rect
	procSystemParametersInfoW.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&r)), 0)
	if r.Width() <= 0 || r.Height() <= 0 {
		return Rect{Left: 0, Top: 0, Right: 1920, Bottom: 1080}
	}
	return r
}

// windowDPI is the window's effective DPI (96 = 100 %).
func windowDPI(hwnd windows.HWND) uint32 {
	if err := procGetDpiForWindow.Find(); err != nil {
		return 96
	}
	dpi, _, _ := procGetDpiForWindow.Call(uintptr(hwnd))
	if dpi == 0 {
		return 96
	}
	return uint32(dpi)
}

// makeFramelessPopup turns the WebView2 host window into a frameless, always-on-top
// tool window: no title bar, no border, no taskbar button and no Alt-Tab entry, with
// Windows 11's rounded corners when the OS offers them.
func makeFramelessPopup(hwnd windows.HWND) {
	procSetWindowLongPtrW.Call(uintptr(hwnd), gwlStyle, wsPopup|wsClipChildren)
	procSetWindowLongPtrW.Call(uintptr(hwnd), gwlExStyle, wsExToolWindow|wsExTopmost)

	pref := int32(dwmwcpRound)
	procDwmSetWindowAttribute.Call(uintptr(hwnd), dwmwaWindowCornerPreference,
		uintptr(unsafe.Pointer(&pref)), unsafe.Sizeof(pref))

	// SWP_FRAMECHANGED makes the shell recompute the (now absent) frame.
	procSetWindowPos.Call(uintptr(hwnd), hwndTopmost, 0, 0, 0, 0,
		swpNoMove|swpNoSize|swpNoActivate|swpFrameChanged)
}

// moveWindow positions and resizes the popover without activating it.
func moveWindow(hwnd windows.HWND, x, y, w, h int32) {
	procSetWindowPos.Call(uintptr(hwnd), hwndTopmost, uintptr(x), uintptr(y),
		uintptr(w), uintptr(h), swpNoActivate)
}

func showWindow(hwnd windows.HWND) {
	procShowWindow.Call(uintptr(hwnd), swShow)
	procSetForegroundWindow.Call(uintptr(hwnd))
}

func hideWindow(hwnd windows.HWND) {
	procShowWindow.Call(uintptr(hwnd), swHide)
}

// hasFocus reports whether the foreground window is the popover or one of its
// children (the WebView2 render widget is a child window, and it is what actually
// holds focus while the user types).
func hasFocus(hwnd windows.HWND) bool {
	fg, _, _ := procGetForegroundWindow.Call()
	if fg == 0 {
		// Nothing is foreground (a menu is opening, the desktop is switching): say
		// yes, so a transient state never dismisses the popover.
		return true
	}
	root, _, _ := procGetAncestor.Call(fg, gaRoot)
	return root == uintptr(hwnd)
}

// openURL hands a link to the default browser. Only http(s) is allowed: this is
// reachable from JavaScript, and ShellExecute would happily launch a local
// executable or a custom protocol handler otherwise.
func openURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("app: bad url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("app: refusing to open %q", u.Scheme)
	}
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	target, err := windows.UTF16PtrFromString(u.String())
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, target, nil, nil, windows.SW_SHOWNORMAL)
}

// setClipboardText puts text on the clipboard as CF_UNICODETEXT, falling back to
// PowerShell's Set-Clipboard when the clipboard is locked by another process.
func setClipboardText(text string) error {
	if err := setClipboardDirect(text); err == nil {
		return nil
	} else if fallbackErr := setClipboardPowerShell(text); fallbackErr != nil {
		return fmt.Errorf("%w (fallback: %v)", err, fallbackErr)
	}
	return nil
}

func setClipboardDirect(text string) error {
	utf16, err := windows.UTF16FromString(text)
	if err != nil {
		return fmt.Errorf("app: clipboard text: %w", err)
	}
	if err := openClipboard(); err != nil {
		return err
	}
	defer procCloseClipboard.Call()

	procEmptyClipboard.Call()

	h, _, callErr := procGlobalAlloc.Call(gmemMoveable, uintptr(len(utf16)*2))
	if h == 0 {
		return fmt.Errorf("app: GlobalAlloc: %v", callErr)
	}
	ptr, _, _ := procGlobalLock.Call(h)
	if ptr == 0 {
		procGlobalFree.Call(h)
		return errors.New("app: GlobalLock failed")
	}
	// Copied through RtlMoveMemory rather than a Go slice so no uintptr is ever
	// converted back into an unsafe.Pointer (which the GC is entitled to break).
	procRtlMoveMem.Call(ptr, uintptr(unsafe.Pointer(&utf16[0])), uintptr(len(utf16)*2))
	procGlobalUnlock.Call(h)

	if ok, _, callErr := procSetClipboardData.Call(cfUnicodeText, h); ok == 0 {
		procGlobalFree.Call(h) // ownership only transfers on success
		return fmt.Errorf("app: SetClipboardData: %v", callErr)
	}
	return nil
}

// openClipboard retries briefly: the clipboard is a single global lock and another
// app may be mid-copy.
func openClipboard() error {
	for attempt := 0; attempt < 6; attempt++ {
		if ok, _, _ := procOpenClipboard.Call(0); ok != 0 {
			return nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	return errors.New("app: OpenClipboard timed out")
}

func setClipboardPowerShell(text string) error {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive",
		"-Command", "$input | Set-Clipboard")
	cmd.Stdin = strings.NewReader(text)
	HideConsole(cmd)
	return cmd.Run()
}

// singleInstanceMutex is held for the whole process lifetime; releasing it would let
// a second Burnt start.
var singleInstanceMutex windows.Handle

// claimSingleInstance reports whether this process is the only Burnt. A named mutex
// in the Local\ namespace scopes it to the current session, so Burnt still runs once
// per user on a shared machine.
func claimSingleInstance() bool {
	name, err := windows.UTF16PtrFromString(`Local\BurntSingleInstance`)
	if err != nil {
		return true
	}
	h, err := windows.CreateMutex(nil, false, name)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		if h != 0 {
			windows.CloseHandle(h)
		}
		return false
	}
	singleInstanceMutex = h
	return true
}
