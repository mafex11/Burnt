package app

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"
)

// focusPollInterval is how often we ask who has the foreground while the popover is
// up. A WS_POPUP tool window gets no WM_ACTIVATE we can hook (the window procedure
// belongs to go-webview2), so polling is the only way to dismiss on focus loss.
const focusPollInterval = 200 * time.Millisecond

// activationGrace ignores focus checks right after showing: SetForegroundWindow and
// WebView2's own focus handoff race, and a check in that window would hide the
// popover the instant it appeared.
const activationGrace = 600 * time.Millisecond

// popover is the frameless WebView2 window that hangs off the tray icon.
//
// The window and every WebView2 call live on one dedicated OS thread (Win32 requires
// it): thread() locks the thread, creates the window, then blocks in webview.Run()
// pumping messages. Everything else asks that thread to do work via w.Dispatch.
type popover struct {
	shell    *shell
	url      string
	dataPath string

	mu         sync.Mutex
	w          webview2.WebView
	hwnd       windows.HWND
	visible    bool
	heightDips int32
	anchor     point
	shownAt    time.Time

	createMu sync.Mutex // serialises lazy creation
	done     chan struct{}
}

func newPopover(s *shell, url, dataPath string) *popover {
	p := &popover{
		shell:      s,
		url:        url,
		dataPath:   dataPath,
		heightDips: PopoverInitDips,
		done:       make(chan struct{}),
	}
	safego("popover.watchFocus", p.watchFocus)
	return p
}

// ensure creates the window the first time it is needed. WebView2 creation costs a
// couple of hundred milliseconds and can fail outright (runtime not installed), so it
// is deliberately not done at launch: the tray must work either way.
func (p *popover) ensure() error {
	p.createMu.Lock()
	defer p.createMu.Unlock()
	if p.window() != nil {
		return nil
	}
	ready := make(chan error, 1)
	go p.thread(ready)
	return <-ready
}

// thread owns the popover window for its whole life.
func (p *popover) thread(ready chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer func() {
		if r := recover(); r != nil {
			logPanic("popover.thread", r)
			select {
			case ready <- errors.New("app: WebView2 window creation panicked"):
			default:
			}
		}
	}()

	// The WebView2 user-data folder is checked up front on purpose: go-webview2's
	// environment callback calls log.Fatal when creation fails, which would take the
	// whole tray down. An unwritable data folder is the likeliest cause, and it is the
	// one we can rule out before asking for a window.
	if err := probeWritable(p.dataPath); err != nil {
		ready <- err
		return
	}

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		DataPath:  p.dataPath,
		WindowOptions: webview2.WindowOptions{
			Title:  "Burnt",
			Width:  PopoverWidthDips,
			Height: PopoverInitDips,
			IconId: 1, // the flame from rsrc_windows_amd64.syso
		},
	})
	if w == nil {
		ready <- errors.New("app: WebView2 runtime is not available")
		return
	}

	hwnd := windows.HWND(w.Window())
	// NewWithOptions shows the window as soon as it is created; hide it before the
	// user sees a bare white rectangle in the middle of the screen.
	hideWindow(hwnd)
	makeFramelessPopup(hwnd)

	p.bind(w)

	p.mu.Lock()
	p.w, p.hwnd = w, hwnd
	p.mu.Unlock()

	w.Navigate(p.url)
	ready <- nil

	w.Run() // pumps messages until the window is destroyed (e.g. Alt+F4)

	p.mu.Lock()
	p.w, p.hwnd, p.visible = nil, 0, false
	p.mu.Unlock()
	log.Print("popover: window closed; it will be recreated on the next open")
}

func (p *popover) window() webview2.WebView {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.w
}

// IsVisible reports whether the popover is on screen. The poll loop uses it to decide
// whether to pay for project attribution.
func (p *popover) IsVisible() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.visible
}

// dispatch runs fn on the popover's own thread.
func (p *popover) dispatch(fn func()) {
	w := p.window()
	if w == nil {
		return
	}
	w.Dispatch(func() {
		defer func() {
			if r := recover(); r != nil {
				logPanic("popover.dispatch", r)
			}
		}()
		fn()
	})
}

// eval runs JavaScript in the dashboard.
func (p *popover) eval(js string) {
	w := p.window()
	if w == nil {
		return
	}
	w.Dispatch(func() { w.Eval(js) })
}

// show places the popover at the cursor and brings it up, creating it if needed.
func (p *popover) show() error {
	if err := p.ensure(); err != nil {
		return err
	}
	anchor := cursorPos()
	p.dispatch(func() {
		p.mu.Lock()
		hwnd, height := p.hwnd, p.heightDips
		p.anchor = anchor
		p.visible = true
		p.shownAt = time.Now()
		p.mu.Unlock()
		if hwnd == 0 {
			return
		}
		p.place(hwnd, anchor, height)
		showWindow(hwnd)
	})
	return nil
}

// hide takes the popover off screen. The window and its WebView2 instance stay alive
// so reopening is instant.
func (p *popover) hide() {
	p.mu.Lock()
	if !p.visible {
		p.mu.Unlock()
		return
	}
	p.visible = false
	hwnd := p.hwnd
	p.mu.Unlock()
	if hwnd != 0 {
		p.dispatch(func() { hideWindow(hwnd) })
	}
}

// Resize handles burnt_resize. app.js reports the same height several times per
// refresh, so an unchanged height must not touch the window: SetWindowPos would make
// the popover visibly twitch.
func (p *popover) Resize(heightDips float64) {
	height := ClampHeightDips(heightDips)
	p.mu.Lock()
	if height == p.heightDips {
		p.mu.Unlock()
		return
	}
	p.heightDips = height
	hwnd, visible, anchor := p.hwnd, p.visible, p.anchor
	p.mu.Unlock()

	if !visible || hwnd == 0 {
		return // remembered for the next show()
	}
	p.place(hwnd, anchor, height)
}

// place sizes the window in physical pixels for its monitor's DPI and re-clamps it
// inside the work area, keeping its bottom-right corner at the click that opened it.
func (p *popover) place(hwnd windows.HWND, anchor point, heightDips int32) {
	dpi := windowDPI(hwnd)
	w := ScaleDips(PopoverWidthDips, dpi)
	h := ScaleDips(heightDips, dpi)
	x, y := PlacePopover(anchor.X, anchor.Y, workArea(), w, h,
		ScaleDips(CursorGapDips, dpi), ScaleDips(EdgeMarginDips, dpi))
	moveWindow(hwnd, x, y, w, h)
}

// watchFocus dismisses the popover when the user clicks anything else, the way a
// macOS NSPopover does.
func (p *popover) watchFocus() {
	ticker := time.NewTicker(focusPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-p.done:
			return
		case <-ticker.C:
			p.mu.Lock()
			visible, hwnd, shownAt := p.visible, p.hwnd, p.shownAt
			p.mu.Unlock()
			if !visible || hwnd == 0 || time.Since(shownAt) < activationGrace {
				continue
			}
			if !hasFocus(hwnd) {
				p.hide()
			}
		}
	}
}

// probeWritable makes sure dir exists and can be written to.
func probeWritable(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("app: WebView2 data folder %s: %w", dir, err)
	}
	probe := filepath.Join(dir, ".writable")
	if err := os.WriteFile(probe, nil, 0o644); err != nil {
		return fmt.Errorf("app: WebView2 data folder %s is not writable: %w", dir, err)
	}
	os.Remove(probe)
	return nil
}

// Close stops the focus watcher. The window itself dies with the process.
func (p *popover) Close() {
	close(p.done)
}
